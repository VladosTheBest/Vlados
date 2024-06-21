package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

func (service *Service) GetAdminLaunchpad(limit, page int) (*model.AdminLaunchpadList, error) {
	launchpads := make([]model.Launchpad, 0)
	var rowCount int64 = 0

	reader := service.repo.ConnReader
	dbc := reader.Table("launchpads").Select("count(*) as total").Row()
	err := dbc.Scan(&rowCount)

	if err != nil {
		return nil, err
	}

	db := reader.Order("id DESC")

	if limit == 0 {
		db = db.Find(&launchpads)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&launchpads)
	}

	if db.Error != nil {
		return nil, db.Error
	}

	return &model.AdminLaunchpadList{
		Launchpads: launchpads,
		Meta: model.PagingMeta{
			Page:  page,
			Count: rowCount,
			Limit: limit,
		},
	}, nil
}

func (service *Service) GetLaunchpadFullInfo(launchpadId string, userId uint64) (*model.GetLaunchpadResponse, error) {
	launchpad, err := service.GetLaunchpad(launchpadId)
	if err != nil {
		return nil, err
	}
	totalContribution, err := service.repo.GetBoughtTokenAmount(launchpad.ID)
	if err != nil {
		return nil, err
	}

	totalUserContribution, err := service.repo.GetBoughtTokenAmountByUser(launchpad.ID, userId)
	if err != nil {
		return nil, err
	}

	boughtTokensByLineLevels, err := service.repo.GetBoughtTokensByLineLevels(launchpad.ID)
	if err != nil {
		return nil, err
	}

	return &model.GetLaunchpadResponse{
		Launchpad:                *launchpad,
		TotalContribution:        totalContribution.V,
		TotalUserContribution:    totalUserContribution.V,
		BoughtTokensByLineLevels: *boughtTokensByLineLevels,
	}, nil
}

func (service *Service) GetLaunchpad(launchpadId string) (*model.Launchpad, error) {
	launchpad := &model.Launchpad{}
	db := service.repo.ConnReader.First(&launchpad, "id = ?", launchpadId)
	if db.Error != nil {
		return nil, db.Error
	}

	return launchpad, nil
}

func (service *Service) LaunchpadMakePayment(launchpadId string, user *model.User, amount *decimal.Big) error {
	var totalUserTokens = &postgres.Decimal{V: model.ZERO}
	var userLevel string
	logger := log.With().
		Str("service", "launchpad").
		Str("method", "LaunchpadMakePayment").
		Uint64("userId", user.ID).
		Interface("launchpadId", launchpadId).
		Logger()

	account, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, user.ID, model.AccountGroupMain)

	if err != nil {
		logger.Error().Err(err).Msg("unable to get account")
		return err
	}
	launchpad, err := service.GetLaunchpad(launchpadId)
	if err != nil {
		logger.Error().Err(err).Msg("unable to get launchpad")
		return err
	}

	lineLevels := &model.LaunchpadLineLevels{}
	err = lineLevels.GetLineLevels(launchpad)
	if err != nil {
		logger.Error().Err(err).Msg("unable to get line levels")
		return err
	}

	prdxCoin, _ := coins.Get("prdx")
	usdtCoin, _ := coins.Get("usdt")

	totalUserTokens.V, err = service.repo.GetUserAvailableBalanceForCoin(user.ID, prdxCoin.Symbol, account)
	if err != nil {
		totalUserTokens = &postgres.Decimal{V: model.ZERO}
	}
	userLevel, _ = service.repo.GetUserLevelByAvailableBalance(totalUserTokens)
	if err != nil {
		logger.Error().Err(err).Msg("unable to get line levels")
		return err
	}

	boughtTokenAmountLineLevel, err := service.repo.GetBoughtTokenAmountByUserLevel(launchpad.ID, userLevel)
	if err != nil {
		logger.Error().Err(err).Msg("unable to get line level spent amount")
		return err
	}

	amount.Context = decimal.Context128
	amount.Context.RoundingMode = decimal.ToZero
	amount.Quantize(usdtCoin.TokenPrecision)

	lineLevelValue, err := lineLevels.GetLaunchpadLineLevelByUserLevel(userLevel)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid amount for current Line level")
		return err
	}

	//if time.Now().Before(launchpad.StartDate) {
	//	logger.Error().Msg("Time of launchpad has not started yet")
	//	return errors.New("time of launchpad has not started yet")
	//}
	//if time.Now().After(launchpad.EndDate) {
	//	logger.Error().Msg("Time of launchpad has passed")
	//	return errors.New("time of launchpad has passed")
	//}
	if launchpad.Status != model.LaunchpadStatusActive {
		logger.Error().Err(err).Msg("Launchpad disabled")
		return errors.New("launchpad disabled")
	}

	boughtTokenAmount := new(decimal.Big)
	boughtTokenAmount.Quo(amount, launchpad.PresalePrice.V)

	boughtTokenAmountCap, err := service.repo.GetBoughtTokenAmount(launchpad.ID)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to retrieve spent amount")
		return err
	}
	boughtTokenAmountCap.V = boughtTokenAmountCap.V.Add(boughtTokenAmountCap.V, boughtTokenAmount)
	if boughtTokenAmountCap.V.Cmp(launchpad.ContributionsCap.V) >= 0 {
		logger.Error().Err(err).Msg("Contribution cap exceeded")
		return errors.New("contribution cap exceeded")
	}

	boughtTokenAmountLineLevel.V = boughtTokenAmountLineLevel.V.Add(boughtTokenAmountLineLevel.V, boughtTokenAmount)

	if lineLevelValue.Cmp(boughtTokenAmountLineLevel.V) <= 0 {
		logger.Error().Msg("Invalid amount for current Line level")
		return errors.New("invalid amount for current line level")
	}

	data := model.NewLaunchpadMakePaymentRequest(launchpad, user.ID, amount, userLevel)
	err = service.ops.LaunchpadMakePayment(data)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to buy launchpad")
		return err
	}

	return nil
}

