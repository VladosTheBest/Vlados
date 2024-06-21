package service

import (
	"errors"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetProfile - get a user's profile
// @todo CH: Only use get profile for all user profile information and eliminate the rest of the details/settings methods
func (service *Service) GetProfile(user *model.User) (*model.UserWithSettings, error) {
	userActivity := model.UserActivity{}
	userSettings := model.UserSettings{}
	userWithSettings := model.UserWithSettings{}

	db := service.repo.ConnReader.Where("user_id = ?", user.ID).Order("created_at desc").Find(&userActivity)

	if db.Error != nil {
		return nil, db.Error
	}

	db = service.repo.ConnReader.First(&userSettings, "user_id = ?", user.ID)
	if db.Error != nil {
		return nil, db.Error
	}

	// userDetails := model.UserDetails{}
	// db := service.repo.Conn.First(&userDetails, "user_id = ?", user.ID)
	// if db.Error != nil {
	// 	return nil, db.Error
	// }

	userWithSettings.User = user
	userWithSettings.Settings = &userSettings
	// userWithSettings.UserDetails = userDetails
	userWithSettings.LoginPasswordExists = len(user.Password) > 0
	userWithSettings.TradePasswordExists = len(userSettings.TradePassword) > 0
	userWithSettings.AntiPhishingExists = len(userSettings.AntiPhishingKey) > 0
	userWithSettings.Google2FaExists = len(userSettings.GoogleAuthKey) > 0
	userWithSettings.SMS2FaExists = len(userSettings.SmsAuthKey) > 0
	userWithSettings.LastLogin = userActivity.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z")
	userWithSettings.Verifications.Type = true
	userWithSettings.Verifications.Kyc = userSettings.UserLevel >= 2
	userWithSettings.Verifications.Account = string(user.Status) != "pending"
	userWithSettings.Verifications.Tfa = (userSettings.GoogleAuthKey != "" || userSettings.SmsAuthKey != "")

	return &userWithSettings, nil
}

// GetProfileDetails - get a user's profile details
// @deprecated CH: Remove this and refactor the code to only use get profile
func (service *Service) GetProfileDetails(user *model.User) (*model.UserDetailsWithVerifications, error) {
	userSettings := model.UserSettings{}
	userDetails := model.UserDetails{}
	userDetailsVerifications := model.UserDetailsWithVerifications{}

	db := service.repo.ConnReader.First(&userDetails, "user_id = ?", user.ID)
	if db.Error != nil {
		return nil, db.Error
	}

	kyc, KycErr := service.GetKycByID(user.KycID)
	userDetailsVerifications.UserDetails = userDetails
	userDetailsVerifications.Verifications.Type = true
	userDetailsVerifications.Verifications.Kyc = KycErr == nil && kyc.Status == "step_two_success"
	userDetailsVerifications.Verifications.Account = user.Status != "pending"
	userDetailsVerifications.Verifications.Tfa = (userSettings.GoogleAuthKey != "" || userSettings.SmsAuthKey != "")

	return &userDetailsVerifications, nil
}

// GetProfileSettings - get a user's settings
// @deprecated CH: Remove this and refactor the code to only use get profile
func (service *Service) GetProfileSettings(id uint64) (*model.UserSettings, error) {
	userSettings := model.UserSettings{}
	db := service.repo.ConnReader.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		return nil, db.Error
	}

	return &userSettings, nil
}

func (service *Service) GetPushNotificationSettings(userId uint64) (*model.PushNotificationSettings, error) {
	pushNotificationSettings := new(model.PushNotificationSettings)
	db := service.repo.ConnReader.First(&pushNotificationSettings, "user_id = ?", userId)
	if db.Error != nil {
		return nil, db.Error
	}

	return pushNotificationSettings, nil
}

func (service *Service) SetPushNotificationSettings(userId uint64, pushNotificationSettingsRequest model.PushNotificationSettingsRequest) error {
	pushNotificationSettings := model.PushNotificationSettings{}
	db := service.repo.ConnReader.First(&pushNotificationSettings, "user_id = ?", userId)
	if db.Error != nil {
		return db.Error
	}

	pushNotificationSettings.UpdatePushNotificationSettings(pushNotificationSettingsRequest)
	db = service.repo.Conn.Save(&pushNotificationSettings).Where("user_id = ?", userId)
	if db.Error != nil {
		return db.Error
	}

	return nil
}

// GetAntiPhishingCode - get Anti Phishing code from user's settings
func (service *Service) GetAntiPhishingCode(id uint64) (string, error) {
	userSettings := model.UserSettings{}
	db := service.repo.ConnReader.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		return "", db.Error
	}

	if userSettings.AntiPhishingKey == "" {
		userSettings.AntiPhishingKey = "-"
	}

	return userSettings.AntiPhishingKey, nil
}

