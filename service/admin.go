package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/rs/zerolog/log"
	kafkaGo "github.com/segmentio/kafka-go"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data/wallet"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/httpagent"

	// "github.com/ericlagergren/decimal/sql/postgres"
	// coinsCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	// "gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

// Statistics type
type Statistics struct {
	UsersCount  map[string]int `json:"users_count"`
	OrdersCount int            `json:"orders_count"`
}

// CoinWithStatistics
type CoinWithStatistics struct {
	Coin        string `json:"coin"`
	Assets      string `json:"assets"`
	Liabilities string `json:"liabilities"`
	Profit      string `json:"profit"`
	BTCValue    string `json:"btc_value"`
	Expenses    string `json:"expenses"`
}

// GetFeatures - Get a list of features
func (service *Service) GetFeatures() ([]model.AdminFeatureSettings, error) {
	featureSettings := make([]model.AdminFeatureSettings, 0)
	db := service.repo.ConnReaderAdmin.Find(&featureSettings)
	if db.Error != nil {
		return nil, db.Error
	}

	return featureSettings, nil
}

// GetWithdrawLimits - Get withdrawal limits
func (service *Service) GetWithdrawLimits() ([]model.AdminFeatureSettings, error) {
	featureSettings := make([]model.AdminFeatureSettings, 0)
	feature := "%withdraw_limit_level%"
	db := service.repo.ConnReader.
		Where("feature ILIKE ?", feature).
		Find(&featureSettings)
	if db.Error != nil {
		return nil, db.Error
	}

	return featureSettings, nil
}

// UpdateWithdrawLimitByUser - updater withdrawal limit by user.
func (service *Service) UpdateWithdrawLimitByUser(externalSystem model.WithdrawExternalSystem, userPaymentDetails model.UserPaymentDetails, withdrawLimit *decimal.Big) error {
	var externalSystemWL string
	switch externalSystem {
	case model.WithdrawExternalSystem_Bitgo:
		externalSystemWL = "withdraw_limit"
	case model.WithdrawExternalSystem_Advcash:
		externalSystemWL = "adv_cash_withdraw_limit"
	case model.WithdrawExternalSystem_ClearJunction:
		externalSystemWL = "clear_junction_withdraw_limit"
	case model.WithdrawExternalSystem_Default:
		externalSystemWL = "default_withdraw_limit"
	}

	wl := &postgres.Decimal{V: withdrawLimit}

	err := service.repo.Conn.Model(userPaymentDetails).Update(externalSystemWL, wl).Error
	if err != nil {
		return err
	}

	return nil
}

// GetFeatureValue - Get value of a feature
func (service *Service) GetFeatureValue(feature string) (string, error) {
	featureSettings := model.AdminFeatureSettings{}
	db := service.repo.ConnReaderAdmin.Where("feature = ?", feature).First(&featureSettings)
	if db.Error != nil {
		return "true", db.Error
	}

	return featureSettings.Value, nil
}

// UpdateFeature - Update information about a feature
func (service *Service) UpdateFeature(feature, value string) (*model.AdminFeatureSettings, error) {
	featureSetting := model.AdminFeatureSettings{}
	db := service.repo.ConnReaderAdmin.Where("feature = ?", feature).First(&featureSetting)
	if db.Error != nil {
		return nil, db.Error
	}
	featureSetting.Value = value
	featureSetting.UpdatedAt = time.Now()
	err := service.repo.Update(featureSetting)
	if err != nil {
		return nil, err
	}
	return &featureSetting, nil
}

// BlockWithdrawByUser - block withdraw for user.
func (service *Service) BlockWithdrawByUser(coinType model.CoinType, switcher string, userPaymentDetails *model.UserPaymentDetails) error {
	var withdrawalAllowed bool
	switch switcher {
	case "on":
		withdrawalAllowed = true
	case "off":
		withdrawalAllowed = false
	}

	var cType string
	switch coinType {
	case model.CoinTypeCrypto:
		cType = "block_withdraw_crypto"
	case model.CoinTypeFiat:
		cType = "block_withdraw_fiat"
	}

	err := service.repo.Conn.Model(userPaymentDetails).Update(cType, withdrawalAllowed).Error
	if err != nil {
		return err
	}

	return nil
}

// BlockDepositByUser - block deposit for user.
func (service *Service) BlockDepositByUser(coinType model.CoinType, switcher string, userPaymentDetails *model.UserPaymentDetails) error {
	var depositAllowed bool
	switch switcher {
	case "on":
		depositAllowed = true
	case "off":
		depositAllowed = false
	}

	var cType string
	switch coinType {
	case model.CoinTypeCrypto:
		cType = "block_deposit_crypto"
	case model.CoinTypeFiat:
		cType = "block_deposit_fiat"
	}

	err := service.repo.Conn.Model(userPaymentDetails).Update(cType, depositAllowed).Error
	if err != nil {
		return err
	}

	return nil
}

// BlockWithdrawByCoin - block withdraw for coin.
func (service *Service) BlockWithdrawByCoin(coin *model.Coin, switcher string) (*model.Coin, error) {
	var withdrawalAllowed bool
	switch switcher {
	case "on":
		withdrawalAllowed = true
	case "off":
		withdrawalAllowed = false
	}

	err := service.repo.Conn.Model(coin).Update("block_withdraw", withdrawalAllowed).Error
	if err != nil {
		return nil, err
	}

	return coin, nil
}

// BlockDepositByCoin - block deposit for coin.
func (service *Service) BlockDepositByCoin(coin *model.Coin, switcher string) (*model.Coin, error) {
	var depositAllowed bool
	switch switcher {
	case "on":
		depositAllowed = true
	case "off":
		depositAllowed = false
	}

	err := service.repo.Conn.Model(coin).Update("block_deposit", depositAllowed).Error
	if err != nil {
		return nil, err
	}
	return coin, nil
}

