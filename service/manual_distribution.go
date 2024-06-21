package service

import (
	"errors"
	"fmt"
	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"strings"
	"sync"
	"time"
)

func (service *Service) GetManualDistributionsForUser(userID uint64, limit, page, from, to int) (*model.LiabilityList, error) {
	liabilities := make([]model.Liability, 0)
	var rowCount int64 = 0

	q := service.repo.ConnReader.Table("liabilities").Where("ref_type = 'distribution' AND user_id = ?", userID)

	if from > 0 {
		q = q.Where("created_at >= to_timestamp(?) ", from)
	}

	if to > 0 {
		q = q.Where("created_at <= to_timestamp(?) ", to)
	}

	dbc := q.Count(&rowCount)

	if dbc.Error != nil {
		return nil, dbc.Error
	}

	q = q.Order("id DESC")
	if limit != 0 {
		q = q.Limit(limit).Offset((page - 1) * limit)
	}
	db := q.Find(&liabilities)

	liabilityList := model.LiabilityList{
		Liabilities: liabilities,
		Meta: model.PagingMeta{
			Page:  int(page),
			Count: rowCount,
			Limit: int(limit),
		},
	}

	return &liabilityList, db.Error
}

//// GetManualDistributedBonus returns the number of distributed bonus (in PRDX) from the system
//func (service *Service) GetManualDistributedBonus() (*decimal.Big, error) {
//	result := decimal.New(0, 1)
//	coins := []string{"prdx", "terc"}
//	rows, err := service.repo.Conn.
//		Table("manual_distribution_funds").
//		Select("SUM(converted_balance) as total").
//		Joins("inner join manual_distributions on manual_distribution_funds.distribution_id = distributions.id").
//		Where("manual_distribution_funds.status = $1 and converted_coin_symbol = ANY($2)", "completed", pq.Array(coins)).Rows()
//
//	if err != nil {
//		return result, err
//	}
//	defer rows.Close()
//	for rows.Next() {
//		totalBalance := &postgres.Decimal{}
//		_ = rows.Scan(&totalBalance)
//		if totalBalance == nil {
//			return result, err
//		}
//		result = totalBalance.V
//	}
//	return result, err
//}

// ExportManualDistributionsForUser  - distribution data to export
func (service *Service) ExportManualDistributionsForUser(format string, liabilities []model.Liability) (*model.GeneratedFile, error) {
	data := [][]string{}
	data = append(data, []string{"ID", "Date & Time", "Distribution ID", "Amount"})
	widths := []int{15, 45, 45, 45}

	for i := 0; i < len(liabilities); i++ {
		o := liabilities[i]
		data = append(data, []string{fmt.Sprint(o.ID), o.CreatedAt.Format("2 Jan 2006 15:04:05"), fmt.Sprint(o.RefID), fmt.Sprint(utils.Fmt(o.Credit.V))})
	}

	var resp []byte
	var err error

	title := "Distributions Report"

	if format == "csv" {
		resp, err = CSVExport(data)
	} else {
		resp, err = PDFExport(data, widths, title)
	}

	generatedFile := model.GeneratedFile{
		Type:     format,
		DataType: "distributions",
		Data:     resp,
	}
	return &generatedFile, err
}

// GetManualDistributionEvents godoc
func (service *Service) GetManualDistributionEvents(limit, page int) (*model.ManualDistributionList, error) {
	distributions := make([]model.ManualDistribution, 0)
	var rowCount int64 = 0

	q := service.repo.ConnReader

	dbc := q.Table("manual_distributions").Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	db := q.Select("*").Order("id DESC")
	if limit == 0 {
		db = db.Find(&distributions)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&distributions)
	}

	distributionList := model.ManualDistributionList{
		Distributions: distributions,
		Meta: model.PagingMeta{
			Page:  int(page),
			Count: rowCount,
			Limit: int(limit),
		},
	}

	return &distributionList, db.Error
}

// GetManualDistributionByID godoc
func (service *Service) GetManualDistributionByID(distID uint64) (model.ManualDistribution, error) {
	dist := model.ManualDistribution{}
	db := service.repo.ConnReader.First(&dist, "id = ?", distID)
	return dist, db.Error
}

