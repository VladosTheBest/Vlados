package service

import (
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/auth_service"
	"mime/multipart"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type SupportRequestEmailType string

const (
	SupportRequestEmailTypeOpportunity     SupportRequestEmailType = "opportunity"
	SupportRequestEmailTypeBugRequest      SupportRequestEmailType = "bug-report"
	SupportRequestEmailTypeCommentsRequest SupportRequestEmailType = "comments"
	SupportRequestEmailTypeIdeaRequest     SupportRequestEmailType = "idea"
)

func (s SupportRequestEmailType) String() string {
	switch s {
	case SupportRequestEmailTypeOpportunity:
		return "Opportunities"
	case SupportRequestEmailTypeBugRequest:
		return "Bug Report"
	case SupportRequestEmailTypeCommentsRequest:
		return "Comments"
	case SupportRequestEmailTypeIdeaRequest:
		return "Idea"
	default:
		return ""
	}
}

func (s SupportRequestEmailType) IsValid() bool {
	switch s {
	case SupportRequestEmailTypeOpportunity,
		SupportRequestEmailTypeBugRequest,
		SupportRequestEmailTypeCommentsRequest,
		SupportRequestEmailTypeIdeaRequest:
		return true
	}

	return false
}

func (s *Service) getLocalFromLocation(location string) *time.Location {
	local := time.UTC
	if location != "" {
		localFromTimezone, err := time.LoadLocation(location)
		if err == nil {
			local = localFromTimezone
		}
	}

	return local
}

// LoginNoticeEmail send an email every time the user logs in with details about geolocation
func (s *Service) LoginNoticeEmail(email, language, name, apCode string, geoLocation GeoLocation, location string) error {
	browser := geoLocation.Agent["Name"].(string) + " (" + geoLocation.Agent["OSVersion"].(string) + " )"
	local := s.getLocalFromLocation(location)

	return s.sendgrid.SendEmail(
		email,
		language,
		"login_notice",
		map[string]string{
			"name":    name,
			"email":   email,
			"domain":  geoLocation.Domain,
			"ip":      geoLocation.IP,
			"country": geoLocation.Country,
			"city":    geoLocation.City,
			"date":    time.Now().In(local).Format(time.RFC822),
			"os":      geoLocation.Agent["OS"].(string),
			"browser": browser,
			"apcode":  apCode,
		},
	)
}

// KycNoticePending send an email with pending status for KYC
func (s *Service) KycNoticePending(email, language, apCode, location string) error {
	local := s.getLocalFromLocation(location)

	return s.sendgrid.SendEmail(
		email,
		language,
		"kyc_notice_pending_result",
		map[string]string{
			"apcode": apCode,
			"date":   time.Now().In(local).Format(time.RFC822),
		},
	)
}

// KycNoticeStatus send an email with pending status for KYC
func (s *Service) KycNoticeStatus(email, language, decision, apCode, location string) error {
	local := s.getLocalFromLocation(location)

	return s.sendgrid.SendEmail(
		email,
		language,
		"kyc_status_notice",
		map[string]string{
			"apcode": apCode,
			"status": decision,
			"date":   time.Now().In(local).Format(time.RFC822),
		},
	)
}

// SendEmailConfirmation - send an email in order to verify the user's email address
func (s *Service) SendEmailConfirmation(email, language, token, location string) error {
	local := s.getLocalFromLocation(location)

	return s.sendgrid.SendEmail(
		email,
		language,
		"confirm_email",
		map[string]string{
			"domain": s.apiConfig.Domain,
			"token":  token,
			"date":   time.Now().In(local).Format(time.RFC822),
		},
	)
}

// GenerateAndSendEmailConfirmation - generate verificatoon token and send it to the given email
func (s *Service) GenerateAndSendEmailConfirmation(email, language, location string) (string, error) {
	token, err := auth_service.CreateToken(jwt.MapClaims{
		"email": email,
	}, s.apiConfig.JWTTokenSecret, 24)
	if err != nil {
		return token, err
	}
	return token, s.SendEmailConfirmation(email, language, token, location)
}

// SendForgotPasswordEmail - send the forgot password email and start the change password flow
func (s *Service) SendForgotPasswordEmail(email, language, apCode, token, source string, geoLocation GeoLocation, location string) error {
	domain := s.apiConfig.Domain
	if source == "admin" {
		domain = s.adminConfig.Domain + "/auth"
	}
	browser := geoLocation.Agent["Name"].(string) + " (" + geoLocation.Agent["OSVersion"].(string) + " )"
	local := s.getLocalFromLocation(location)

	return s.sendgrid.SendEmail(
		email,
		language,
		"forgot_password",
		map[string]string{
			"domain":  domain,
			"ip":      geoLocation.IP,
			"country": geoLocation.Country,
			"city":    geoLocation.City,
			"date":    time.Now().In(local).Format(time.RFC822),
			"os":      geoLocation.Agent["OS"].(string),
			"browser": browser,
			"email":   email,
			"token":   token,
			"apcode":  apCode,
		},
	)
}

// SendForgotPasswordEmail - send the forgot password email and start the change password flow
func (s *Service) SendSupportRequestEmail(emailRequestType SupportRequestEmailType, email, name, message string, attachments []*multipart.FileHeader, location string) error {
	local := s.getLocalFromLocation(location)
	return s.sendgrid.SendEmailWithAttachments(
		s.cfg.Server.Info.Contact.Email,
		"en",
		"admin_notice_support_request",
		map[string]string{
			"domain":       s.apiConfig.Domain,
			"date":         time.Now().In(local).Format(time.RFC822),
			"email":        s.cfg.Server.Info.Contact.Email,
			"email_type":   emailRequestType.String(),
			"user_name":    name,
			"user_email":   email,
			"user_message": message,
		},
		attachments,
	)
}

func (s *Service) SendPaymentDocumentVerificationEmail(email string, name string, attachments []*multipart.FileHeader) error {
	return s.sendgrid.SendEmailWithAttachments(
		s.cfg.Server.Verification.Email,
		"en",
		"kyc_verification_step_3",
		map[string]string{
			"user_name":  name,
			"user_email": email,
		},
		attachments,
	)
}

// SendTwoFactorRecoveryCode - send two factor recovery code to the email
func (s *Service) SendTwoFactorRecoveryCode(email, language, name, headline, code string) error {
	return s.sendgrid.SendEmail(
		email,
		language,
		"recover_2fa",
		map[string]string{
			"user_name": name,
			"headline":  headline,
			"code":      code,
		},
	)
}

func (s *Service) SendEmailForWithdrawRequest(userEmail, userAmount, userCoin, location string) error {
	local := s.getLocalFromLocation(location)
	for _, email := range s.cfg.Emails {
		err := s.sendgrid.SendEmail(
			email,
			"en",
			"funds_were_withdrawn",
			map[string]string{
				"email":  userEmail,
				"amount": userAmount,
				"coin":   userCoin,
				"date":   time.Now().In(local).Format(time.RFC822),
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SendEmailForManualDepositRequest(sender, recipient, userCoin, amount, confirmUrl string, timeCreated time.Time) error {
	for _, email := range s.cfg.Server.ManualTransactions.ConfirmingUsers {
		err := s.sendgrid.SendEmail(
			email,
			"en",
			"manual_deposit_notification",
			map[string]string{
				"sender":     sender,
				"recipient":  recipient,
				"coin":       userCoin,
				"amount":     amount,
				"time":       timeCreated.Format(time.RFC822),
				"confirmUrl": confirmUrl,
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SendEmailForDepositConfirmed(transaction *model.Transaction, language string) error {

	user, err := s.GetUserByID(uint(transaction.UserID))
	if err != nil {
		return err
	}
	account, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, user.ID, model.AccountGroupMain)
	if err != nil {
		return err
	}
	userBalance, err := s.GetLiabilityBalances(user.ID, account)
	if err != nil {
		return err
	}
	transactionCoinBalance := userBalance[transaction.CoinSymbol]

	err = s.sendgrid.SendEmail(
		user.Email,
		language,
		"email_deposit_confirmed",
		map[string]string{
			"userName":              user.FullName(),
			"depositAddress":        transaction.Address,
			"timeOfConfirmation":    transaction.UpdatedAt.Format(time.RFC822),
			"depositAmount":         transaction.Amount.V.String(),
			"accountBalance":        transactionCoinBalance.GetTotal().String(),
			"transactionCoinSymbol": transaction.CoinSymbol,
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) SendEmailForBotNotify(bot *model.Bot, botContract *model.BonusAccountContractViewWithBonusContract, message, language string) error {
	user, err := s.GetUserByID(uint(bot.UserId))
	if err != nil {
		return err
	}

	botContractID := ""
	botContractAmount := ""
	if botContract != nil {
		botContractID = strconv.FormatUint(botContract.ContractID, 10)
		botContractAmount = botContract.Amount.V.String()
	}

	if user.EmailStatus.IsAllowed() {
		err = s.sendgrid.SendEmail(
			user.Email,
			language,
			"email_bot_notify",
			map[string]string{
				"userName":       user.FullName(),
				"botID":          strconv.FormatUint(bot.ID, 10),
				"contractID":     botContractID,
				"contractAmount": botContractAmount,
				"message":        message,
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SendEmailForManualWithdrawalRequest(sender, recipient, userCoin, amount, confirmUrl string, timeCreated time.Time) error {
	for _, email := range s.cfg.Server.ManualTransactions.ConfirmingUsers {
		err := s.sendgrid.SendEmail(
			email,
			"en",
			"manual_withdrawal_notification",
			map[string]string{
				"sender":     sender,
				"recipient":  recipient,
				"coin":       userCoin,
				"amount":     amount,
				"time":       timeCreated.Format(time.RFC822),
				"confirmUrl": confirmUrl,
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SendEmailForWithdrawConfirmed(transaction *model.Transaction, language string) error {

	user, err := s.GetUserByID(uint(transaction.UserID))
	if err != nil {
		return err
	}
	account, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, user.ID, model.AccountGroupMain)
	if err != nil {
		return err
	}
	userBalance, err := s.GetLiabilityBalances(user.ID, account)
	if err != nil {
		return err
	}
	transactionCoinBalance := userBalance[transaction.CoinSymbol]

	err = s.sendgrid.SendEmail(
		user.Email,
		language,
		"email_withdraw_confirmed",
		map[string]string{
			"userName":              user.FullName(),
			"depositAddress":        transaction.Address,
			"timeOfConfirmation":    transaction.UpdatedAt.Format(time.RFC822),
			"depositAmount":         transaction.Amount.V.String(),
			"accountBalance":        transactionCoinBalance.GetTotal().String(),
			"transactionCoinSymbol": transaction.CoinSymbol,
		},
	)
	if err != nil {
		return err
	}
	return nil
}