// BlockWithdraw - block withdraw for coin.
func (service *Service) BlockWithdraw(switcher string) error {
	adminFeatureSettings := model.AdminFeatureSettings{}
	db := service.repo.ConnReaderAdmin.Where("feature = ?", "block_withdraw").First(&adminFeatureSettings)
	if db.Error != nil {
		return db.Error
	}

	var withdrawalAllowed string
	switch switcher {
	case "on":
		withdrawalAllowed = "true"
	case "off":
		withdrawalAllowed = "false"
	}

	err := service.repo.Conn.Model(adminFeatureSettings).Updates(map[string]interface{}{"value": withdrawalAllowed, "updated_at": time.Now()}).Error
	if err != nil {
		return err
	}

	return nil
}

// BlockDeposit - block deposit for coin.
func (service *Service) BlockDeposit(switcher string) error {
	adminFeatureSettings := model.AdminFeatureSettings{}
	db := service.repo.ConnReaderAdmin.Where("feature = ?", "block_deposit").First(&adminFeatureSettings)
	if db.Error != nil {
		return db.Error
	}

	var depositAllowed string
	switch switcher {
	case "on":
		depositAllowed = "true"
	case "off":
		depositAllowed = "false"
	}

	err := service.repo.Conn.Model(adminFeatureSettings).Updates(map[string]interface{}{"value": depositAllowed, "updated_at": time.Now()}).Error
	if err != nil {
		return err
	}

	return nil
}

// UpdateFiatFee - Update fiat fees
func (service *Service) UpdateFiatFee(value string, externalSystem model.WithdrawExternalSystem) error {
	tx := service.repo.Conn.Begin()
	db := tx.Table("coins").Where("type = ?", model.CoinTypeFiat)
	if db.Error != nil {
		return db.Error
	}
	withdrawFee := ""
	switch externalSystem {
	case model.WithdrawExternalSystem_Advcash:
		withdrawFee = "withdraw_fee_adv_cash"
	case model.WithdrawExternalSystem_ClearJunction:
		withdrawFee = "withdraw_fee_clear_junction"
	case model.WithdrawExternalSystem_Default:
		withdrawFee = "withdraw_fee"
	}
	db = db.Update(withdrawFee, value)
	if db.Error != nil {
		return db.Error
	}

	return tx.Commit().Error
}

// GetUsersCountByLevel
func (service *Service) GetUsersCountByLevel(from, to int) (*Statistics, error) {
	statistics := Statistics{}
	statistics.UsersCount = make(map[string]int)

	q := service.repo.ConnReaderAdmin.
		Table("user_settings")

	if from > 0 {
		q = q.Where("created_at >= to_timestamp(?) ", from)
	}

	if to > 0 {
		q = q.Where("created_at <= to_timestamp(?) ", to)
	}

	rows, err := q.
		Select("user_level, count(*) as total").
		Group("user_level").
		Rows()

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		level := ""
		amount := 0
		_ = rows.Scan(&level, &amount)
		statistics.UsersCount["level"+level] = amount
	}

	return &statistics, nil
}

// GetActiveOrderCount godoc
func (service *Service) GetActiveOrderCount() int {
	var marketMakers []uint64
	service.repo.ConnReaderAdmin.Table("users").
		Where("email = ?", "marketmaker@paramountdax.com").
		Pluck("id", &marketMakers)

	rowCount := 0
	db := service.repo.ConnReaderAdmin.
		Table("orders").
		Select("count(*) as total").
		Where("status in (?)", []model.OrderStatus{model.OrderStatus_Pending, model.OrderStatus_Untouched, model.OrderStatus_PartiallyFilled})
	if len(marketMakers) != 0 {
		db.Where("owner_id NOT IN (?)", marketMakers)
	}
	dbRow := db.Row()

	_ = dbRow.Scan(&rowCount)
	return rowCount
}

// TradeInfoStats godoc
type TradeInfoStats map[string]int

// GetInfoAboutTrades - daily trade numbers month/week
func (service *Service) GetInfoAboutTrades(datatype, from, to string) (TradeInfoStats, error) {
	trades := TradeInfoStats{}
	days := "30"
	if datatype == "weekly" {
		days = "7"
	}

	generateSeries := "generate_series(date_trunc('day', now()) - '`" + days + "` day'::interval, date_trunc('days', now()), '1 day'::interval) day) d"

	if from != "" && to == "" {
		generateSeries = "generate_series(to_timestamp(" + from + "), now(),'1 day'::interval) day) d"
	} else if from != "" && to != "" {
		generateSeries = "generate_series(to_timestamp(" + from + "), to_timestamp(" + to + "),'1 day'::interval) day) d"
	}

	rows, err := service.repo.ConnReaderAdmin.
		Raw(`SELECT *
		FROM  (
		SELECT day::date
		FROM ` + generateSeries + `
		LEFT   JOIN (
		SELECT date_trunc('day', created_at)::date AS day, count(*) AS total
		FROM   trades
		WHERE  created_at >= NOW() - INTERVAL '` + days + ` DAY'
		GROUP  BY 1
		) t USING (day)
		ORDER  BY day`).
		Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		date := ""
		total := 0
		_ = rows.Scan(&date, &total)
		trades[date] = total
	}

	return trades, nil
}

