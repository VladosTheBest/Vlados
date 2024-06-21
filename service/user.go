package service

import (
	"errors"
	"github.com/segmentio/encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/user_fees"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/user_referrals"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"

	"github.com/ericlagergren/decimal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gorm.io/gorm"
)

// GetUserByEmail - get an user with userEmail email
func (service *Service) GetUserByEmail(userEmail string) (*model.User, error) {
	user := model.User{}
	db := service.repo.ConnReader.First(&user, "email = ?", userEmail)
	if db.Error != nil {
		return nil, db.Error
	}
	return &user, nil

}

func (service *Service) GetUserByPhone(userPhone string) (*model.User, error) {
	// Initialize an empty user struct
	user := model.User{}
	// Connect to the database using the connection from the repository
	// and retrieve the user with the provided phone number
	db := service.repo.ConnReader.First(&user, "phone = ?", userPhone)
	// Check for errors
	if db.Error != nil {
		// Return nil and the error if there was an error
		return nil, db.Error
	}
	// Return the user and nil error if there was no error
	return &user, nil
}

func (service *Service) GetUserCountryByIP(ip string) (string, error) {
	resp, err := http.Get("http://ip-api.com/json/" + ip)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	type GeoIP struct {
		CountryCode string `json:"countryCode"`
	}

	var geo GeoIP
	err = json.NewDecoder(resp.Body).Decode(&geo)
	if err != nil {
		return "", err
	}

	return geo.CountryCode, nil
}

// GetUserByID - get an user by a specific id
func (service *Service) GetUserByID(id uint) (*model.User, error) {
	user := model.User{}
	db := service.repo.ConnReader.First(&user, "id = ?", id)
	if db.Error != nil {
		return nil, db.Error
	}
	return &user, nil
}

// GetUserFeesRow godoc
func (service *Service) GetUserFeesRow(userID uint64) (*model.UserFee, error) {
	fees, found := cache.Get(userID)
	if found {
		return fees, nil
	}

	return service.repo.GetUserFeesRow(userID)
}

func (service *Service) ExportUserBalances() (*model.GeneratedFile, error) {
	data := [][]string{}
	data = append(data, []string{"ID", "Available", "Locked"})
	userBalances := make([]model.UserBalance, 0)
	crossRates, err := service.coinValues.GetAll()
	if err != nil {
		return nil, err
	}

	db := service.repo.ConnReader.Table("users u").
		Select("u.email, b.available, b.locked, b.coin_symbol").
		Joins("LEFT JOIN balances b ON u.id = b.user_id").
		Find(&userBalances)

	collectedBalances := make(map[string]map[string]*decimal.Big)
	if db.Error != nil {
		return nil, db.Error
	}

	for _, userBalance := range userBalances {
		if collectedBalances[userBalance.Email]["available"] == nil {
			collectedBalances[userBalance.Email]["available"] = conv.NewDecimalWithPrecision()
		}
		if collectedBalances[userBalance.Email]["locked"] == nil {
			collectedBalances[userBalance.Email]["locked"] = conv.NewDecimalWithPrecision()
		}

		crossRate := crossRates[strings.ToUpper(userBalance.CoinSymbol)]["USDT"]
		available := conv.NewDecimalWithPrecision().Mul(&userBalance.Available, crossRate)
		locked := conv.NewDecimalWithPrecision().Mul(&userBalance.Locked, crossRate)

		collectedBalances[userBalance.Email]["available"] = available.Add(collectedBalances[userBalance.Email]["available"], available)
		collectedBalances[userBalance.Email]["locked"] = available.Add(collectedBalances[userBalance.Email]["locked"], locked)
	}

	for userEmail, user := range collectedBalances {
		data = append(data, []string{userEmail, user["available"].String(), user["locked"].String()})
	}

	resp, err := CSVExport(data)
	if err != nil {
		return nil, err
	}
	generatedFile := model.GeneratedFile{
		Type:     "csv",
		DataType: "user balances",
		Data:     resp,
	}

	return &generatedFile, nil
}

