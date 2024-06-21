package service

import (
	"github.com/Unleash/unleash-client-go/v3"
	"github.com/ericlagergren/decimal"
	"github.com/jackc/pgconn"
	"github.com/rs/zerolog/log"
	kafkaGo "github.com/segmentio/kafka-go"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/coins"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data/wallet"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gorm.io/gorm"
	"strconv"
	"time"
)

func (service *Service) AddLeadBonusForKYC(tx *gorm.DB, kyc *model.KYC, user *model.User, userDetails *model.UserDetails) error {
	if !unleash.IsEnabled("api.lead-bonus.enable") {
		return nil
	}

	if kyc.Status != model.KYCStatusStepTwoSuccess && kyc.Status != model.KYCStatusStepThreeFail &&
		kyc.Status != model.KYCStatusStepThreePending && kyc.Status != model.KYCStatusStepThreeSuccess {
		return nil
	}

	savePointName := "AddLeadBonusForKYC"
	if err := tx.SavePoint(savePointName).Error; err != nil {
		return err
	}

	if lbc, err := service.repo.GetLeadBonusCampaignByMarketingID(userDetails.LeadFromResource); err == nil {
		if !lbc.AllowRewardAfterExpiration && !lbc.IsActive() {
			return nil
		}

		op := model.NewOperation(model.OperationType_Deposit, model.OperationStatus_Accepted)
		if err := tx.Create(op).Error; err != nil {
			return err
		}

		leadBonus := &model.LeadBonus{
			UserID:      user.ID,
			MarketingID: userDetails.LeadFromResource,
			Amount:      lbc.Amount,
			CoinSymbol:  lbc.CoinSymbol,
			RefID:       op.RefID,
			SubAccount:  0,
			Comment:     lbc.Comment,
			RewardsTime: time.Now().AddDate(0, 0, lbc.RewardsDelay),
			Status:      model.LeadBonusStatus_Pending,
		}

		var txStatus = model.TxStatus_Unconfirmed
		if lbc.RewardsType == model.LeadBonusCampaignRewardsType_Instant {
			leadBonus.Status = model.LeadBonusStatus_Payed
			txStatus = model.TxStatus_Confirmed
		}

		if err := tx.Table("lead_bonuses").Create(leadBonus).Error; err != nil {
			if pgerr, ok := err.(*pgconn.PgError); pgerr != nil && ok {
				if pgerr.Code == "23505" && pgerr.ConstraintName == "lead_bonus_idx" {
					tx.RollbackTo(savePointName)
					return nil
				}
			}
			return err
		}

		coin, err := coins.Get(lbc.CoinSymbol)
		if err != nil {
			return err
		}

		amountInUnits := conv.ToUnits(leadBonus.Amount.V.String(), uint8(coin.TokenPrecision))
		amountAsDecimal := new(decimal.Big)
		amountAsDecimal.SetUint64(amountInUnits)
		feeAmountInUnits := conv.ToUnits("0.0", uint8(coin.TokenPrecision))
		feeAmountAsDecimal := new(decimal.Big)
		feeAmountAsDecimal.SetUint64(feeAmountInUnits)

		event := wallet.Event{
			Event:  string(wallet.EventType_Deposit),
			UserID: user.ID,
			ID:     op.RefID,
			Coin:   coin.Symbol,
			Meta:   map[string]string{},
			Payload: map[string]string{
				"confirmations":   strconv.Itoa(1),
				"amount":          amountAsDecimal.String(),
				"fee":             feeAmountAsDecimal.String(),
				"address":         "",
				"status":          txStatus.String(),
				"txid":            "",
				"external_system": "bonus_deposit",
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
	} else {
		log.Error().Err(err).
			Str("service", "kyc").
			Str("method", "AddLeadBonusForKYC").
			Uint64("userId", user.ID).
			Msg("Unable to find the campaign")
	}

	return nil
}

func (service *Service) FillMissedLeadBonuses() {

	logger := log.With().
		Str("service", "kyc").
		Str("method", "FillMissedLeadBonuses").
		Logger()

	ids := []uint{}

	err := service.repo.ConnReaderAdmin.
		Table("user_details ud").
		Joins("left join users u on u.id = ud.user_id").
		Joins("left join kycs k on u.kyc_id = k.id").
		Joins("left join lead_bonuses lb on u.id = lb.user_id").
		Joins("left join lead_bonus_campaigns lbc on lbc.marketing_id = ud.lead_from_resource").
		Where("ud.lead_from_resource is not null").
		Where("ud.lead_from_resource != ''").
		Where("lb is null").
		Where("k.status in ('step_two_success', 'step_three_fail', 'step_three_pending', 'step_three_success')").
		Select("u.id").Find(&ids).Error

	if err != nil {
		logger.Error().Err(err).Msg("Unable to find user ids")
		return
	}

	if len(ids) > 0 {
		for _, userID := range ids {
			tx := service.repo.Conn.Begin()

			loggerIn := logger.With().Uint("userID", userID).Logger()
			user, err := service.GetUserByID(userID)
			if err != nil {
				loggerIn.Error().Err(err).Msg("Unable to find user")
				tx.Rollback()
				continue
			}

			kyc, err := service.GetKycByID(user.KycID)
			if err != nil {
				loggerIn.Error().Err(err).Msg("Unable to find kyc")
				tx.Rollback()
				continue
			}

			userDetails := model.UserDetails{}
			if err := service.repo.ConnReader.First(&userDetails, "user_id = ?", userID).Error; err != nil {
				loggerIn.Error().Err(err).Msg("Unable to user details")
				tx.Rollback()
				continue
			}

			if err := service.AddLeadBonusForKYC(tx, kyc, user, &userDetails); err != nil {
				loggerIn.Error().Err(err).Msg("Unable to add lead bonus")
				tx.Rollback()
				continue
			} else {
				if err := tx.Commit().Error; err != nil {
					logger.Error().Err(err).Msg("Unable to commit tx")
				} else {
					loggerIn.Info().Msg("Lead bonus added")
				}
			}
		}
	}
}
