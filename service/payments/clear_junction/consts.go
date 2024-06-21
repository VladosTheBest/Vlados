package clear_junction

type PaymentType int

func (p PaymentType) IsValidWithdrawal() bool {
	switch p {
	case PaymentTypeWithdrawalSEPA,
		PaymentTypeWithdrawalBankTransferCHAPS,
		PaymentTypeWithdrawalFasterPayments,
		PaymentTypeWithdrawalInternal,
		// PaymentTypeWithdrawalCreditCard,
		PaymentTypeWithdrawalQiwi,
		PaymentTypeWithdrawalYandexMoney,
		PaymentTypeWithdrawalBankTransferSCT,
		PaymentTypeWithdrawalBankTransferSWIFT:
		return true
	default:
		return false
	}
}

const (
	PaymentTypeDepositBankTransferSEPA     PaymentType = 1 // instant
	PaymentTypeDepositBankTransferCHAPS    PaymentType = 2
	PaymentTypeWithdrawalSEPA              PaymentType = 3 // instant
	PaymentTypeWithdrawalBankTransferCHAPS PaymentType = 4
	PaymentTypeWithdrawalFasterPayments    PaymentType = 5
	PaymentTypeWithdrawalInternal          PaymentType = 6
	// PaymentTypeWithdrawalCreditCard        PaymentType = 7
	PaymentTypeWithdrawalQiwi              PaymentType = 8
	PaymentTypeWithdrawalYandexMoney       PaymentType = 9
	PaymentTypeWithdrawalBankTransferSCT   PaymentType = 10
	PaymentTypeWithdrawalBankTransferSWIFT PaymentType = 11
)

type TransactionAction string

const (
	TransactionActionApprove TransactionAction = "approve"
	TransactionActionCancel  TransactionAction = "cancel"
)

func (t TransactionAction) String() string {
	return string(t)
}

func GetWithdrawalsMethodsList() map[PaymentType]string {
	return map[PaymentType]string{
		PaymentTypeWithdrawalSEPA:              "Bank Transfer (SEPA Instant)",
		PaymentTypeWithdrawalBankTransferSCT:   "Bank Transfer (SEPA Standard)",
		PaymentTypeWithdrawalBankTransferCHAPS: "Bank Transfer CHAPS",
		PaymentTypeWithdrawalFasterPayments:    "Faster Payments",
		PaymentTypeWithdrawalInternal:          "Internal",
		//PaymentTypeWithdrawalCreditCard:        "Credit Card",
		PaymentTypeWithdrawalQiwi:              "Qiwi",
		PaymentTypeWithdrawalYandexMoney:       "Yandex Money",
		PaymentTypeWithdrawalBankTransferSWIFT: "Bank Transfer SWIFT",
	}
}

func GetWithdrawalsMethodsFieldsList() map[PaymentType][]string {
	return map[PaymentType][]string{
		PaymentTypeWithdrawalSEPA: {"iban"}, // SEPA Inst
		//PaymentTypeWithdrawalBankTransferSCT:   {"iban", "bankSwiftCode"},
		PaymentTypeWithdrawalBankTransferSCT:   {"iban"}, // SEPA Standard
		PaymentTypeWithdrawalBankTransferCHAPS: {"sortCode", "accountNumber", "bankSwiftCode"},
		PaymentTypeWithdrawalFasterPayments:    {"sortCode", "accountNumber"},
		PaymentTypeWithdrawalInternal:          {"iban"},
		//PaymentTypeWithdrawalCreditCard:      {},
		PaymentTypeWithdrawalQiwi:              {"accountNumber"},
		PaymentTypeWithdrawalYandexMoney:       {"accountNumber"},
		PaymentTypeWithdrawalBankTransferSWIFT: {"bankAccountNumber", "bankSwiftCode"},
	}
}
