package model

import (
	"encoding/json"
	"mime/multipart"
	"time"
)

//type KYBBusinessRegistration struct {
//	ID                       uint64
//	UserID                   uint64
//	KYBID                    uint64
//	BusinessName             string
//	RegistrationNumber       string
//	DateOfIncorporation      time.Time
//	Phone                    string
//	Website                  string
//	SourceOfFunds            string
//	NatureOfBusiness         string
//	ApplicationReasons       string
//	RegisteredCountry        string
//	RegisteredCity           string
//	RegisteredAddress        string
//	RegisteredZipcode        string
//	OperatingBusinessCountry string
//	OperatingBusinessCity    string
//	OperatingBusinessAddress string
//	OperatingBusinessZipcode string
//}

type KYBBusinessRegistration struct {
	ID                       uint64    `gorm:"column:id"`
	UserID                   uint64    `gorm:"column:user_id"`
	KYBID                    uint64    `gorm:"column:kyb_id"`
	BusinessName             string    `form:"business_name" gorm:"column:business_name"`
	RegistrationNumber       string    `form:"registrations_number" gorm:"column:registration_number"`
	DateOfIncorporation      time.Time `form:"date_of_incorporation" gorm:"column:date_of_incorporation" time_format:"02/01/2006"`
	Phone                    string    `form:"phone" gorm:"column:phone"`
	Website                  string    `form:"website" gorm:"column:website"`
	SourceOfFunds            string    `form:"source_of_funds" gorm:"column:source_of_funds"`
	SourceOfCapital          string    `form:"source_of_capital" gorm:"column:source_of_capital"` // Add this line
	NatureOfBusiness         string    `form:"nature_of_business" gorm:"column:nature_of_business"`
	ApplicationReasons       string    `form:"applications_reasons" gorm:"column:application_reasons"`
	RegisteredCountry        string    `form:"registered_country" gorm:"column:registered_country"`
	RegisteredCity           string    `form:"registered_city" gorm:"column:registered_city"`
	RegisteredAddress        string    `form:"registered_address" gorm:"column:registered_address"`
	RegisteredZipcode        string    `form:"registered_zipcode" gorm:"column:registered_zipcode"`
	OperatingBusinessCountry string    `form:"operating_business_country" gorm:"column:operating_business_country"`
	OperatingBusinessCity    string    `form:"operating_business_city" gorm:"column:operating_business_city"`
	OperatingBusinessAddress string    `form:"operating_business_address" gorm:"column:operating_business_address"`
	OperatingBusinessZipcode string    `form:"operating_business_zipcode" gorm:"column:operating_business_zipcode"`
}

type UserBusinessDetailsSchema struct {
	ID                                 uint64
	UserID                             uint64
	KYBID                              uint64
	BusinessName                       string
	RegistrationNumber                 string
	DateOfIncorporation                time.Time
	Phone                              string
	Website                            string
	SourceOfFunds                      string
	NatureOfBusiness                   string
	ApplicationReasons                 string
	RegisteredCountry                  string
	RegisteredCity                     string
	RegisteredAddress                  string
	RegisteredZipcode                  string
	OperatingBusinessCountry           string
	OperatingBusinessCity              string
	OperatingBusinessAddress           string
	OperatingBusinessZipcode           string
	CertificateOfIncorporationName     string `gorm:"column:certificate_of_incorporation"`
	MemorandumArticleOfAssociationName string `gorm:"column:memorandum_article_association"`
	RegisterDirectorsName              string `gorm:"column:register_directors"`
	RegisterOfMembers                  string `gorm:"column:register_of_members"`
	OwnershipStructure                 string `gorm:"column:ownership_structure"`
	SanctionsQuestionnaire             string `gorm:"column:sanctions_questionnaire"`
}

func (s UserBusinessDetailsSchema) Schema() string {
	return "user_business_details"
}

//func (k *KYBBusinessRegistration) MoveDataFromRequest(request KYBBusinessRegistrationRequest, userID uint64) {
//	k.UserID = userID
//	k.BusinessName = request.BusinessName
//	k.RegistrationNumber = request.RegistrationNumber
//	k.Phone = request.Phone
//	k.Website = request.Website
//	k.SourceOfFunds = request.SourceOfFunds
//	k.NatureOfBusiness = request.NatureOfBusiness
//	k.ApplicationReasons = request.ApplicationReasons
//	k.RegisteredCountry = request.RegisteredCountry
//	k.RegisteredCity = request.RegisteredCity
//	k.RegisteredAddress = request.RegisteredAddress
//	k.RegisteredZipcode = request.RegisteredZipcode
//	k.OperatingBusinessCountry = request.OperatingBusinessCountry
//	k.OperatingBusinessCity = request.OperatingBusinessCity
//	k.OperatingBusinessAddress = request.OperatingBusinessAddress
//	k.OperatingBusinessZipcode = request.OperatingBusinessZipcode
//
//	//if request.DateOfIncorporation != "" {
//	//	k.DateOfIncorporation, _ = time.Parse(time.RFC3339, request.DateOfIncorporation)
//	//}
//}

type KYBDocumentsRegistration struct {
	ID                             uint64
	UserID                         uint64
	CertificateOfIncorporation     []byte
	MemorandumArticleOfAssociation []byte
	RegisterDirectors              []byte
	RegisterOfMembers              []byte
	OwnershipStructure             []byte
	SanctionsQuestionnaire         []byte
}

type KYBs struct {
	ID                 uint64
	Status             string
	VerificationData   []byte
	VerificationResult uint8
	StepOneStatus      string
}

type KYBStatus string

