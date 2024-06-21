package clear_junction

type CheckIbanResponse struct {
	RequestReference  string `json:"requestReference,omitempty"`
	BankSwiftCode     string `json:"bankSwiftCode,omitempty"`
	BankName          string `json:"bankName,omitempty"`
	SepaReachable     bool   `json:"sepaReachable,omitempty"`
	SepaInstReachable bool   `json:"sepaInstReachable,omitempty"`
}

type actionWithTransaction struct {
	OrderReferenceArray []string `json:"orderReferenceArray,omitempty"`
}