// RegisterUser - register a new user
func (service *Service) RegisterUser(tx *gorm.DB, firstName, lastName, email, phone, password, accountType, role, referralCode, leadFromResource string) (*model.User, error) {
	// create user entity
	user := model.NewUser(accountType, firstName, lastName, email, phone, password, role, referralCode)
	_ = user.EncodePass()

	// if no transaction is received create a new one
	if tx == nil {
		tx = service.repo.Conn.Begin()
	}
	// persist the user
	db := tx.Create(user)

	if db.Error != nil {
		tx.Rollback()
		return nil, db.Error
	}
	// save user setting
	userSettings := model.NewUserSettings(user.ID, "f", "t", "", "", "", "", 0)
	db = tx.Create(userSettings)
	if db.Error != nil {
		tx.Rollback()
		return nil, db.Error
	}
	pushNotificationSettings := model.NewPushNotificationSettings(user.ID)
	db = tx.Create(pushNotificationSettings)
	if db.Error != nil {
		tx.Rollback()
		return nil, db.Error
	}
	// save user fees
	fee := model.NewUserFee(user.ID, service.cfg.Fee)
	db = tx.Create(fee)
	if db.Error != nil {
		tx.Rollback()
		return nil, db.Error
	}
	// save user details
	userDetails := model.NewUserDetails(user.ID, phone, "", "", "", "", "", model.GenderTypeMale, model.LanguageCodeEnglish, nil, leadFromResource)
	db = tx.Create(userDetails)
	if db.Error != nil {
		tx.Rollback()
		return nil, db.Error
	}

	pd := model.NewUserPaymentDetails(user.ID)
	db = tx.Create(&pd)
	if db.Error != nil {
		tx.Rollback()
		return nil, db.Error
	}

	// commit the transaction
	db = tx.Commit()
	if db.Error != nil {
		return nil, db.Error
	}

	var referredUser model.User
	if err := service.GetRepo().ConnReader.Where("referral_code = ?", referralCode).First(&referredUser).Error; err == nil {
		user_referrals.AddReferralUser(referredUser.ID, user)
	}

	// return the new created user
	return user, nil
}