func (service *Service) GetLaunchpadFullInfoList(userId uint64) (*model.LaunchpadFullInfoList, error) {
	launchpadFullInfoList := make([]model.GetLaunchpadResponse, 0)
	launchpadList, err := service.GetLaunchpadList()
	if err != nil {
		return nil, err
	}

	for _, launchpad := range launchpadList.Launchpads {
		totalContribution, err := service.repo.GetBoughtTokenAmount(launchpad.ID)
		if err != nil {
			return nil, err
		}

		totalUserContribution, err := service.repo.GetBoughtTokenAmountByUser(launchpad.ID, userId)
		if err != nil {
			return nil, err
		}

		boughtTokensByLineLevels, err := service.repo.GetBoughtTokensByLineLevels(launchpad.ID)
		if err != nil {
			return nil, err
		}

		launchpadFullInfo := &model.GetLaunchpadResponse{
			Launchpad:                launchpad,
			TotalContribution:        totalContribution.V,
			TotalUserContribution:    totalUserContribution.V,
			BoughtTokensByLineLevels: *boughtTokensByLineLevels,
		}
		launchpadFullInfoList = append(launchpadFullInfoList, *launchpadFullInfo)
	}

	return &model.LaunchpadFullInfoList{Launchpads: launchpadFullInfoList}, nil
}

func (service *Service) GetLaunchpadList() (*model.LaunchpadList, error) {
	launchpads := make([]model.Launchpad, 0)

	db := service.repo.ConnReader.Table("launchpads").Order("id DESC").Find(&launchpads)

	if db.Error != nil {
		return nil, db.Error
	}

	return &model.LaunchpadList{
		Launchpads: launchpads,
	}, nil
}

func (service *Service) LaunchpadEndPresale(launchpadId string) error {
	launchpad, err := service.GetLaunchpad(launchpadId)

	if err != nil {
		return err
	}

	logger := log.With().
		Str("service", "launchpad").
		Str("method", "LaunchpadMakePayment").
		Interface("launchpadId", launchpadId).
		Logger()

	if time.Now().Before(launchpad.EndDate) {
		logger.Error().Msg("Time of launchpad has not passed")
		return errors.New("time of launchpad has not passed")
	}

	err = service.ops.LaunchpadEndPresale(launchpad)

	return err
}

func (service *Service) CreateLaunchpad(launchpadRequest *model.LaunchpadRequest, logo *multipart.FileHeader) (*model.Launchpad, error) {
	fileBytes := bytes.NewBuffer(nil)
	logoFile, err := logo.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = logoFile.Close()
	}()

	if _, err := io.Copy(fileBytes, logoFile); err != nil {
		return nil, err
	}
	mimeType := http.DetectContentType(fileBytes.Bytes())

	logoBase64 := ""
	switch mimeType {
	case "image/jpeg":
		logoBase64 += "data:image/jpeg;base64,"
	case "image/png":
		logoBase64 += "data:image/png;base64,"
	}
	logoBase64 += base64.StdEncoding.EncodeToString(fileBytes.Bytes())

	status, err := model.GetLaunchpadStatusFromString(launchpadRequest.Status)

	if err != nil {
		return nil, err
	}

	launchpad := model.NewLaunchpad(launchpadRequest, logoBase64, status)

	coin, err := service.GetCoin(launchpad.CoinSymbol)
	if err != nil {
		return nil, err
	}

	if !coin.IsCoinStatusPresale() {
		return nil, errors.New("launchpad can be created with only presale tokens")
	}

	err = service.repo.Create(launchpad)
	if err != nil {
		return nil, err
	}

	return launchpad, nil
}

func (service *Service) UpdateLaunchpad(launchpadRequest *model.LaunchpadRequest, logo *multipart.FileHeader, launchpadId string) (*model.Launchpad, error) {
	fileBytes := bytes.NewBuffer(nil)
	logoFile, err := logo.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = logoFile.Close()
	}()

	if _, err := io.Copy(fileBytes, logoFile); err != nil {
		return nil, err
	}
	mimeType := http.DetectContentType(fileBytes.Bytes())

	logoBase64 := ""
	switch mimeType {
	case "image/jpeg":
		logoBase64 += "data:image/jpeg;base64,"
	case "image/png":
		logoBase64 += "data:image/png;base64,"
	}
	logoBase64 += base64.StdEncoding.EncodeToString(fileBytes.Bytes())

	status, err := model.GetLaunchpadStatusFromString(launchpadRequest.Status)

	if err != nil {
		return nil, err
	}

	launchpad, err := service.GetLaunchpad(launchpadId)

	if time.Now().After(launchpad.StartDate) && launchpadRequest.PresalePrice.Cmp(launchpad.PresalePrice.V) != 0 {
		return nil, errors.New("presale price cannot be changed after launchpad started")
	}
	launchpad.UpdateLaunchpad(launchpadRequest, logoBase64, status)
	if err != nil {
		return nil, err
	}

	coin, err := service.GetCoin(launchpadRequest.CoinSymbol)
	if err != nil {
		return nil, err
	}

	if !coin.IsCoinStatusPresale() {
		return nil, errors.New("launchpad can be created with only presale tokens")
	}

	db := service.repo.Conn.Table("launchpads").Where("id = ?", launchpadId).Save(launchpad)
	if db.Error != nil {
		return nil, db.Error
	}

	return launchpad, nil
}