// IsIPChangeEnabled - check IP change from user's settings
func (service *Service) IsIPChangeEnabled(id uint64) (bool, error) {
	userSettings := model.UserSettings{}
	db := service.repo.ConnReader.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		return true, db.Error
	}
	enabled := userSettings.DetectIPChange == "true"

	return enabled, nil
}

// EnableTradePassword - set trade password for an user
func (service *Service) EnableTradePassword(id uint64, tradePassword string) (*model.UserSettings, error) {
	userSettings := model.UserSettings{}
	db := service.repo.ConnReader.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		return nil, db.Error
	}
	//if password is the same with the current password
	if userSettings.ValidatePass(tradePassword) {
		return nil, errors.New("You have already used that password, try another.")
	}
	// trade password exist and its length is less than 3
	if len(tradePassword) <= 3 && len(tradePassword) != 0 {
		return nil, errors.New("Password has to be at least 4 characters long.")
	}
	// if the trade password is not the old password (who is encoded)
	if userSettings.TradePassword != tradePassword {
		userSettings.TradePassword = tradePassword
		_ = userSettings.EncodePass()
	}
	updateDB := service.repo.Conn.Model(&userSettings).Where("user_id = ?", id).Update("trade_password", userSettings.TradePassword)

	if updateDB.Error != nil {
		return nil, updateDB.Error
	}
	return &userSettings, nil
}

// DisableTradePassword - disable trade password for an user
func (service *Service) DisableTradePassword(id uint64, tradePassword string) (*model.UserSettings, error) {
	userSettings := model.UserSettings{}
	db := service.repo.ConnReader.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		return nil, db.Error
	}
	//if the old password from frontend it's not the same with the current password
	if len(tradePassword) > 0 && !userSettings.ValidatePass(tradePassword) {
		return nil, errors.New("The old password you have entered is incorrect.")
	}
	// set the trade password to a null string
	updateDB := service.repo.Conn.Model(&userSettings).Where("user_id = ?", id).Update("trade_password", "")
	if updateDB.Error != nil {
		return nil, updateDB.Error
	}
	return &userSettings, nil
}

// AdminDisableTradePassword - disable trade password for an user from admin
func (service *Service) AdminDisableTradePassword(id uint64) (*model.UserSettings, error) {
	userSettings := model.UserSettings{}
	// set the trade password to a null string
	updateDB := service.repo.Conn.Model(&userSettings).Where("user_id = ?", id).Update("trade_password", "")
	if updateDB.Error != nil {
		return nil, updateDB.Error
	}
	return &userSettings, nil
}

// EditDetectIP - edit detect IP from user_settings for an user
func (service *Service) EditDetectIP(id uint64, detectIP string) (*model.UserSettings, error) {
	userSettings := model.UserSettings{}
	updateDB := service.repo.Conn.Model(&userSettings).
		Where("user_id = ?", id).Update("detect_ip_change", detectIP).
		Find(&userSettings)
	if updateDB.Error != nil {
		return nil, updateDB.Error
	}
	return &userSettings, nil
}

// EditAntiPhishingCode - edit Anti Phishing Code from user_settings for an user
func (service *Service) EditAntiPhishingCode(id uint64, key string) (*model.UserSettings, error) {
	userSettings := model.UserSettings{}
	updateDB := service.repo.Conn.Model(&userSettings).Where("user_id = ?", id).Update("anti_phishing_key", key)

	if updateDB.Error != nil {
		return nil, updateDB.Error
	}
	return &userSettings, nil
}

// UpdateProfileSettings - update a user's settings
func (service *Service) UpdateProfileSettings(id uint64, fees_payed_with_prdx, detect_ip_change, anti_phishing_key, google_auth_key, sms_auth_key string) (*model.UserSettings, error) {
	userSettings := model.UserSettings{}
	db := service.repo.Conn.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		return nil, db.Error
	}

	userSettings.FeesPayedWithPrdx = fees_payed_with_prdx
	userSettings.DetectIPChange = detect_ip_change
	userSettings.AntiPhishingKey = anti_phishing_key
	userSettings.GoogleAuthKey = google_auth_key
	userSettings.SmsAuthKey = sms_auth_key

	err := service.repo.Update(userSettings)

	if err != nil {
		return nil, err
	}
	return &userSettings, nil
}

// UpdateUserSettings - update a user settings record
func (service *Service) UpdateUserSettings(tx *gorm.DB, userSettings *model.UserSettings, feesPayedWithPrdx, detectIPChange, antiPhishingKey, googleAuthKey, smsAuthKey, tradePassword string, userLevel int) (*model.UserSettings, error) {
	userSettings.FeesPayedWithPrdx = feesPayedWithPrdx
	userSettings.DetectIPChange = detectIPChange
	userSettings.AntiPhishingKey = antiPhishingKey
	userSettings.GoogleAuthKey = googleAuthKey
	userSettings.SmsAuthKey = smsAuthKey
	userSettings.TradePassword = tradePassword
	userSettings.UserLevel = userLevel
	userSettings.UpdatedAt = time.Now()
	db := tx.Table("user_settings").Where("id = ?", userSettings.ID).Save(userSettings)
	if db.Error != nil {
		return nil, db.Error
	}
	return userSettings, nil
}

