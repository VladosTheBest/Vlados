package model

type AccountGroup string

const (
	AccountGroupMain        AccountGroup = "main"
	AccountGroupBonus       AccountGroup = "bonus"
	AccountGroupStaking     AccountGroup = "staking"
	AccountGroupCardPayment AccountGroup = "card_payment"
)

func (ag AccountGroup) String() string {
	return string(ag)
}

var AccountGroupList = []AccountGroup{
	AccountGroupMain,
	AccountGroupBonus,
	AccountGroupStaking,
	AccountGroupCardPayment,
}