// GetInfoAboutFees - daily fees numbers month/week
func (service *Service) GetInfoAboutFees(datatype, from, to string, mode model.GeneratedMode) ([]*model.FeesInfoStats, error) {
	fees := make([]*model.FeesInfoStats, 0)
	days := "30"
	if datatype == "weekly" {
		days = "7"
	}

	crossRates, err := service.coinValues.GetAll()
	if err != nil {
		return nil, err
	}

	generateSeries := "generate_series(date_trunc('day', now()) - '`" + days + "` day'::interval, date_trunc('days', now()), '1 day'::interval) day) d"

	if from != "" && to == "" {
		generateSeries = "generate_series(to_timestamp(" + from + "), now(),'1 day'::interval) day) d"
	} else if from != "" && to != "" {
		generateSeries = "generate_series(to_timestamp(" + from + "), to_timestamp(" + to + "),'1 day'::interval) day) d"
	}

	queryMode := ""
	if len(mode) > 0 {
		switch mode {
		case model.GeneratedModeByManual:
			queryMode = "AND b.sub_account is NULL"
		case model.GeneratedModeByBot:
			queryMode = "AND b.sub_account = r.sub_account"
		}
	}

	rows, err := service.repo.ConnReaderAdmin.
		Raw(`SELECT *
		FROM  (
		SELECT day::date
		FROM ` + generateSeries + `
		LEFT   JOIN (
		SELECT date_trunc('day', r.created_at)::date AS day, r.coin_symbol as coin, c.token_precision, sum(r.credit) as fee
		FROM   revenue_aggr as r
		left join users as u on u.id = r.user_id
		left join coins as c on r.coin_symbol = c.symbol
		left join bots as b on r.sub_account = b.sub_account
		WHERE  u.role_alias in ('member','admin','business','broker') AND r.created_at >= NOW() - INTERVAL '` + days + ` DAY' AND r.ref_type = 'trade' ` + queryMode + ` AND u.email != 'marketmaker@paramountdax.com'
		GROUP  BY r.coin_symbol, date_trunc('day', r.created_at)::date, c.token_precision
		) t USING (day)
		ORDER  BY day`).Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		data := model.FeesInfoStats{}

		if err := service.repo.ConnReaderAdmin.ScanRows(rows, &data); err != nil {
			return nil, err
		}

		fees = append(fees, &data)
	}

	feesMapped := map[string]*model.FeesInfoStats{}

	var feesOut []*model.FeesInfoStats
	for _, value := range fees {
		if feesMapped[value.Day] == nil {
			feesMapped[value.Day] = &model.FeesInfoStats{
				Day:  value.Day,
				Coin: "total_usdt",
				Fee:  &postgres.Decimal{V: conv.NewDecimalWithPrecision()},
			}
		}
		if feesMapped[value.Day] != nil && value.Coin == "usdt" {
			feesMapped[value.Day].TokenPrecision = value.TokenPrecision
		}

		if crossRates[strings.ToUpper(value.Coin)]["USDT"] != nil {
			totalCross := conv.NewDecimalWithPrecision().Mul(crossRates[strings.ToUpper(value.Coin)]["USDT"], value.Fee.V)
			feesMapped[value.Day].Fee.V.Add(feesMapped[value.Day].Fee.V, totalCross)
		}
	}

	for _, item := range feesMapped {
		feesOut = append(feesOut, item)
	}

	sort.Slice(feesOut, func(i, j int) bool {
		day, err := time.Parse(time.RFC3339, feesOut[i].Day)
		if err != nil {
			return false
		}
		day2, err := time.Parse(time.RFC3339, feesOut[j].Day)
		if err != nil {
			return false
		}
		return day.Before(day2)
	})

	return feesOut, nil
}

func (service *Service) GetManualWithdrawals(limit, page int, status string) (*model.ManualTransactionList, error) {
	transactions := make([]model.ManualTransactionResponse, 0)
	var rowCount int64 = 0
	db := service.repo.ConnReaderAdmin.
		Table("manual_transactions").
		Where("manual_transactions.tx_type = ? ", model.TxType_Withdraw)

	if len(status) > 0 {
		db = db.Where("manual_transactions.status = ?", status)
	}

	dbc := db.Select("count(manual_transactions.id) as total").Row()
	_ = dbc.Scan(&rowCount)

	db.
		Joins("LEFT JOIN users u ON manual_transactions.user_id = u.id").
		Select("manual_transactions.*, u.email as email").
		Order("manual_transactions.created_at DESC").
		Limit(limit).Offset((page - 1) * limit).Find(&transactions)
	if db.Error != nil {
		return nil, db.Error
	}

	return &model.ManualTransactionList{
		ManualTransactions: transactions,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}, db.Error
}

func (service *Service) CreateAdminManualWithdrawal(createWithdrawalRequest *model.CreateWithdrawalRequest, adminUser *model.User) (*model.ManualTransaction, error) {
	var coin model.Coin
	db := service.repo.ConnReaderAdmin.Where("symbol = ?", createWithdrawalRequest.CoinSymbol).First(&coin)
	if db.Error != nil {
		return nil, db.Error
	}

	var user model.User
	db = service.repo.ConnReaderAdmin.Where("id = ?", createWithdrawalRequest.UserID).First(&user)
	if db.Error != nil {
		return nil, db.Error
	}

	manualWithdrawal, err := model.NewManualWithdrawal(createWithdrawalRequest)
	if err != nil {
		return nil, err
	}

	err = service.repo.Conn.Table("manual_transactions").Create(&manualWithdrawal).Error
	if err != nil {
		return nil, err
	}

	err = service.publishManualTransactionEvent(manualWithdrawal, model.TxStatus_Unconfirmed, wallet.EventType_Deposit)
	if err != nil {
		return nil, err
	}

	sender := fmt.Sprintf("%s %s %s", adminUser.FirstName, adminUser.LastName, adminUser.Email)
	recipient := fmt.Sprintf("%s %s %s", user.FirstName, user.LastName, user.Email)

	userDetails := model.UserDetails{}
	err = service.repo.ConnReaderAdmin.First(&userDetails, "user_id = ?", user.ID).Error
	if err != nil {
		return nil, err
	}

	confirmUrl := service.cfg.Server.ManualTransactions.ConfirmUrl
	err = service.SendEmailForManualWithdrawalRequest(sender, recipient, manualWithdrawal.CoinSymbol, manualWithdrawal.Amount.V.String(), confirmUrl, manualWithdrawal.CreatedAt)
	if err != nil {
		return nil, err
	}

	return manualWithdrawal, nil
}

