package model

import (
	"encoding/json"
	"strings"
	"time"
)

type NotificationStatus string

const (
	NotificationStatus_Read    NotificationStatus = "read"
	NotificationStatus_UnRead  NotificationStatus = "unread"
	NotificationStatus_Deleted NotificationStatus = "deleted"
)

func (n NotificationStatus) IsValid() bool {
	switch n {
	case NotificationStatus_Read,
		NotificationStatus_UnRead,
		NotificationStatus_Deleted:
		return true
	default:
		return false
	}
}

type RelatedObjectType string

const (
	Notification_Deposit_Crypto        RelatedObjectType = "deposit_crypto"
	Notification_Deposit_Fiat          RelatedObjectType = "deposit_fiat"
	Notification_Withdraw_Crypto       RelatedObjectType = "withdraw_crypto"
	Notification_Withdraw_Fiat         RelatedObjectType = "withdraw_fiat"
	Notification_2factorAuthentication RelatedObjectType = "2factor_authentication"
	Notification_PasswordChanged       RelatedObjectType = "password_changed"
	Notification_DetectIpChange        RelatedObjectType = "detect_ip_change"
	Notification_EntryFromAnotherIP    RelatedObjectType = "entry_from_anotherIP"
	Notification_TradePassword         RelatedObjectType = "trade_password"
	Notification_AntiPhishingCode      RelatedObjectType = "anti_phishing_code"
	Notification_NewAPIKey             RelatedObjectType = "new_API_key"
	Notification_KYCVerification       RelatedObjectType = "kyc_verification"
	Notification_Announcement          RelatedObjectType = "announcement"
	Notification_Bot_Notify            RelatedObjectType = "bot_notify"
	//  Notification_BonusContract         RelatedObjectType = "bonus_contract"
	//  Notification_DistributionBonus     RelatedObjectType = "distribution_bonus"
	//  Notification_AddedNewCoin          RelatedObjectType = "added_new_coin"
	//	Notification_PRDXLine              RelatedObjectType = "prdx_line"
	//	Notification_NewRefferal           RelatedObjectType = "new_referral"
)

type NotificationTitle string

const (
	NotificationTitle_PasswordChanged    NotificationTitle = "Change Password"
	NotificationTitle_TradePassword      NotificationTitle = "Trade Password"
	NotificationTitle_2fa                NotificationTitle = "Two Factor Authentication"
	NotificationTitle_AntiPhishingCode   NotificationTitle = "Anti-Phishing Code"
	NotificationTitle_DetectIpChange     NotificationTitle = "Detect IP address change"
	NotificationTitle_EntryFromAnotherIP NotificationTitle = "Entry from another IP address into your account"
	NotificationTitle_NewAPIKey          NotificationTitle = "New API key"
	NotificationTitle_Deposit            NotificationTitle = "Deposit Coin"
	NotificationTitle_Withdraw           NotificationTitle = "Withdraw Coin"
	NotificationTitle_KYCVerification    NotificationTitle = "KYC Verification"
	NotificationTitle_BotNotify          NotificationTitle = "Bot Notify"
)

func (n NotificationTitle) String() string {
	return string(n)
}

type NotificationMessage string

const (
	NotificationMessage_PasswordChanged     NotificationMessage = "Your password has been changed"
	NotificationMessage_TradePasswordON     NotificationMessage = "You have activated your trading password"
	NotificationMessage_TradePasswordOFF    NotificationMessage = "You have deactivated your trading password"
	NotificationMessage_2faGoogleON         NotificationMessage = "You have activated google two-factor authentication"
	NotificationMessage_2faGoogleOFF        NotificationMessage = "You have deactivated google two-factor authentication"
	NotificationMessage_2faSmsON            NotificationMessage = "You have activated SMS two-factor authentication"
	NotificationMessage_2faSmsOFF           NotificationMessage = "You have deactivated SMS two-factor authentication"
	NotificationMessage_AntiPhishingCodeON  NotificationMessage = "You have activated Anti-Phishing code"
	NotificationMessage_AntiPhishingCodeOFF NotificationMessage = "You have deactivated Anti-Phishing code"
	NotificationMessage_DetectIpChangeON    NotificationMessage = "You have activated Detect IP address Change"
	NotificationMessage_DetectIpChangeOFF   NotificationMessage = "You have deactivated Detect IP address Change"
	NotificationMessage_EntryFromAnotherIP  NotificationMessage = "Your account was logged in from a different IP address"
	NotificationMessage_NewAPIKeyON         NotificationMessage = "You generated new API key: '⇶%s⇶'"
	NotificationMessage_NewAPIKeyOFF        NotificationMessage = "You deleted your API key: '⇶%s⇶'"
	NotificationMessage_Deposit             NotificationMessage = "You have increased your balance account on ⇶%s⇶"
	NotificationMessage_Withdraw            NotificationMessage = "You have withdrawn from the account ⇶%s⇶"
	NotificationMessage_KYCVerification     NotificationMessage = "You got level of your account: '⇶%d⇶'"
	NotificationMessage_BotNotify           NotificationMessage = "⇶%s⇶"
)