func NewKYBStatus(status string) KYBStatus {
	return KYBStatus(status)
}

func (k KYBStatus) ToString() string {
	return string(k)
}

func (k *KYBStatus) SetStatus(status string) {
	*k = KYBStatus(status)
}

const (
	KYBStatusNone             KYBStatus = "none"
	KYBStatusStepOneFail      KYBStatus = "step_one_fail"
	KYBStatusStepTwoFail      KYBStatus = "step_two_fail"
	KYBStatusStepThreeFail    KYBStatus = "step_three_fail"
	KYBStatusStepFourFail     KYBStatus = "step_four_fail"
	KYBStatusStepOnePending   KYBStatus = "step_one_pending"
	KYBStatusStepTwoPending   KYBStatus = "step_two_pending"
	KYBStatusStepThreePending KYBStatus = "step_three_pending"
	KYBStatusStepFourPending  KYBStatus = "step_four_pending"
	KYBStatusStepOneSuccess   KYBStatus = "step_one_success"
	KYBStatusStepTwoSuccess   KYBStatus = "step_two_success"
	KYBStatusStepThreeSuccess KYBStatus = "step_three_success"
	KYBStatusStepFourSuccess  KYBStatus = "step_four_success"
	KYBStatusSuccess          KYBStatus = "success"
	KYBStatusFail             KYBStatus = "fail"
)

type KYBSchema struct {
	ID                 uint64
	Status             KYBStatus
	VerificationData   []byte
	VerificationResult uint8
	StepOneStatus      KYBStatus
	StepTwoStatus      KYBStatus
	StepThreeStatus    KYBStatus
	StepFourStatus     KYBStatus
}

func NewKYBSchema() KYBSchema {
	return KYBSchema{
		Status:          "pending",
		StepOneStatus:   KYBStatusNone,
		StepTwoStatus:   KYBStatusNone,
		StepThreeStatus: KYBStatusNone,
		StepFourStatus:  KYBStatusNone,
	}
}

func (k *KYBSchema) Schema() string {
	return "kybs"
}

type ShuftiProKYBRequest struct {
	ShuftiProAuthRequest
	KYB KYBCompanyName `json:"kyb"`
}

type ShuftiProAuthRequest struct {
	Reference        string `json:"reference"`
	CallbackURL      string `json:"callback_url"`
	Country          string `json:"country"`
	Email            string `json:"email"`
	Language         string `json:"language"`
	VerificationMode string `json:"verification_mode"`
}

type FaceShuftiPro struct {
	Proof string `json:"proof"`
}

type ShuftiProKYBResponse struct {
	Reference          string                      `json:"reference"`
	Error              ShuftiProError              `json:"error"`
	Event              ShuftiProEvent              `json:"event"`
	VerificationData   ShuftiProVerificationData   `json:"verification_data"`
	VerificationResult ShuftiProVerificationResult `json:"verification_result"`
}

type ShuftiProVerificationResult struct {
	KYBService uint8 `json:"kyb_service"`
}

type ShuftiProVerificationData struct {
	Kyb []json.RawMessage `json:"kyb,omitempty"`
}

type ShuftiProEvent string

const (
	ShuftiProVerificationAccepted ShuftiProEvent = "verification.accepted"
	ShuftiProVerificationDeclined ShuftiProEvent = "verification.declined"
	ShuftiProRequestError         ShuftiProEvent = "request.invalid"
)

type ShuftiProError struct {
	Service string `json:"service,omitempty"`
	Key     string `json:"key,omitempty"`
	Message string `json:"message,omitempty"`
}

type KYBCompanyName struct {
	CompanyName string `json:"company_name"`
}

//----------------------------------------------

type KYBMemberRegistration struct {
	FirstName            string                  `form:"first_name" binding:"required"`
	MiddleName           string                  `form:"middle_name"`
	LastName             string                  `form:"last_name" binding:"required"`
	Gender               GenderType              `form:"gender" binding:"required"`
	DOB                  time.Time               `form:"date_of_birth" binding:"required" time_format:"02/01/2006"`
	IdentificationNumber string                  `form:"identification_number" binding:"required"`
	Email                string                  `form:"email" binding:"required"`
	Timestamp            uint64                  `form:"timestamp"` // deleted binding:"required"
	Country              string                  `form:"country" binding:"required"`
	Address              string                  `form:"address" binding:"required"`
	City                 string                  `form:"city" binding:"required"`
	PostalCode           string                  `form:"postal_code" binding:"required"`
	DocumentType         string                  `form:"document_type" binding:"required"`
	MemberRole           BusinessRole            `form:"role" binding:"required"`
	DocumentImages       []*multipart.FileHeader `form:"document_images" binding:"required"`
	SelfieImage          []*multipart.FileHeader `form:"selfie_image" binding:"required"`
}

type BusinessRole string

const (
	UBOs         BusinessRole = "ubos"
	Director     BusinessRole = "director"
	UBOSDirector BusinessRole = "ubosdirector"
)

func (b BusinessRole) ToString() string {
	return string(b)
}

type BusinessMembersSchema struct {
	ID                   uint64 //`gorm:"primaryKey;autoIncrement"`
	BusinessDetailsID    uint64
	BusinessRole         BusinessRole
	Timestamp            uint64 `gorm:"column:time_stamp"`
	IsActive             bool
	KycID                uint64
	FirstName            string
	LastName             string
	MiddleName           string
	DOB                  time.Time
	Gender               GenderType
	Country              string
	IdentificationNumber string
	Email                string
}

func (b *BusinessMembersSchema) Schema() string {
	return "business_members"
}