func (service *Service) ConfirmAdminManualWithdrawal(manualWithdrawalId string, adminUserId uint64) error {
	var manualWithdrawal model.ManualTransaction
	isAdminUserValid := false
	confirmingUserList := make([]model.User, 0)

	db := service.repo.ConnReaderAdmin.Table("manual_transactions").
		Where("id = ?", manualWithdrawalId).
		Where("tx_type = ?", model.TxType_Withdraw).
		Where("status = ?", model.TxStatus_Unconfirmed).
		First(&manualWithdrawal)
	if db.Error != nil {
		return db.Error
	}

	confirmingUserEmailList := service.cfg.Server.ManualTransactions.ConfirmingUsers

	db = service.repo.ConnReaderAdmin.Table("users").
		Where("email IN (?)", confirmingUserEmailList).
		Find(&confirmingUserList)
	if db.Error != nil {
		return db.Error
	}

	for _, user := range confirmingUserList {
		if user.ID == adminUserId {
			isAdminUserValid = true
			break
		}
	}

	if !isAdminUserValid {
		return errors.New("you are not allowed to perform this operation")
	}

	for _, confirmedByUser := range manualWithdrawal.ConfirmedBy {
		if uint64(confirmedByUser) == adminUserId {
			return errors.New("you confirmed this withdrawal already")
		}
	}

	manualWithdrawal.ConfirmedBy = append(manualWithdrawal.ConfirmedBy, int64(adminUserId))
	manualWithdrawal.Confirmations = len(manualWithdrawal.ConfirmedBy)

	db = service.repo.Conn.
		Table("manual_transactions").
		Where("id = ?", manualWithdrawal.ID).
		Save(&manualWithdrawal)
	if db.Error != nil {
		return db.Error
	}

	if len(manualWithdrawal.ConfirmedBy) == len(confirmingUserList) {
		err := service.publishManualTransactionEvent(&manualWithdrawal, model.TxStatus_Confirmed, wallet.EventType_Withdraw)
		if err != nil {
			return err
		}

		db = service.repo.Conn.
			Table("manual_transactions").
			Where("id = ?", manualWithdrawal.ID).
			Save(&manualWithdrawal)
		if db.Error != nil {
			return db.Error
		}
	}

	return nil
}

func (service *Service) GetManualDeposits(limit, page int, status string) (*model.ManualTransactionList, error) {
	transactions := make([]model.ManualTransactionResponse, 0)
	var rowCount int64 = 0
	db := service.repo.ConnReaderAdmin.
		Table("manual_transactions").
		Where("manual_transactions.tx_type = ? ", model.TxType_Deposit)

	if len(status) > 0 {
		db = db.Where("manual_transactions.status = ?", status)
	}

	dbc := db.Select("count(manual_transactions.id) as total").Row()
	_ = dbc.Scan(&rowCount)

	db.
		Joins("LEFT JOIN users u ON manual_transactions.user_id = u.id").
		Select("manual_transactions.*, u.email as email").
		Order("manual_transactions.created_at DESC").
		Limit(limit).Offset((page - 1) * limit).Find(&transactions)
	if db.Error != nil {
		return nil, db.Error
	}

	return &model.ManualTransactionList{
		ManualTransactions: transactions,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}, db.Error
}

func (service *Service) GetManualDepositConfirmingUsers() *model.ManualDepositsConfirmingUsersResponse {
	confirmingUserEmailList := service.cfg.Server.ManualTransactions.ConfirmingUsers

	return &model.ManualDepositsConfirmingUsersResponse{Emails: confirmingUserEmailList}
}

func (service *Service) CreateAdminManualDeposit(createDepositRequest *model.CreateDepositRequest, adminUser *model.User) (*model.ManualTransaction, error) {
	var coin model.Coin
	db := service.repo.ConnReaderAdmin.Where("symbol = ?", createDepositRequest.CoinSymbol).First(&coin)
	if db.Error != nil {
		return nil, db.Error
	}

	var user model.User
	db = service.repo.ConnReaderAdmin.Where("id = ?", createDepositRequest.UserID).First(&user)
	if db.Error != nil {
		return nil, db.Error
	}

	manualDeposit, err := model.NewManualDeposit(createDepositRequest)
	if err != nil {
		return nil, err
	}

	err = service.repo.Conn.Table("manual_transactions").Create(&manualDeposit).Error
	if err != nil {
		return nil, err
	}

	err = service.publishManualTransactionEvent(manualDeposit, model.TxStatus_Unconfirmed, wallet.EventType_Deposit)
	if err != nil {
		return nil, err
	}

	sender := fmt.Sprintf("%s %s %s", adminUser.FirstName, adminUser.LastName, adminUser.Email)
	recipient := fmt.Sprintf("%s %s %s", user.FirstName, user.LastName, user.Email)

	userDetails := model.UserDetails{}
	err = service.repo.ConnReaderAdmin.First(&userDetails, "user_id = ?", user.ID).Error
	if err != nil {
		return nil, err
	}

	confirmUrl := service.cfg.Server.ManualTransactions.ConfirmUrl
	err = service.SendEmailForManualDepositRequest(sender, recipient, manualDeposit.CoinSymbol, manualDeposit.Amount.V.String(), confirmUrl, manualDeposit.CreatedAt)
	if err != nil {
		return nil, err
	}

	return manualDeposit, nil
}