// GetManualDistributionEntries godoc
func (service *Service) GetManualDistributionEntries(distID uint64, limit, page int) (*model.LiabilityList, error) {
	liabilities := make([]model.Liability, 0)
	var rowCount int64 = 0

	q := service.repo.ConnReader.Where("ref_type = 'distribution' AND ref_id = ?", distID)

	dbc := q.Table("liabilities").Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	q = q.Order("id DESC")
	if limit != 0 {
		q = q.Limit(limit).Offset((page - 1) * limit)
	}
	db := q.Find(&liabilities)

	liabilityList := model.LiabilityList{
		Liabilities: liabilities,
		Meta: model.PagingMeta{
			Page:  int(page),
			Count: rowCount,
			Limit: int(limit),
		},
	}

	return &liabilityList, db.Error
}

// GetManualDistributionFunds godoc
func (service *Service) GetManualDistributionFunds(distID uint64) (*model.ManualDistributionFundsList, error) {
	funds := make([]model.ManualDistributionFund, 0)
	var rowCount int64 = 0
	crossRates, err := service.coinValues.GetAll()

	if err != nil {
		return nil, err
	}
	approximateAmount := conv.NewDecimalWithPrecision()

	q := service.repo.ConnReader
	dbc := q.Table("manual_distribution_funds").Where("distribution_id = ?", distID).Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)
	db := q.Where("distribution_id = ?", distID).Find(&funds)

	for _, v := range funds {
		crossPRDX := conv.NewDecimalWithPrecision().Mul(crossRates[strings.ToUpper(v.CoinSymbol)]["PRDX"], v.TotalBalance.V)
		approximateAmount = approximateAmount.Add(approximateAmount, crossPRDX)
	}

	distributionList := model.ManualDistributionFundsList{
		DistributionFunds: funds,
		Meta: model.PagingMeta{
			Page:  1,
			Limit: int(rowCount),
			Count: rowCount,
		},
		ApproximateAmount: approximateAmount,
	}
	return &distributionList, db.Error
}

// GetManualDistributionBalances godoc
func (service *Service) GetManualDistributionBalances(distID uint64, page, limit int, userEmail, level string) (*model.ManualDistributionBalancesList, error) {
	balances := make([]model.ManualDistributionBalance, 0)
	var rowCount int64 = 0
	totalRedeemedPRDX := &postgres.Decimal{V: conv.NewDecimalWithPrecision()}

	rows, err := service.repo.ConnReaderAdmin.
		Table("manual_distribution_balances as mdb").
		Where("distribution_id = ?", distID).
		Select("SUM(allocated_balance) as total").
		Where("mdb.status = ?", model.DistributionBalanceStatus_Claimed).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		_ = rows.Scan(totalRedeemedPRDX)
	}

	db := service.repo.ConnReaderAdmin.
		Table("manual_distribution_balances as mdb").
		Where("distribution_id = ?", distID)

	if len(userEmail) > 0 {
		qUserEmail := "%" + userEmail + "%"
		var usersID []uint64

		if err := service.repo.ConnReader.Table("users").Where("email LIKE ?", qUserEmail).
			Pluck("id", &usersID).Error; err != nil {
			return nil, err
		}

		db = db.Where("user_id IN (?)", usersID)
	}

	if model.UserFeeLevel(level).IsValidUserLevel() {
		db = db.Where("level = ?", level)
	}

	dbc := db.Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	db = db.Joins("left join users as u ON mdb.user_id = u.id").
		Select("mdb.*, u.email as email")

	if limit > 0 {
		db = db.Limit(limit).Offset((page - 1) * limit)
	}
	db = db.Find(&balances)

	resp := model.ManualDistributionBalancesList{
		DistributionBalances: balances,
		Meta: model.PagingMeta{
			Page:  page,
			Limit: limit,
			Count: rowCount,
		},
		TotalRedeemedPRDX: totalRedeemedPRDX.V,
	}
	return &resp, db.Error
}

func (service *Service) GetManualDistributionCurrentID() (distId uint64, err error) {
	err = service.repo.ConnReader.Table("manual_distributions as m").
		Where("current_timestamp > m.day AND current_timestamp < m.day + '+23h 30m'").
		Where("m.status = 'completed'").
		Select("id").Row().Scan(&distId)

	if err != nil {
		return
	}
	if distId == 0 {
		err = errors.New("distribution for current day not found")
		return
	}

	return
}