// UpdateProfileDetails - update a user's details
func (service *Service) UpdateProfileDetails(userID uint64, details UserDetails) (*model.UserDetails, error) {
	user := model.User{}
	db := service.repo.ConnReader.First(&user, "id = ?", userID)
	if db.Error != nil {
		return nil, db.Error
	}

	userDetails := model.UserDetails{}
	db = service.repo.ConnReader.First(&userDetails, "user_id = ?", userID)
	if db.Error != nil {
		return nil, db.Error
	}

	user.FirstName = details.FirstName
	user.LastName = details.LastName

	userDetails.DOB = details.DOB
	userDetails.Country = details.Country
	userDetails.Phone = details.Phone
	userDetails.Address = details.Address
	userDetails.City = details.City
	userDetails.State = details.State
	userDetails.PostalCode = details.PostalCode
	userDetails.Gender, _ = model.GetGenderTypeFromString(string(userDetails.Gender))
	// userDetails.Language = model.LanguageCode(details.Language)

	tx := service.repo.Conn.Begin()
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

	// commit the transaction and return the new data
	return &userDetails, tx.Commit().Error
}

// GetProfileLoginLogs - get a user's Login logs
func (service *Service) GetProfileLoginLogs(id uint64, query, pageValue, limitValue string) (*model.UserActivityLogsList, error) {
	limit, _ := strconv.ParseInt(limitValue, 10, 64)
	page, _ := strconv.ParseInt(pageValue, 10, 64)
	userActivity := make([]model.UserActivity, 0)

	var rowCount int64 = 0
	q := service.repo.ConnReader.Where("user_id=? AND event='login'", id)
	if len(query) > 0 {
		query = "%" + query + "%"
		q = q.Table("user_activities").Where("ip LIKE ?", query)
	}

	dbc := q.Table("user_activities").Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)
	db := q.Table("user_activities").Select("*").Group("id")
	db = db.Limit(int(limit)).Offset(int((page - 1) * limit)).Group("created_at").Order("created_at DESC").Find(&userActivity)

	if db.Error != nil {
		return nil, db.Error
	}
	logs := model.UserActivityLogsList{
		Logs: userActivity,
		Meta: model.PagingMeta{
			Page:  int(page),
			Count: rowCount,
			Limit: int(limit)},
	}

	return &logs, nil
}

// SetPassword - used when changing password of user
// @todo CH: Refactor this code to only update the password and not make another select
func (service *Service) SetPassword(id uint64, currentpass, newpass string) error {
	user := model.User{}

	db := service.repo.Conn.Where("id = ?", id).First(&user)
	if db.Error != nil {
		// invalid user
		return db.Error
	}

	if !user.ValidatePass(currentpass) {
		// invalid password
		return errors.New("Invalid password")
	}

	user.Password = newpass
	_ = user.EncodePass()
	err := service.repo.Update(user)
	if err != nil {
		return err
	}

	// password was updated
	return nil
}

func (service *Service) UpdateUserSettingsSelectedLayout(id uint64, selectedLayout string) error {
	err := service.repo.UpdateUserSettingsSelectedLayout(id, selectedLayout)
	if err != nil {
		log.Error().Err(err).
			Uint64("user_id", id).
			Str("selected_layout", selectedLayout).
			Msg("unable to update selected_layout in user_settings")
		return err
	}

	return nil
}

func (service *Service) UpdateUserNickname(user *model.User) error {
	logger := log.With().
		Str("section", "service").
		Str("method", "UpdateUserAvatarAndNickname").
		Logger()

	existUser, err := service.repo.GetUserByNickname(user.Nickname)
	if err != nil {
		logger.Error().Err(err).Msg("Can not get user by nickname")
		return err
	}

	if existUser.ID != 0 && existUser.ID != user.ID {
		logger.Error().Msg("User with this nickname already exists")
		return errors.New("user with this nickname already exists")
	}

	err = service.repo.UpdateUser(user)
	if err != nil {
		logger.Error().Msg("Can not update user")
		return err
	}

	return nil
}

func (service *Service) UpdateUserAvatar(user *model.User) error {
	logger := log.With().
		Str("section", "service").
		Str("method", "UpdateUserAvatarAndNickname").
		Logger()

	err := service.repo.UpdateUser(user)
	if err != nil {
		logger.Error().Msg("Can not update user")
		return err
	}

	return nil
}