func (service *Service) ConfirmAdminManualDeposit(manualDepositId string, adminUserId uint64) error {
	var manualDeposit model.ManualTransaction
	isAdminUserValid := false
	confirmingUserList := make([]model.User, 0)

	db := service.repo.ConnReaderAdmin.Table("manual_transactions").
		Where("id = ?", manualDepositId).
		Where("tx_type = ?", model.TxType_Deposit).
		Where("status = ?", model.TxStatus_Unconfirmed).
		First(&manualDeposit)
	if db.Error != nil {
		return db.Error
	}

	confirmingUserEmailList := service.cfg.Server.ManualTransactions.ConfirmingUsers

	db = service.repo.ConnReaderAdmin.Table("users").Where("email IN (?)", confirmingUserEmailList).Find(&confirmingUserList)
	if db.Error != nil {
		return db.Error
	}

	for _, user := range confirmingUserList {
		if user.ID == adminUserId {
			isAdminUserValid = true
		}
	}

	if !isAdminUserValid {
		return errors.New("you are not allowed to perform this operation")
	}

	for _, confirmedByUser := range manualDeposit.ConfirmedBy {
		if uint64(confirmedByUser) == adminUserId {
			return errors.New("you confirmed this deposit already")
		}
	}

	manualDeposit.ConfirmedBy = append(manualDeposit.ConfirmedBy, int64(adminUserId))
	manualDeposit.Confirmations = len(manualDeposit.ConfirmedBy)

	db = service.repo.Conn.
		Table("manual_transactions").
		Where("id = ?", manualDeposit.ID).
		Save(&manualDeposit)
	if db.Error != nil {
		return db.Error
	}

	if len(manualDeposit.ConfirmedBy) == len(confirmingUserList) {
		err := service.publishManualTransactionEvent(&manualDeposit, model.TxStatus_Confirmed, wallet.EventType_Deposit)
		if err != nil {
			return err
		}

		db = service.repo.Conn.
			Table("manual_transactions").
			Where("id = ?", manualDeposit.ID).
			Save(&manualDeposit)
		if db.Error != nil {
			return db.Error
		}
	}

	return nil
}

func (service *Service) publishManualTransactionEvent(manualTransaction *model.ManualTransaction, status model.TxStatus, eventType wallet.EventType) error {
	manualTransaction.Status = status

	event := wallet.Event{
		Event:  string(eventType),
		UserID: manualTransaction.UserID,
		ID:     manualTransaction.ID,
		Coin:   manualTransaction.CoinSymbol, //from rq
		Meta:   map[string]string{},
		Payload: map[string]string{
			"confirmations":   strconv.Itoa(manualTransaction.Confirmations),
			"amount":          manualTransaction.Amount.V.String(),
			"fee":             manualTransaction.FeeAmount.V.String(),
			"address":         manualTransaction.Address,
			"status":          manualTransaction.Status.String(),
			"txid":            manualTransaction.TxID,
			"external_system": "manual",
		},
	}

	bytes, err := event.ToBinary()
	if err != nil {
		return err
	}

	message := kafkaGo.Message{Value: bytes}
	err = service.dm.Publish("wallet_events", map[string]string{}, message)
	if err != nil {
		return err
	}

	return nil
}

// GetTransactions - list of all withdrawals or deposits, filtered by type
func (service *Service) GetTransactions(limit, page int, transactionType model.TxType, status, query, coinSymbol string, from, to int) (*model.TransactionWithUserList, error) {
	transactions := make([]model.TransactionWithUser, 0)
	var rowCount int64 = 0

	db := service.repo.ConnReaderAdmin.
		Table("transactions").
		Joins("inner join users on transactions.user_id = users.id").
		Joins("inner join coins on transactions.coin_symbol = coins.symbol").
		Where("transactions.tx_type = ?", transactionType)

	if len(status) > 0 {
		db = db.Where("transactions.status = ?", status)
	}
	if len(query) > 0 {
		squery := "%" + query + "%"
		db = db.Where("users.email LIKE ? OR txid LIKE ? OR address LIKE ?", squery, squery, squery)
	}
	if len(coinSymbol) > 0 {
		db = db.Where("coin_symbol = ?", coinSymbol)
	}
	if from > 0 && to > 0 {
		db = db.Where("date_trunc('day', transaction.created_at) BETWEEN to_timestamp(?) and to_timestamp(?)", from, to)
	}

	dbc := db.Select("count(transactions.id) as total").Row()
	_ = dbc.Scan(&rowCount)

	db.
		Select("users.first_name, users.last_name, users.email, coins.blockchain_explorer, transactions.*").
		Order("transactions.created_at DESC").
		Limit(limit).Offset((page - 1) * limit).Find(&transactions)

	transactionsList := model.TransactionWithUserList{
		Transactions: transactions,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}

	return &transactionsList, db.Error
}

// GetWithdrawsRequests - list of all withdrawals requests  filtered by type
func (service *Service) GetWithdrawsRequests(limit, page, from, to int, status, query string) (*model.WithdrawRequestWithUserList, error) {
	withdraws := make([]model.WithdrawRequestWithUser, 0)
	var rowCount int64 = 0
	db := service.repo.ConnReader.
		Table("withdraw_requests").
		Joins("inner join users on withdraw_requests.user_id = users.id").
		Joins("inner join coins on withdraw_requests.coin_symbol = coins.symbol")

	if model.WithdrawStatus(status).IsValid() {
		db = db.Where("withdraw_requests.status = ?", status)
	}

	if len(query) > 0 {
		squery := "%" + query + "%"
		db = db.Where("users.email LIKE ? OR withdraw_requests.txid LIKE ? OR withdraw_requests.to LIKE ?", squery, squery, squery)
	}

	if from > 0 && to > 0 {
		db = db.Where("date_trunc('day', withdraw_requests.created_at) BETWEEN to_timestamp(?) and to_timestamp(?)", from, to)
	}

	dbc := db.Select("count(withdraw_requests.id) as total").Row()
	_ = dbc.Scan(&rowCount)
	db.
		Select("users.first_name, users.last_name, users.email, coins.blockchain_explorer, withdraw_requests.*").
		Order("withdraw_requests.created_at DESC").
		Limit(limit).Offset((page - 1) * limit).Find(&withdraws)

	withdrawList := model.WithdrawRequestWithUserList{
		WithdrawRequests: withdraws,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}
	return &withdrawList, db.Error
}

