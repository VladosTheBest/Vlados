package model

import (
	postgresDialects "github.com/jinzhu/gorm/dialects/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"strconv"
	"strings"

	"github.com/ericlagergren/decimal/sql/postgres"
)

const DefaultPointLimit string = "-1"

// UserPaymentDetails structure
type UserPaymentDetails struct {
	ID                         uint64                 `sql:"type:bigint" gorm:"primary_key" json:"id"`
	UserID                     uint64                 `sql:"type:bigint REFERENCES users(id)" json:"user_id"`
	ReferenceCode              string                 `sql:"type:varchar(10)" json:"reference_code"`
	BlockDepositCrypto         bool                   `sql:"type:bool" json:"block_deposit_crypto"`
	BlockDepositFiat           bool                   `sql:"type:bool" json:"block_deposit_fiat"`
	BlockWithdrawCrypto        bool                   `sql:"type:bool" json:"block_withdraw_crypto"`
	BlockWithdrawFiat          bool                   `sql:"type:bool" json:"block_withdraw_fiat"`
	IsDepositAddress           postgresDialects.Jsonb `sql:"type:decimal(36,18)" json:"is_deposit_address"`
	WithdrawLimit              *postgres.Decimal      `sql:"type:decimal(36,18)" json:"withdraw_limit"`
	AdvCashWithdrawLimit       *postgres.Decimal      `sql:"type:decimal(36,18)" json:"adv_cash_withdraw_limit"`
	ClearJunctionWithdrawLimit *postgres.Decimal      `sql:"type:decimal(36,18)" json:"clear_junction_withdraw_limit"`
	DefaultWithdrawLimit       *postgres.Decimal      `sql:"type:decimal(36,18)" json:"default_withdraw_limit"`
}

func NewUserPaymentDetails(userID uint64) UserPaymentDetails {
	refCode := strconv.FormatUint(userID, 10)
	refCode = utils.NewHashSum(refCode)
	refCode = strings.ToUpper(refCode[:8])
	defaultPoint, _ := conv.NewDecimalWithPrecision().SetString(DefaultPointLimit)

	return UserPaymentDetails{
		UserID:                     userID,
		ReferenceCode:              refCode,
		WithdrawLimit:              &postgres.Decimal{V: defaultPoint},
		AdvCashWithdrawLimit:       &postgres.Decimal{V: defaultPoint},
		ClearJunctionWithdrawLimit: &postgres.Decimal{V: defaultPoint},
		DefaultWithdrawLimit:       &postgres.Decimal{V: defaultPoint},
	}
}