func (n NotificationMessage) String() string {
	return string(n)
}

func (n NotificationMessage) StringPlain() string {
	return strings.ReplaceAll(n.String(), "⇶", "")
}

type NotificationType string

const (
	NotificationType_Info    NotificationType = "info"
	NotificationType_Warning NotificationType = "warning"
	NotificationType_System  NotificationType = "system"
)

type NotificationURL string

const (
	NotificationURL_Deposit_Crypto  NotificationURL = "/deposit/crypto"
	NotificationURL_Deposit_Fiat    NotificationURL = "/deposit/fiat"
	NotificationURL_Withdraw_Crypto NotificationURL = "/withdraw/crypto"
	NotificationURL_Withdraw_Fiat   NotificationURL = "/withdraw/fiat"
	NotificationURL_Security        NotificationURL = "/security"
	NotificationURL_APIKey          NotificationURL = "/security/api"
	NotificationURL_KYCVerification NotificationURL = "/security/kyc"
)

func (n NotificationURL) String() string {
	return string(n)
}

type Notification struct {
	ID                uint64              `form:"id"                  json:"id"`
	UserID            uint64              `form:"user_id"             json:"user_id"             binding:"required"`
	Status            NotificationStatus  `form:"status"              json:"status"              binding:"required"`
	RelatedObjectType RelatedObjectType   `form:"related_object_type" json:"related_object_type" binding:"required"`
	RelatedObjectID   string              `form:"related_object_id"   json:"related_object_id"   binding:"required"`
	Type              NotificationType    `form:"type"                json:"type"                binding:"required"`
	Title             NotificationTitle   `form:"title"               json:"title"               binding:"required"`
	Message           NotificationMessage `form:"message"             json:"message"             binding:"required"`
	CreatedAt         time.Time           `form:"created_at"          json:"created_at"`
	UpdatedAt         time.Time           `form:"updated_at"          json:"updated_at"`
}

type NotificationWithMeta struct {
	Notification []Notification `json:"notification"`
	Meta         PagingMeta     `json:"meta"`
}

type NotificationWithTotalUnread struct {
	Notification
	TotalUnreadNotifications int `json:"total_unread_notifications"`
}

type OperationSystem string

const (
	IOSOperationSystem     OperationSystem = "ios"
	AndroidOperationSystem OperationSystem = "android"
)

func (os OperationSystem) IsValid() bool {
	switch os {
	case IOSOperationSystem,
		AndroidOperationSystem:
		return true
	default:
		return false
	}
}

type PushToken struct {
	ID              uint64          `form:"id"                  json:"id"`
	UserID          uint64          `form:"user_id"             json:"user_id"`
	PushToken       string          `form:"push_token"          json:"push_token"          binding:"required"`
	Name            string          `form:"name"                json:"name"                binding:"required"`
	UniqueID        string          `form:"unique_id"           json:"unique_id"           binding:"required"`
	OperationSystem OperationSystem `form:"operation_system"    json:"operation_system"    binding:"required"`
}

func (n *Notification) MarshalJSON() ([]byte, error) {
	var url NotificationURL
	switch n.RelatedObjectType {
	case Notification_Deposit_Crypto:
		url = NotificationURL_Deposit_Crypto
	case Notification_Deposit_Fiat:
		url = NotificationURL_Deposit_Fiat
	case Notification_Withdraw_Crypto:
		url = NotificationURL_Withdraw_Crypto
	case Notification_Withdraw_Fiat:
		url = NotificationURL_Withdraw_Fiat
	case Notification_NewAPIKey:
		url = NotificationURL_APIKey
	case Notification_KYCVerification:
		url = NotificationURL_KYCVerification
	default:
		url = NotificationURL_Security
	}

	return json.Marshal(map[string]interface{}{
		"id":                  n.ID,
		"user_id":             n.UserID,
		"status":              n.Status,
		"related_object_type": n.RelatedObjectType,
		"related_object_id":   n.RelatedObjectID,
		"type":                n.Type,
		"title":               n.Title,
		"message":             n.Message,
		"created_at":          n.CreatedAt,
		"updated_at":          n.UpdatedAt,
		"url":                 url,
	})
}