func (service *Service) GetManualDistributionGetBonus(distId uint64, userId uint64) error {
	err := service.ops.GetManualDistributionGetBonus(distId, userId)
	return err
}

type ManualDistributionInfoResponse struct {
	TotalEarned        *decimal.Big `json:"total_earned"`
	TotalUserTokens    *decimal.Big `json:"total_user_tokens"`
	CurrentBonusAmount *decimal.Big `json:"current_bonus_amount"`
	CurrentBonusCoin   string       `json:"current_bonus_coin"`
	CountdownTimer     int64        `json:"countdown_timer"`
	NextEvent          int64        `json:"next_event_timer"`
	UserLevel          string       `json:"user_level"`
}

func (service *Service) GetManualDistributionInfoByUser(userId uint64) *ManualDistributionInfoResponse {
	var totalEarned = &postgres.Decimal{V: model.ZERO}
	var currentBonusAmount = &postgres.Decimal{V: model.ZERO}
	var currentBonusCoin = service.cfg.Distribution.Coin
	var currentBonusCoinObj, _ = coins.Get(service.cfg.Distribution.Coin)
	var countdownTimer int64
	var nextEventTime int64
	var userLevel string
	var totalUserTokens = &postgres.Decimal{V: model.ZERO}
	wg := sync.WaitGroup{}

	account, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, userId, model.AccountGroupMain)
	if err != nil {
		return &ManualDistributionInfoResponse{}
	}

	if distId, err := service.GetManualDistributionCurrentID(); err == nil {

		// current bonus amount
		// current bonus symbol

		wg.Add(1)
		go func() {
			defer wg.Done()
			q := service.repo.ConnReader
			err := q.Table("manual_distribution_balances").
				Where("user_id = ?", userId).
				Where("status = ?", "allocated").
				Where("distribution_id = ?", distId).
				Select("allocated_balance, allocated_coin_symbol").
				Row().Scan(&currentBonusAmount, &currentBonusCoin)

			if err != nil {
				currentBonusAmount = &postgres.Decimal{V: model.ZERO}
				currentBonusCoin = service.cfg.Distribution.Coin

				currentBonusAmount.V.Context = decimal.Context128
				currentBonusAmount.V.Context.RoundingMode = decimal.ToZero
				currentBonusAmount.V.Quantize(currentBonusCoinObj.TokenPrecision)
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			q := service.repo.Conn

			var dist = model.ManualDistribution{}
			err := q.Table("manual_distributions").
				Where("id = ?", distId).First(&dist).Error

			if err != nil {
				return
			}

			countdownTimerDiff := time.Until(dist.Day.AddDate(0, 0, 1).Add(-5 * time.Minute))

			countdownTimer = int64(countdownTimerDiff.Truncate(time.Second).Seconds())
			if countdownTimer < 0 {
				countdownTimer = 0
			}
			nextEventTime = dist.CreatedAt.AddDate(0, 0, 1).Unix()
		}()
	}

	// PRDX earned
	wg.Add(1)
	go func() {
		defer wg.Done()

		q := service.repo.ConnReader
		dbc := q.Table("liabilities").
			Where("user_id = ?", userId).
			Where("ref_type = 'distribution'").
			Where("sub_account = ?", account.ID).
			Select("COALESCE(sum(credit), 0.0) as total").
			Row()
		err := dbc.Scan(&totalEarned)
		if err != nil {
			totalEarned = &postgres.Decimal{V: model.ZERO}
		}
	}()

	// all PRDX for user
	wg.Add(1)
	go func() {
		defer wg.Done()

		totalUserTokens.V, err = service.repo.GetUserAvailableBalanceForCoin(userId, service.cfg.Distribution.Coin, account)
		if err != nil {
			totalUserTokens = &postgres.Decimal{V: model.ZERO}
		}
		userLevel, _ = service.repo.GetUserLevelByAvailableBalance(totalUserTokens)
	}()
	// user level

	wg.Wait()

	return &ManualDistributionInfoResponse{
		TotalEarned:        totalEarned.V,
		TotalUserTokens:    totalUserTokens.V,
		CurrentBonusAmount: currentBonusAmount.V,
		CurrentBonusCoin:   currentBonusCoin,
		CountdownTimer:     countdownTimer,
		NextEvent:          nextEventTime,
		UserLevel:          userLevel,
	}
}
