package wallet

import (
	"github.com/jackc/pgconn"
	"github.com/rs/zerolog/log"
	data "gitlab.com/paramountdax-exchange/exchange_api_v2/data/wallet"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// CreateAddress handler
func (app *App) CreateAddress(event *data.Event) error {
	addrType, ok := event.Meta["type"]
	if !ok {
		addrType = string(model.AddressType_User)
	}
	depositCode, ok := event.Payload["deposit_code"]
	if !ok {
		depositCode = ""
	}
	address := model.NewAddress(
		event.ID,
		event.UserID,
		event.Chain,
		model.AddressType(addrType),
		model.AddressStatus_Active,
		"exchange",
		event.Payload["address"],
		depositCode,
	)
	// try to create the record
	err := app.repo.Create(&address)
	if err == nil {
		// if successful
		log.Info().
			Str("section", "app:wallet").
			Str("action", "create_address").
			Str("event_id", event.ID).
			Str("event_chain", event.Chain).
			Uint64("user_id", event.UserID).
			Str("address", event.Payload["address"]).
			Str("deposit_code", depositCode).
			Msg("New deposit address saved")
		return nil
	}

	// if we receive a Postgres Error with a unique constraint error code just skip over it
	if pgerr, ok := err.(*pgconn.PgError); pgerr != nil && ok {
		if pgerr.Code == "23505" && pgerr.ConstraintName == "addresses_pkey" {
			log.Error().Err(pgerr).
				Str("section", "app:wallet").
				Str("action", "create_address").
				Str("event_id", event.ID).
				Str("event_chain", event.Chain).
				Uint64("user_id", event.UserID).
				Str("address", event.Payload["address"]).
				Msg("Address already exists")
			return nil
		}
		return nil
	}
	// otherwise log the error and skip the message
	log.Error().Err(err).
		Str("section", "app:wallet").
		Str("action", "create_address").
		Str("event_id", event.ID).
		Str("event_chain", event.Chain).
		Uint64("user_id", event.UserID).
		Str("address", event.Payload["address"]).
		Msg("Unable to save new address")
	return err
}