// UserChangePassword - update a user's password
func (service *Service) UserChangePassword(user *model.User, pass string) (*model.User, error) {
	user.Password = pass
	_ = user.EncodePass()
	err := service.repo.Update(user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// ListUsers - list all users in the database
func (service *Service) ListUsers(page, limit int, query, filter string) (*model.UserList, error) {
	users := make([]model.UserWithKycStatus, 0)
	var rowCount int64 = 0

	db := service.repo.ConnReader.Table("users as u").
		Select("u.*, kycs.status").
		Joins("left join kycs on u.kyc_id = kycs.id").
		Where("u.email_status = ?", model.UserEmailStatusAllowed).
		Where("u.status != ?", model.UserStatusDeleted).
		Where("u.status != ?", model.UserStatusRemoved)

	if len(filter) > 0 && filter == "pending_kyc" {
		db = db.Where("(kycs.status = ? OR kycs.status = ?)", model.KYCStatusStepTwoPending, model.KYCStatusStepThreePending)
	}
	if len(filter) > 0 && filter == "pending_deposit" {
		db = db.Joins("inner join transactions as t on u.id = t.user_id").
			Where("t.tx_type  = ?", model.TxType_Deposit).
			Where("t.status = ?", model.TxStatus_Pending)
	}
	if len(filter) > 0 && filter == "pending_withdraw" {
		db = db.Joins("inner join withdraw_requests as w on u.id = w.user_id").
			Where("w.status = ?", model.TxStatus_Pending)
	}
	if len(filter) > 0 {
		db = db.Group("kycs.status, u.id")
	}
	if len(query) > 0 {
		queryp := "%" + query + "%"
		db = db.Where("u.first_name LIKE ? OR u.last_name LIKE ? OR u.email LIKE ?", queryp, queryp, queryp)
	}
	dbc := db.Count(&rowCount)

	if dbc.Error != nil {
		return nil, dbc.Error
	}

	db = db.Order("u.id ASC").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&users)

	if db.Error != nil {
		return nil, db.Error
	}
	// todo: temporary solution for users without kyc registation
	for k, user := range users {
		if user.KycID == nil {
			user.KycStatus = string(model.KYCStatusNone)
			users[k] = user
		}
	}
	userList := model.UserList{
		Users: users,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}
	userList.Meta.Filter["status"] = "users"
	return &userList, db.Error
}

// ConfirmUserEmail - confirm the email address for a user if not already confirmed
func (service *Service) ConfirmUserEmail(email string) error {
	user, err := service.GetUserByEmail(email)
	if err != nil {
		return err
	}
	if user.Status == model.UserStatusPending {
		user.Status = model.UserStatusActive
		return service.repo.Update(user)
	}
	return errors.New("Email address already confirmed")
}

// ConfirmUserPhone - confirm the phone number
func (service *Service) ConfirmUserPhone(user *model.User) error {
	// Check if the user's status is pending
	if user.Status == model.UserStatusPending {
		// Update the user's status to active
		user.Status = model.UserStatusActive
		// Save the updated user to the repository
		return service.repo.Update(user)
	}

	return nil
}

// UpdateUserStatus - Update status of a user
func (service *Service) UpdateUserStatus(user *model.User, status model.UserStatus) (*model.User, error) {
	user.Status = status
	err := service.repo.Update(user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateUserFirstLogin - Update status of a user
func (service *Service) UpdateUserFirstLogin(user *model.User, firstLogin bool) (*model.User, error) {
	user.FirstLogin = firstLogin
	err := service.repo.Update(user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateUserFees godoc
func (service *Service) UpdateUserFees(userID uint64, discountable bool, takerFee, makerFee *decimal.Big) error {
	return service.repo.UpdateUserFees(userID, discountable, takerFee, makerFee)
}

// UsersInfo structure
type UsersInfo struct {
	TotalNumberOfUsers     uint64 `json:"total_number_of_users"`
	UsersPendingKYC        uint64 `json:"users_pending_kyc"`
	UsersPendingDeposit    uint64 `json:"users_pending_deposit"`
	UsersPendingWithdrawal uint64 `json:"users_pending_withdrawal"`
}

// NewUsersInfo creates a new UsersInfo structure
func (service *Service) NewUsersInfo(total, pendingKYC, pendingDeposit, pendingWithdrawal uint64) *UsersInfo {
	return &UsersInfo{
		TotalNumberOfUsers:     total,
		UsersPendingKYC:        pendingKYC,
		UsersPendingDeposit:    pendingDeposit,
		UsersPendingWithdrawal: pendingWithdrawal}
}

// GetTotalNumberOfUsers - get total number of users
func (service *Service) GetTotalNumberOfUsers() (uint64, error) {
	var count int64
	db := service.repo.ConnReader.Table("users").Count(&count)
	if db.Error != nil {
		return 0, db.Error
	}
	return uint64(count), nil
}

// GetTotalNumberOfUsersPendingKyc - get total of users pending KYC
func (service *Service) GetTotalNumberOfUsersPendingKyc() (uint64, error) {
	var count int64
	db := service.repo.ConnReader.Table("users").
		Joins("left join kycs on users.kyc_id = kycs.id").
		Where("kycs.status = ?", model.KYCStatusStepTwoPending).
		Or("kycs.status = ?", model.KYCStatusStepThreePending).
		Count(&count)
	if db.Error != nil {
		return 0, db.Error
	}
	return uint64(count), nil
}

// GetTotalNumberOfUsersPendingTransactions - get total of users pending Deposits/Withdraws
func (service *Service) GetTotalNumberOfUsersTransactions(transactionStatus model.TxStatus, transactionType model.TxType) (uint64, error) {
	var count int64
	db := service.repo.ConnReader.Table("users").
		Joins("inner join transactions on users.id = transactions.user_id").
		Where("transactions.status = ?", transactionStatus).
		Where("transactions.tx_type = ?", transactionType).
		Group("users.id").
		Count(&count)
	if db.Error != nil {
		return 0, db.Error
	}
	return uint64(count), nil
}

// GetUsersInfo
func (service *Service) GetUsersInfo() (*UsersInfo, error) {
	total, err := service.GetTotalNumberOfUsers()
	if err != nil {
		return nil, err
	}
	pendingKYC, err := service.GetTotalNumberOfUsersPendingKyc()
	if err != nil {
		return nil, err
	}
	pendingDeposit, err := service.GetTotalNumberOfUsersTransactions(model.TxStatus_Pending, model.TxType_Deposit)
	if err != nil {
		return nil, err
	}
	pendingWithdraw, err := service.GetTotalNumberOfUsersTransactions(model.TxStatus_Pending, model.TxType_Withdraw)
	if err != nil {
		return nil, err
	}
	return service.NewUsersInfo(total, pendingKYC, pendingDeposit, pendingWithdraw), nil
}

// UserDetails struct
type UserDetails struct {
	FirstName           string             `form:"first_name" json:"first_name" binding:"required"`
	LastName            string             `form:"last_name" json:"last_name" binding:"required"`
	Email               string             `json:"email"`
	RoleAlias           string             `json:"role_alias"`
	ReferralCode        string             `json:"referral_code"`
	DOB                 *time.Time         `form:"dob" json:"dob" binding:"required"`
	Gender              string             `json:"gender"`
	Status              string             `json:"status"`
	Phone               string             `form:"phone" json:"phone" binding:"required"`
	Address             string             `form:"address" json:"address" binding:"required"`
	Country             string             `form:"country" json:"country" binding:"required"`
	State               string             `form:"state" json:"state"  binding:"required"`
	City                string             `form:"city" json:"city"  binding:"required"`
	PostalCode          string             `form:"postal_code" json:"postal_code"  binding:"required"`
	Language            model.LanguageCode `form:"language" json:"language"  binding:"required"`
	FeesPayedWithPrdx   bool               `json:"fees_payed_with_prdx"`
	DetectIPChange      bool               `json:"detect_ip_change"`
	UserLevel           int                `json:"user_level"`
	AntiPhishingKey     string             `json:"-"`
	GoogleAuthKey       string             `json:"-"`
	TradePassword       string             `json:"-"`
	SmsAuthKey          string             `json:"-"`
	ShowPayWithPrdx     bool               `json:"show_pay_with_prdx"`
	LastLogin           string             `json:"last_login"`
	TradePasswordExists bool               `json:"trade_password_exists"`
	LoginPasswordExists bool               `json:"login_password_exists"`
	Google2FaExists     bool               `json:"google_2fa_exists"`
	SMS2FaExists        bool               `json:"sms_2fa_exists"`
	AntiPhishingExists  bool               `json:"anti_phishing_exists"`
}

// GetUserByIDWithDetails - return user with all details
func (service *Service) GetUserByIDWithDetails(userID uint) (*UserDetails, error) {
	user := UserDetails{}
	db := service.repo.ConnReader.Table("users").
		Select("users.first_name, users.status, users.last_name, users.email, users.role_alias, "+
			"users.referral_code,user_details.dob, user_details.phone, user_details.gender, user_details.address, "+
			"user_details.address,user_details.country, user_details.state, user_details.city, user_details.postal_code, "+
			"user_settings.anti_phishing_key, user_settings.google_auth_key, user_settings.sms_auth_key, user_settings.show_pay_with_prdx, "+
			"user_settings.user_level, user_settings.fees_payed_with_prdx, user_settings.detect_ip_change, user_settings.trade_password").
		Joins("inner join user_details on user_details.user_id = users.id").
		Joins("inner join user_settings on user_settings.user_id = users.id").
		Where("users.id  = ?", userID).
		Find(&user)
	if db.Error != nil {
		return nil, db.Error
	}
	return &user, nil
}

// UpdateUserByAdmin - update a user's details
func (service *Service) UpdateUserByAdmin(user *model.User, firstName, lastName, country, phone, address, city, state, postalCode string, level int, dob *time.Time, status model.UserStatus, gender model.GenderType, role string, emailStatus model.UserEmailStatus) (*model.User, *model.KYC, error) {

	tx := service.repo.Conn.Begin()
	if err := tx.Error; err != nil {
		return nil, nil, err
	}

	userDetails := model.UserDetails{}
	if err := tx.First(&userDetails, "user_id = ?", user.ID).Error; err != nil {
		return nil, nil, err
	}

	userSettings := model.UserSettings{}
	if err := tx.First(&userSettings, "user_id = ?", user.ID).Error; err != nil {
		return nil, nil, err
	}

	userKyc := model.KYC{}
	if user.KycID != nil {
		if err := tx.First(&userKyc, "id = ?", user.KycID).Error; err != nil {
			return nil, nil, err
		}

		userKyc.Status = service.GetKYCStatusByUserLevel(level, userKyc.Status)

		if err := service.AddLeadBonusForKYC(tx, &userKyc, user, &userDetails); err != nil {
			tx.Rollback()
			return nil, nil, err
		}
	}

	user.FirstName = firstName
	user.LastName = lastName
	user.Status = status
	user.RoleAlias = role
	user.EmailStatus = emailStatus

	userSettings.UserLevel = level

	userDetails.DOB = dob
	userDetails.Country = country
	userDetails.Phone = phone
	userDetails.Gender = gender
	userDetails.Address = address
	userDetails.City = city
	userDetails.State = state
	userDetails.PostalCode = postalCode

	// save the data
	if err := tx.Save(user).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	if err := tx.Save(userDetails).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	if err := tx.Save(userSettings).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	if user.KycID != nil {
		if err := tx.Save(userKyc).Error; err != nil {
			tx.Rollback()
			return nil, nil, err
		}
	}
	// commit the transaction and return the new data
	return user, &userKyc, tx.Commit().Error
}

// UpdateUserPhone - update  user details phone
func (service *Service) UpdateUserPhone(userID uint64, phone string) error {
	userDetails := model.UserDetails{}
	updateDB := service.repo.Conn.Model(&userDetails).Where("user_id = ?", userID).Update("phone", phone)
	return updateDB.Error
}

func (service *Service) SetInitSubAccounts() {

	subAccounts.InitCacheRepo(service.repo)

	var usersId []uint64

	rows, err := service.repo.ConnReader.Table("users").Select("id").Rows()
	if err != nil {
		log.Error().Err(err).Msg("Unable to load user id's list")
		return
	}

	for rows.Next() {
		var userId uint64
		err = rows.Scan(&userId)
		if err != nil {
			log.Error().Err(err).Msg("Unable to scan user id")
			return
		}

		usersId = append(usersId, userId)
	}

	subAccounts.InitDefaultUsers(usersId)
}

func (service *Service) SetInitUserReferrals() {
	userReferrals, err := service.repo.GetAllUserReferrals()
	if err != nil {
		log.Error().Err(err).Msg("Unable to get user referrals")
		return
	}
	userReferred, err := service.repo.GetAllUserReferred()
	if err != nil {
		log.Error().Err(err).Msg("Unable to get user referred")
		return
	}

	user_referrals.SetUserReferralData(userReferrals, userReferred)
}

func (service *Service) GetTotalUserLineLevels() (*model.TotalUserLineLevelsList, error) {
	totalUserLineLevels := make([]model.TotalUserLineLevels, 0)

	db := service.repo.ConnReader.Table("user_fees").
		Select("count(user_id) as count, level as user_level").
		Group("user_level").Find(&totalUserLineLevels)

	if db.Error != nil {
		return nil, db.Error
	}

	return &model.TotalUserLineLevelsList{
		LineLevels: totalUserLineLevels,
	}, nil
}

func (service *Service) GetUserPaymentDetails(userID uint64) (*model.UserPaymentDetails, error) {
	return service.repo.GetUserPaymentDetails(userID)
}

func (service *Service) GetUserDetails(userID uint64) (model.UserDetails, error) {
	return service.repo.GetUserDetails(userID)
}

func (service *Service) GetKYCUserDetails(userID uint64) (*model.KycCustomerInfo, error) {
	userKycDetails, err := service.repo.GetKycUserDetailsById(userID)
	if err != nil {
		return nil, err
	}

	return userKycDetails, err
}

func (service *Service) UpdateUserKycData(userData *model.KycCustomerRegistrationRequest) error {
	return service.repo.UpdateUserKycDetails(userData)
}

func (service *Service) GetUserByUserBusinessDetails(userBusinessDetailsID uint64) (*model.User, error) {
	userID, err := service.repo.GetUserIDByBusinessDetailsID(userBusinessDetailsID)
	if err != nil {
		return nil, err
	}

	user, err := service.repo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (service *Service) GetUserByUserID(userID uint64) (*model.User, error) {
	user, err := service.repo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	return user, nil
}
