package model

/*
 * Copyright © 2018-2019 Around25 SRL <office@around25.com>
 *
 * Licensed under the Around25 Wallet License Agreement (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.around25.com/licenses/EXCHANGE_LICENSE
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @author		Cosmin Harangus <cosmin@around25.com>
 * @copyright 2018-2019 Around25 SRL <office@around25.com>
 * @license 	EXCHANGE_LICENSE
 */

import (
	"encoding/json"
	"fmt"
	"github.com/lib/pq"
	gouuid "github.com/nu7hatch/gouuid"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

var ZERO = conv.NewDecimalWithPrecision()

type TxStatus string

const (
	TxStatus_Pending     TxStatus = "pending"
	TxStatus_Unconfirmed TxStatus = "unconfirmed"
	TxStatus_Confirmed   TxStatus = "confirmed"
	TxStatus_Failed      TxStatus = "failed"
)

func (txStatus TxStatus) String() string {
	return string(txStatus)
}

func (txStatus TxStatus) IsValid() bool {
	switch txStatus {
	case TxStatus_Pending,
		TxStatus_Unconfirmed,
		TxStatus_Confirmed,
		TxStatus_Failed:
		return true
	default:
		return false
	}
}

type TxType string

const (
	TxType_Deposit  TxType = "deposit"
	TxType_Withdraw TxType = "withdraw"
)

type TransactionExternalSystem string

const (
	TransactionExternalSystem_Advcash       TransactionExternalSystem = "advcash"
	TransactionExternalSystem_Bitgo         TransactionExternalSystem = "bitgo"
	TransactionExternalSystem_ClearJunction TransactionExternalSystem = "clear_junction"
	TransactionExternalSystem_BonusDeposit  TransactionExternalSystem = "bonus_deposit"
	TransactionExternalSystem_Empty         TransactionExternalSystem = ""
)

// Transaction structure
type Transaction struct {
	ID             string                    `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	UserID         uint64                    `gorm:"column:user_id" json:"user_id"`
	TxID           string                    `gorm:"column:txid" json:"txid"`
	Address        string                    `gorm:"column:address" json:"address"`
	Amount         *postgres.Decimal         `sql:"type:decimal(32,16)"`
	FeeAmount      *postgres.Decimal         `sql:"type:decimal(32,16)"`
	FeeCoin        string                    `gorm:"column:fee_coin" json:"fee_coin"`
	TxType         TxType                    `sql:"not null;type:tx_type_t;default:'deposit'" json:"tx_type"`
	Status         TxStatus                  `sql:"not null;type:tx_status_t;default:'pending'" json:"status"`
	CoinSymbol     string                    `gorm:"column:coin_symbol" json:"coin_symbol"`
	Confirmations  int                       `json:"confirmations"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
	ExternalSystem TransactionExternalSystem `sql:"not null;type:transaction_external_system_t" json:"external_system"`
}

func (t *Transaction) GetHashID() uint64 {
	h := fnv.New64()
	_, _ = h.Write([]byte(t.ID))
	return h.Sum64()
}

type ManualTransaction struct {
	ID            string            `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	UserID        uint64            `gorm:"column:user_id" json:"user_id"`
	TxID          string            `gorm:"column:txid" json:"txid"`
	Address       string            `gorm:"column:address" json:"address"`
	Amount        *postgres.Decimal `sql:"type:decimal(32,16)"`
	FeeAmount     *postgres.Decimal `sql:"type:decimal(32,16)"`
	FeeCoin       string            `gorm:"column:fee_coin" json:"fee_coin"`
	TxType        TxType            `gorm:"not null;type:tx_type_t;default:'deposit'" json:"tx_type"`
	Status        TxStatus          `gorm:"not null;type:tx_status_t;default:'pending'" json:"status"`
	CoinSymbol    string            `gorm:"column:coin_symbol" json:"coin_symbol"`
	Confirmations int               `json:"confirmations"`
	ConfirmedBy   pq.Int64Array     `gorm:"not null;type:[]bigint" json:"confirmed_by"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type ManualTransactionResponse struct {
	*ManualTransaction `json:",inline"`
	Email              string `json:"email"`
}

type ManualTransactionList struct {
	ManualTransactions []ManualTransactionResponse `json:"manual_transactions"`
	Meta               PagingMeta                  `json:"meta"`
}

type CreateDepositRequest struct {
	UserID        string       `form:"user_id" json:"user_id"`
	Address       string       `form:"address" json:"address"`
	Amount        *decimal.Big `form:"amount,float64" json:"amount"`
	CoinSymbol    string       `form:"coin_symbol" json:"coin_symbol"`
	Confirmations string       `form:"confirmations" json:"confirmations"`
}

type CreateWithdrawalRequest struct {
	CreateDepositRequest
}

type ManualDepositsConfirmingUsersResponse struct {
	Emails []string `json:"emails"`
}

// TransactionWithUser strucutre
type TransactionWithUser struct {
	Transaction
	BlockchainExplorer string `json:"blockchain_explorer"`
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	UserEmail          string `gorm:"column:email" json:"user_email"`
}

// TransactionWithUserList structure
type TransactionWithUserList struct {
	Transactions []TransactionWithUser
	Meta         PagingMeta
}

// TransactionList structure
type TransactionList struct {
	Transactions []Transaction `json:"transactions"`
	Meta         PagingMeta    `json:"meta"`
}

type TransactionListWithUser struct {
	Transactions []TransactionWithUser `json:"transactions"`
	Meta         PagingMeta            `json:"meta"`
}

// NewDeposit create a new deposit tx
func NewDeposit(
	id string,
	userID uint64,
	status TxStatus,
	coinSymbol,
	txid,
	address string,
	amount *decimal.Big,
	confirmations int,
	externalSystem TransactionExternalSystem,
) *Transaction {
	return &Transaction{
		ID:             id,
		UserID:         userID,
		TxType:         TxType_Deposit,
		Status:         status,
		CoinSymbol:     coinSymbol,
		FeeCoin:        coinSymbol,
		TxID:           txid,
		Address:        address,
		Amount:         &postgres.Decimal{V: amount},
		FeeAmount:      &postgres.Decimal{V: ZERO},
		Confirmations:  confirmations,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		ExternalSystem: externalSystem,
	}
}

func NewManualDeposit(createDepositRequest *CreateDepositRequest) (*ManualTransaction, error) {
	userId, err := strconv.Atoi(createDepositRequest.UserID)
	if err != nil {
		return nil, err
	}
	id, err := gouuid.NewV4()
	if err != nil {
		return nil, err
	}

	return &ManualTransaction{
		ID:            id.String(),
		UserID:        uint64(userId),
		TxType:        TxType_Deposit,
		Status:        "unconfirmed",
		CoinSymbol:    createDepositRequest.CoinSymbol,
		FeeCoin:       createDepositRequest.CoinSymbol,
		TxID:          "",
		Address:       "",
		Amount:        &postgres.Decimal{V: createDepositRequest.Amount},
		FeeAmount:     &postgres.Decimal{V: ZERO},
		Confirmations: 0,
		ConfirmedBy:   make([]int64, 0),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}

func NewManualWithdrawal(createWithdrawalRequest *CreateWithdrawalRequest) (*ManualTransaction, error) {
	userId, err := strconv.Atoi(createWithdrawalRequest.UserID)
	if err != nil {
		return nil, err
	}
	id, err := gouuid.NewV4()
	if err != nil {
		return nil, err
	}

	return &ManualTransaction{
		ID:            id.String(),
		UserID:        uint64(userId),
		TxType:        TxType_Withdraw,
		Status:        "unconfirmed",
		CoinSymbol:    createWithdrawalRequest.CoinSymbol,
		FeeCoin:       createWithdrawalRequest.CoinSymbol,
		TxID:          "",
		Address:       "",
		Amount:        &postgres.Decimal{V: createWithdrawalRequest.Amount},
		FeeAmount:     &postgres.Decimal{V: ZERO},
		Confirmations: 0,
		ConfirmedBy:   make([]int64, 0),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}

// NewWithdraw create a new withdraw tx
func NewWithdraw(
	id string,
	userID uint64,
	status TxStatus,
	coinSymbol, feeCoin, txid, address string,
	amount *decimal.Big,
	feeAmount *decimal.Big,
	confirmations int,
	externalSystem TransactionExternalSystem,
) *Transaction {
	return &Transaction{
		ID:             id,
		UserID:         userID,
		TxType:         TxType_Withdraw,
		Status:         status,
		CoinSymbol:     coinSymbol,
		FeeCoin:        feeCoin,
		TxID:           txid,
		Address:        address,
		Amount:         &postgres.Decimal{V: amount},
		FeeAmount:      &postgres.Decimal{V: feeAmount},
		Confirmations:  confirmations,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		ExternalSystem: externalSystem,
	}
}

// MarshalJSON - convert the TransactionWithUser into a json
func (tx *TransactionWithUser) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                  tx.ID,
		"user_id":             tx.UserID,
		"tx_type":             tx.TxType,
		"status":              tx.Status,
		"coin_symbol":         tx.CoinSymbol,
		"fee_coin":            tx.FeeCoin,
		"txid":                tx.TxID,
		"address":             tx.Address,
		"amount":              fmt.Sprintf("%f", tx.Amount.V),
		"fee_amount":          fmt.Sprintf("%f", tx.FeeAmount.V),
		"confirmations":       tx.Confirmations,
		"created_at":          tx.CreatedAt,
		"updated_at":          tx.UpdatedAt,
		"first_name":          tx.FirstName,
		"last_name":           tx.LastName,
		"email":               tx.UserEmail,
		"blockchain_explorer": tx.BlockchainExplorer,
		"external_system":     tx.ExternalSystem,
	})
}

// MarshalJSON - convert the transaction into a json
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":            tx.ID,
		"user_id":       tx.UserID,
		"tx_type":       tx.TxType,
		"status":        tx.Status,
		"coin_symbol":   tx.CoinSymbol,
		"fee_coin":      tx.FeeCoin,
		"txid":          tx.TxID,
		"address":       tx.Address,
		"amount":        fmt.Sprintf("%f", tx.Amount.V),
		"fee_amount":    fmt.Sprintf("%f", tx.FeeAmount.V),
		"confirmations": tx.Confirmations,
		"created_at":    tx.CreatedAt,
		"updated_at":    tx.UpdatedAt,
	})
}