// GetUserWithdraws - get user withdraws
func (service *Service) GetUserWithdraws(userID uint64, status string, limit, page int, query string) (*model.WithdrawRequestList, error) {
	withdrawals := make([]model.WithdrawRequestWithUserEmail, 0)
	var rowCount int64 = 0
	q := service.repo.ConnReaderAdmin.Table("withdraw_requests as wr").Where("wr.user_id = ?", userID)

	if len(query) > 0 {
		q = q.Where("wr.coin_symbol = ? or wr.to = ?", query, query)
	}

	if len(status) > 0 {
		q = q.Where("wr.status = ? ", status)
	}

	dbc := q.Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	db := q.Select("wr.*, u.email as email, coins.blockchain_explorer").
		Joins("left join users as u on wr.user_id = u.id").
		Joins("inner join coins on wr.coin_symbol = coins.symbol").
		Order("created_at DESC")
	if limit == 0 {
		db = db.Find(&withdrawals)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&withdrawals)
	}

	withdrawalsList := model.WithdrawRequestList{
		WithdrawRequests: withdrawals,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}

	return &withdrawalsList, db.Error
}

// GetOperations - list of all operations
func (service *Service) GetOperations(limit, page int, status string) (*model.OperationList, error) {
	operations := make([]model.Operation, 0)
	var rowCount int64 = 0

	db := service.repo.ConnReaderAdmin.Table("operations")

	if len(status) > 1 {
		db = db.Where("status = ?", status)
	}

	dbc := db.Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	db.Select("*").Order("id DESC").Group("id").
		Limit(limit).Offset((page - 1) * limit).Find(&operations)

	operationList := model.OperationList{
		Operations: operations,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}

	return &operationList, db.Error
}

// GetCoinStatistics - statistica data of coins incuding assets, liabilities, profit expenses
func (service *Service) GetCoinStatistics() ([]*model.CoinsStats, error) {
	coins := make([]*model.CoinsStats, 0)

	q := service.repo.ConnReaderAdmin.Table("coins_stats as cs").
		Joins("left join revenue_aggr r on cs.coin_symbol = r.coin_symbol").
		Joins("left join coins c on cs.coin_symbol = c.symbol").
		Where("ref_type = ?", model.OperationType_Trade).
		Select("cs.*, c.token_precision, sum(r.credit) as fee").
		Group("cs.coin_symbol, c.token_precision")

	db := q.Find(&coins)
	if db.Error != nil {
		return nil, db.Error
	}

	if len(coins) > 0 {
		coinValues, err := service.coinValues.GetAll()
		if err != nil {
			return nil, err
		}

		for _, ct := range coins {
			index := strings.ToUpper(ct.CoinSymbol)
			ct.BTCValue = coinValues[index]["BTC"].String()
		}
	}
	return coins, db.Error
}

// UpdateAdminProfile - update admin user's details
func (service *Service) UpdateAdminProfile(id uint64, firstName, lastName, country, phone, address, city, state, postalCode string, dob *time.Time, gender model.GenderType) (*model.User, error) {
	user := model.User{}
	db := service.repo.Conn.First(&user, "id = ?", id)
	if db.Error != nil {
		return nil, db.Error
	}

	userDetails := model.UserDetails{}
	db = service.repo.Conn.First(&userDetails, "user_id = ?", id)
	if db.Error != nil {
		return nil, db.Error
	}

	userSettings := model.UserSettings{}
	db = service.repo.Conn.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		return nil, db.Error
	}

	user.FirstName = firstName
	user.LastName = lastName

	userDetails.DOB = dob
	userDetails.Country = country
	userDetails.Phone = phone
	userDetails.Gender = gender
	userDetails.Address = address
	userDetails.City = city
	userDetails.State = state
	userDetails.PostalCode = postalCode

	tx := db.Begin()
	if err := tx.Error; err != nil {
		return nil, err
	}

	// save the data
	if err := tx.Save(user).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Save(userDetails).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Save(userSettings).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	// commit the transaction and return the new data
	return &user, tx.Commit().Error
}

