package model

type RequestOperationType string

const (
	RequestOperationType_All    RequestOperationType = "all"
	RequestOperationType_Crypto RequestOperationType = "crypto"
	RequestOperationType_Fiat   RequestOperationType = "fiat"
)