type Balance struct {
	ID              uint64            `sql:"type:bigint" gorm:"primary_key" json:"id"`
	UserID          uint64            `sql:"type:bigint" json:"-"`
	LastLiabilityID uint64            `sql:"type:bigint" json:"-"`
	CoinSymbol      string            `gorm:"column:coin_symbol" json:"coin_symbol"`
	Available       *postgres.Decimal `json:"available"`
	Locked          *postgres.Decimal `json:"locked"`
	InOrders        *postgres.Decimal `json:"in_orders"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	SubAccount      uint64            `sql:"type:bigint" gorm:"column:sub_account" json:"sub_account"`
}

// BalanceView godoc
type BalanceView struct {
	Available *decimal.Big `json:"available"`
	Locked    *decimal.Big `json:"locked"`
	InOrders  *decimal.Big `json:"in_orders"`
}

// MarshalJSON - convert the transaction into a json
func (balance *BalanceView) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"available": utils.Fmt(balance.Available),
		"locked":    utils.Fmt(balance.Locked),
		"in_orders": utils.Fmt(balance.InOrders),
	})
}

func (balance *BalanceView) GetTotal() *decimal.Big {
	return conv.NewDecimalWithPrecision().Add(balance.Available, balance.Locked)
}

// Balances godoc
type Balances map[string]BalanceView

// WalletEvent godoc
type WalletEvent struct {
	Symbol       string       `json:"symbol"`
	Op           string       `json:"op"`
	Amount       *decimal.Big `json:"amount"`
	LockedAmount *decimal.Big `json:"locked"`
	InOrder      *decimal.Big `json:"in_order"`
	InBTC        *decimal.Big `json:"in_btc"`
}

// MarshalJSON - convert the transaction into a json
func (tx *WalletEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"symbol":   tx.Symbol,
		"amount":   utils.Fmt(tx.Amount),
		"locked":   utils.Fmt(tx.LockedAmount),
		"in_order": utils.Fmt(tx.InOrder),
		"in_btc":   utils.Fmt(tx.InBTC),
	})
}

func (manualTransaction *ManualTransactionResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"ID":            manualTransaction.ID,
		"user_id":       manualTransaction.UserID,
		"txid":          manualTransaction.TxID,
		"address":       manualTransaction.Address,
		"amount":        utils.Fmt(manualTransaction.Amount.V),
		"fee_amount":    utils.Fmt(manualTransaction.FeeAmount.V),
		"fee_coin":      manualTransaction.FeeCoin,
		"tx_type":       manualTransaction.TxType,
		"status":        manualTransaction.Status,
		"coin_symbol":   manualTransaction.CoinSymbol,
		"confirmations": manualTransaction.Confirmations,
		"confirmed_by":  manualTransaction.ConfirmedBy,
		"created_at":    manualTransaction.CreatedAt,
		"updated_at":    manualTransaction.UpdatedAt,
		"email":         manualTransaction.Email,
	})
}

func (manualTransaction *ManualTransaction) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"ID":            manualTransaction.ID,
		"user_id":       manualTransaction.UserID,
		"txid":          manualTransaction.TxID,
		"address":       manualTransaction.Address,
		"amount":        manualTransaction.Amount.V,
		"fee_amount":    manualTransaction.FeeAmount.V,
		"fee_coin":      manualTransaction.FeeCoin,
		"tx_type":       manualTransaction.TxType,
		"status":        manualTransaction.Status,
		"coin_symbol":   manualTransaction.CoinSymbol,
		"confirmations": manualTransaction.Confirmations,
		"confirmed_by":  manualTransaction.ConfirmedBy,
		"created_at":    manualTransaction.CreatedAt,
		"updated_at":    manualTransaction.UpdatedAt,
	})
}