// UpdateUserPassword - set a users password by admin
func (service *Service) UpdateUserPassword(user *model.User, password string) (*model.User, error) {
	user.Password = password
	err := user.EncodePass()
	if err != nil {
		return nil, err
	}
	err = service.repo.Update(user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetRolesWithStats - get roles for specified scope
func (service *Service) GetRolesWithStats(scope string, limit, page int) (model.RoleWithStatsList, error) {
	roles := make([]model.RoleWithStats, 0)
	var rowCount int64 = 0

	db := service.repo.ConnReaderAdmin.
		Table("roles").
		Where("roles.scope = ?", scope)

	dbc := db.Select("count(roles.alias) as total").Row()
	_ = dbc.Scan(&rowCount)

	db = db.
		Joins("inner join role_has_permissions on role_has_permissions.role_alias = roles.alias").
		Select("COUNT(role_has_permissions.role_alias) as permissions_count, roles.*").
		Group("roles.alias").
		Order("roles.alias ASC")
	if limit > 0 {
		db = db.Limit(limit).Offset((page - 1) * limit)
	}
	db.Find(&roles)

	rolesList := model.RoleWithStatsList{
		Roles: roles,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}

	return rolesList, db.Error
}

// GetPermissionInsertQuery
func (service *Service) GetPermissionInsertQuery(permissions []string, roleAlias string) (string, []interface{}) {
	// create bulk insert query
	valueStrings := []string{}
	valueArgs := []interface{}{}

	for _, f := range permissions {
		valueStrings = append(valueStrings, "(?, ?)")
		valueArgs = append(valueArgs, roleAlias)
		valueArgs = append(valueArgs, f)
	}

	query := `INSERT INTO role_has_permissions(role_alias, permission_alias) VALUES %s`
	query = fmt.Sprintf(query, strings.Join(valueStrings, ","))

	return query, valueArgs
}

// AddUserRoleWithPermissions - add a user role
func (service *Service) AddUserRoleWithPermissions(scope, roleAlias, name string, permissions []string) (*model.Role, error) {
	role := model.NewRole(scope, roleAlias, name)

	db := service.repo.Conn
	tx := db.Begin()
	if err := tx.Error; err != nil {
		return nil, err
	}

	// create role
	if err := tx.Create(role).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// insert current list
	query, valueArgs := service.GetPermissionInsertQuery(permissions, roleAlias)
	if err := tx.Exec(query, valueArgs...).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// commit the transaction and return the new data
	return role, tx.Commit().Error
}

// UpdateUserRoleWithPermissions - update role permissions
func (service *Service) UpdateUserRoleWithPermissions(roleAlias, name string, permissions []string) (*model.Role, error) {
	role, _ := service.GetRoleByAlias(roleAlias)
	role.Name = name

	// get current permission list
	userPermissions, err := service.GetPermissionsByRoleAlias(roleAlias)
	if err != nil {
		return nil, err
	}

	permissionsList := map[string]string{}
	for _, perm := range userPermissions {
		permissionsList[perm.Alias] = perm.Name
	}

	db := service.repo.Conn
	tx := db.Begin()
	if err := tx.Error; err != nil {
		return nil, err
	}

	// update role
	if err := tx.Save(role).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// remove disabled permissions
	type RoleHasPermissions struct {
		RoleAlias        string
		PermissionAlilas string
	}
	if err := tx.Where("role_alias = ? AND permission_alias NOT IN (?)", roleAlias, permissions).Delete(RoleHasPermissions{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// insert current list
	permissionsToAdd := []string{}
	for _, perm := range permissions {
		if _, ok := permissionsList[perm]; !ok {
			permissionsToAdd = append(permissionsToAdd, perm)
		}
	}
	if len(permissionsToAdd) > 0 {
		query, valueArgs := service.GetPermissionInsertQuery(permissionsToAdd, roleAlias)
		if err := tx.Set("gorm:insert_option", "ON CONFLICT").Exec(query, valueArgs...).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// commit the transaction and return the new data
	return role, tx.Commit().Error
}

// RemoveUserRole - remove role and set 'member' role to all users having removed role
func (service *Service) RemoveUserRole(roleAlias string) (*model.Role, error) {
	roleAlias_ := model.RoleAlias(roleAlias)
	if !roleAlias_.IsBaseRole() {
		return nil, errors.New("Cannot remove base roles")
	}

	role, err := service.GetRoleByAlias(roleAlias)
	if err != nil {
		return nil, err
	}

	db := service.repo.Conn
	tx := db.Begin()
	if err := tx.Error; err != nil {
		return nil, err
	}

	// remove permissions of role
	type RoleHasPermissions struct {
		RoleAlias        string
		PermissionAlilas string
	}
	if err := tx.Where("role_alias = ? ", roleAlias).Delete(RoleHasPermissions{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// update users
	if err := tx.Table("users").Where("role_alias = ?", roleAlias).Select("role_alias").Updates(map[string]interface{}{"role_alias": "member"}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// remove role
	if err := tx.Delete(role).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// commit the transaction and return the new data
	return role, tx.Commit().Error
}

func (service *Service) GetPRDXCirculation() (*model.PRDXCirculation, error) {
	prdxCirculation := new(model.PRDXCirculation)
	mmUsersList := make([]model.User, 0)
	number, err := service.GetDistributedBonus()
	if err != nil {
		return nil, err
	}

	prdxCoin, _ := coins.Get("prdx")

	mmAccounts := service.cfg.Server.MMAccounts
	db := service.repo.ConnReaderAdmin.Table("users").Where("email IN (?)", mmAccounts).Find(&mmUsersList)
	if db.Error != nil {
		return nil, db.Error
	}
	for _, user := range mmUsersList {
		account, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, user.ID, model.AccountGroupMain)
		if err != nil {
			return nil, err
		}

		userValue, err := service.repo.GetUserTotalBalanceForCoin(user.ID, prdxCoin.Symbol, account)
		if err != nil {
			return nil, err
		}
		accountMm := model.AccountMMValue{Email: user.Email, Value: userValue}
		prdxCirculation.AccountsMM = append(prdxCirculation.AccountsMM, accountMm)
	}

	prdxDistributionEmail := "prdxdistribution@paramountdax.com"
	prdxCirculationUser := new(model.User)

	db = service.repo.ConnReaderAdmin.Table("users").Where("email = ?", prdxDistributionEmail).First(&prdxCirculationUser)
	if db.Error != nil {
		return nil, db.Error
	}
	account, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, prdxCirculationUser.ID, model.AccountGroupMain)
	if err != nil {
		return nil, err
	}
	accountDistribution, err := service.repo.GetUserTotalBalanceForCoin(prdxCirculationUser.ID, prdxCoin.Symbol, account)
	if err != nil {
		return nil, err
	}

	_, body, err := httpagent.Get(service.cfg.Server.BurnedTokenApi.Url + "/supply/circulating")
	if err != nil {
		return nil, err
	}
	strBody := string(body)
	totalCirculation, _ := conv.NewDecimalWithPrecision().SetString(strings.Trim(strBody, "\""))

	_, body, err = httpagent.Get(service.cfg.Server.BurnedTokenApi.Url + "/supply/total")
	if err != nil {
		return nil, err
	}
	totalOnChain, _ := conv.NewDecimalWithPrecision().SetString(strings.Trim(string(body), "\""))

	_, body, err = httpagent.Get(service.cfg.Server.BurnedTokenApi.Url + "/burned-token-history/info")
	if err != nil {
		return nil, err
	}
	burnedTokensInfo := new(model.BurnedTokensInfo)
	err = json.Unmarshal(body, &burnedTokensInfo)
	if err != nil {
		return nil, err
	}

	totalExchange := &postgres.Decimal{V: model.Zero}
	err = service.repo.ConnReaderAdmin.Table("balances").
		Select("coalesce(SUM(available + locked), 0.0) as s").
		Where("sub_account = ?", 0).
		Where("coin_symbol = ?", prdxCoin.Symbol).
		Row().Scan(&totalExchange)
	if err != nil {
		return nil, err
	}

	totalUsersExchange := &postgres.Decimal{V: model.Zero}
	err = service.repo.ConnReaderAdmin.Table("balances as b").
		Select("coalesce(SUM(b.available + b.locked), 0.0) as s").
		Joins("left join users as u ON b.user_id = u.id").
		Where("b.sub_account = ?", 0).
		Where("b.coin_symbol = ?", prdxCoin.Symbol).
		Where("u.role_alias IN (?)", []string{model.Member.String(), model.Admin.String(), model.Business.String(), model.Broker.String()}).
		Row().Scan(&totalUsersExchange)
	if err != nil {
		return nil, err
	}

	for _, mmAccountValue := range prdxCirculation.AccountsMM {
		totalUsersExchange.V = totalUsersExchange.V.Sub(totalUsersExchange.V, mmAccountValue.Value)
	}

	recoveryAccountId := service.cfg.Server.ManualDistributionRecoveryAccountId
	account, err = subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, recoveryAccountId, model.AccountGroupMain)
	if err != nil {
		return nil, err
	}

	notClaimedPrdx, err := service.repo.GetUserTotalBalanceForCoin(recoveryAccountId, prdxCoin.Symbol, account)
	if err != nil {
		return nil, err
	}

	prdxDistributorUserEmail := service.cfg.Server.PrdxDistributorUser
	prdxDistributorUser := new(model.User)
	db = service.repo.ConnReaderAdmin.Table("users").Where("email = ?", prdxDistributorUserEmail).First(&prdxDistributorUser)
	if db.Error != nil {
		return nil, db.Error
	}
	account, err = subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, prdxDistributorUser.ID, model.AccountGroupMain)
	if err != nil {
		return nil, err
	}
	prdxDistributor, err := service.repo.GetUserTotalBalanceForCoin(recoveryAccountId, prdxCoin.Symbol, account)
	if err != nil {
		return nil, err
	}

	prdxUnusedFoundsUserEmail := service.cfg.Server.PrdxDistributorUser
	prdxUnusedFoundsUser := new(model.User)
	db = service.repo.ConnReaderAdmin.Table("users").Where("email = ?", prdxUnusedFoundsUserEmail).First(&prdxUnusedFoundsUser)
	if db.Error != nil {
		return nil, db.Error
	}
	account, err = subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, prdxUnusedFoundsUser.ID, model.AccountGroupMain)
	if err != nil {
		return nil, err
	}
	unusedPrdx, err := service.repo.GetUserTotalBalanceForCoin(recoveryAccountId, prdxCoin.Symbol, account)
	if err != nil {
		return nil, err
	}

	prdxCirculation.TotalCirculation = totalCirculation
	prdxCirculation.TotalOnChain = totalOnChain
	prdxCirculation.TotalOnExchange = totalExchange.V
	prdxCirculation.AccountRecoverNotClaimed = notClaimedPrdx
	prdxCirculation.TotalBurned = burnedTokensInfo.TotalBurned
	prdxCirculation.TotalDistributed = number
	prdxCirculation.AccountDistribution = accountDistribution
	prdxCirculation.TotalUserOnExchange = totalUsersExchange.V
	prdxCirculation.PrdxDistributor = prdxDistributor
	prdxCirculation.UnusedPrdx = unusedPrdx

	return prdxCirculation, nil
}

func (service *Service) SetPriceLimits(minPrice string, maxPriceDec string) error {

	tx := service.repo.Conn.Begin()

	db := tx.Table("admin_feature_settings").Where("feature = ?", model.MIN_MARKET_PRICE_FEATURE_KEY).Update("value", minPrice)
	if db.Error != nil {
		tx.Rollback()
		return db.Error
	}

	db = tx.Table("admin_feature_settings").Where("feature = ?", model.MAX_MARKET_PRICE_FEATURE_KEY).Update("value", maxPriceDec)
	if db.Error != nil {
		tx.Rollback()
		return db.Error
	}

	db = tx.Commit()
	if db.Error != nil {
		return db.Error
	}

	return nil
}

func (service *Service) GetPriceLimits() (*model.PriceLimitResponse, error) {
	minPriceDec, maxPriceDec, err := service.repo.GetPriceLimits()
	if err != nil {
		return nil, err
	}

	return &model.PriceLimitResponse{
		MinPrice: minPriceDec,
		MaxPrice: maxPriceDec,
	}, nil
}

func (service *Service) GetBonusAccountContractsHistoryAdmin(pair, status string, from, to int) ([]*model.BonusAccountContractHistory, error) {
	return service.repo.GetBonusAccountContractsHistoryAdmin(pair, status, from, to)
}

func (service *Service) GetBonusAccountContractsHistoryByContractIDAdmin(contractID uint64) (*model.BonusAccountContractHistory, error) {
	return service.repo.GetBonusAccountContractsHistoryByContractIDAdmin(contractID)
}

func (service *Service) UpdateVolumeDistributedPercent(percent string) error {
	err := service.repo.Conn.Table("admin_feature_settings").Where("feature = ?", model.MANUAL_DISTRIBUTION_PERCENT_FEATURE_KEY).Update("value", percent).Error
	if err != nil {
		return err
	}
	return nil
}

func (service *Service) UpdateAnnouncementsSettings(settings *model.AnnouncementsSettingsSchema) error {
	logger := log.With().
		Str("section", "admin").
		Str("service", "UpdateAnnouncementsSettings").
		Logger()

	err := service.repo.UpdateAnnouncementSettings(settings)
	if err != nil {
		logger.Error().Err(err).Msg("UpdateAnnouncementSettings error")
		return err
	}

	return nil
}

func (service *Service) GetAnnouncementsSettings() (*model.AnnouncementsSettingsSchema, error) {
	logger := log.With().
		Str("section", "admin").
		Str("service", "GetAnnouncementsSettings").
		Logger()

	settings, err := service.repo.GetAnnouncementSettings()
	if err != nil {
		logger.Error().Err(err).Msg("GetAnnouncementSettings error")
		return nil, err
	}

	return settings, nil
}
