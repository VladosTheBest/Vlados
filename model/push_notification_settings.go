package model

type PushNotificationSettings struct {
	Id                      uint64 `sql:"type: bigint" gorm:"primary_key" json:"-"`
	UserId                  uint64 `sql:"type: bigint" gorm:"primary_key" json:"-"`
	DepositCrypto           bool
	DepositFiat             bool
	WithdrawCrypto          bool
	WithdrawFiat            bool
	TwoFactorAuthentication bool
	PasswordChanged         bool
	DetectIpChange          bool
	EntryFromAnotherIp      bool
	TradePassword           bool
	AntiPhishingCode        bool
	NewApiKey               bool
	KycVerification         bool
	Announcement            bool
	BotNotify               bool
}

type PushNotificationSettingsRequest struct {
	DepositCrypto           bool `json:"deposit_crypto"`
	DepositFiat             bool `json:"deposit_fiat"`
	WithdrawCrypto          bool `json:"withdraw_crypto"`
	WithdrawFiat            bool `json:"withdraw_fiat"`
	TwoFactorAuthentication bool `json:"two_factor_authentication"`
	PasswordChanged         bool `json:"password_changed"`
	DetectIpChange          bool `json:"detect_ip_change"`
	EntryFromAnotherIp      bool `json:"entry_from_another_ip"`
	TradePassword           bool `json:"trade_password"`
	AntiPhishingCode        bool `json:"anti_phishing_code"`
	NewApiKey               bool `json:"new_api_key"`
	KycVerification         bool `json:"kyc_verification"`
	Announcement            bool `json:"announcement"`
	BotNotify               bool `json:"bot_notify"`
}

func NewPushNotificationSettings(userId uint64) *PushNotificationSettings {
	return &PushNotificationSettings{
		UserId: userId,
	}
}

func (pushNotificationSettings *PushNotificationSettings) UpdatePushNotificationSettings(pushNotificationSettingsRequest PushNotificationSettingsRequest) {
	pushNotificationSettings.DepositCrypto = pushNotificationSettingsRequest.DepositCrypto
	pushNotificationSettings.DepositFiat = pushNotificationSettingsRequest.DepositFiat
	pushNotificationSettings.WithdrawCrypto = pushNotificationSettingsRequest.WithdrawCrypto
	pushNotificationSettings.WithdrawFiat = pushNotificationSettingsRequest.WithdrawFiat
	pushNotificationSettings.TwoFactorAuthentication = pushNotificationSettingsRequest.TwoFactorAuthentication
	pushNotificationSettings.PasswordChanged = pushNotificationSettingsRequest.PasswordChanged
	pushNotificationSettings.DetectIpChange = pushNotificationSettingsRequest.DetectIpChange
	pushNotificationSettings.EntryFromAnotherIp = pushNotificationSettingsRequest.EntryFromAnotherIp
	pushNotificationSettings.TradePassword = pushNotificationSettingsRequest.TradePassword
	pushNotificationSettings.AntiPhishingCode = pushNotificationSettingsRequest.AntiPhishingCode
	pushNotificationSettings.NewApiKey = pushNotificationSettingsRequest.NewApiKey
	pushNotificationSettings.KycVerification = pushNotificationSettingsRequest.KycVerification
	pushNotificationSettings.Announcement = pushNotificationSettingsRequest.Announcement
	pushNotificationSettings.BotNotify = pushNotificationSettingsRequest.BotNotify
}

func (pushNotificationSettings PushNotificationSettings) IsEnabled(objectType RelatedObjectType) bool {
	switch objectType {
	case Notification_Deposit_Crypto:
		return pushNotificationSettings.DepositCrypto
	case Notification_Deposit_Fiat:
		return pushNotificationSettings.DepositFiat
	case Notification_Withdraw_Crypto:
		return pushNotificationSettings.WithdrawCrypto
	case Notification_Withdraw_Fiat:
		return pushNotificationSettings.WithdrawFiat
	case Notification_2factorAuthentication:
		return pushNotificationSettings.TwoFactorAuthentication
	case Notification_PasswordChanged:
		return pushNotificationSettings.PasswordChanged
	case Notification_DetectIpChange:
		return pushNotificationSettings.DetectIpChange
	case Notification_EntryFromAnotherIP:
		return pushNotificationSettings.EntryFromAnotherIp
	case Notification_TradePassword:
		return pushNotificationSettings.TradePassword
	case Notification_AntiPhishingCode:
		return pushNotificationSettings.AntiPhishingCode
	case Notification_NewAPIKey:
		return pushNotificationSettings.NewApiKey
	case Notification_KYCVerification:
		return pushNotificationSettings.KycVerification
	case Notification_Announcement:
		return pushNotificationSettings.KycVerification
	case Notification_Bot_Notify:
		return pushNotificationSettings.KycVerification
	default:
		return false
	}
}
