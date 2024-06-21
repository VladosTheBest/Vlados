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
	"errors"
	"mime/multipart"
	"time"
)

// KYCStatus defined the list of possible kyc statuses
type KYCStatus string

type KYCCallbackStatus string

const (
	KYCStatusNone             KYCStatus = "none"
	KYCStatusStepOneFail      KYCStatus = "step_one_fail"
	KYCStatusStepTwoFail      KYCStatus = "step_two_fail"
	KYCStatusStepThreeFail    KYCStatus = "step_three_fail"
	KYCStatusStepTwoPending   KYCStatus = "step_two_pending"
	KYCStatusStepThreePending KYCStatus = "step_three_pending"
	KYCStatusStepOneSuccess   KYCStatus = "step_one_success"
	KYCStatusStepTwoSuccess   KYCStatus = "step_two_success"
	KYCStatusStepThreeSuccess KYCStatus = "step_three_success"
)

const (
	KYCCallbackStatusPending KYCCallbackStatus = "pending"
	KYCCallbackStatusSuccess KYCCallbackStatus = "success"
	KYCCallbackStatusError   KYCCallbackStatus = "error"
)

type KYCStatusErrorReason string

const (
	KYCStatusErrorReasonLastNameMismatch  KYCStatusErrorReason = "last_name_mismatch"
	KYCStatusErrorReasonFirstNameMismatch KYCStatusErrorReason = "first_name_mismatch"
	KYCStatusErrorReasonGenderMismatch    KYCStatusErrorReason = "gender_mismatch"
	KYCStatusErrorReasonDoBMismatch       KYCStatusErrorReason = "dob_mismatch"
	KYCStatusErrorReasonCountryMismatch   KYCStatusErrorReason = "country_mismatch"
	KYCStatusErrorReasonInvalidSelfie     KYCStatusErrorReason = "invalid_selfie"
)

const (
	KYCApprove string = "Approved"
	KYCDecline string = "Declined"
)

type JumioKycDocumentType string

const (
	KycIDCard         JumioKycDocumentType = "id_card"
	KycPassport       JumioKycDocumentType = "passport"
	KycDrivingLicense JumioKycDocumentType = "driving_license"
	KycVisa           JumioKycDocumentType = "visa"
	KycSelfie         JumioKycDocumentType = "selfie"
)

func DetermineKYCDocumentType(documentType string) JumioKycDocumentType {
	switch documentType {
	case "passport":
		return KycPassport
	case "id_card", "nationalid":
		return KycIDCard
	case "driving_license", "driverlicense":
		return KycDrivingLicense
	case "visa":
		return KycVisa
	default:
		return ""
	}
}

type JumioIdVerificationStatus string

const (
	JumioIdVerificationStatusApproved                   JumioIdVerificationStatus = "APPROVED_VERIFIED"
	JumioIdVerificationStatusDeniedFraud                JumioIdVerificationStatus = "DENIED_FRAUD"
	JumioIdVerificationStatusDeniedUnsupportedIdType    JumioIdVerificationStatus = "DENIED_UNSUPPORTED_ID_TYPE"
	JumioIdVerificationStatusDeniedUnsupportedIdCountry JumioIdVerificationStatus = "DENIED_UNSUPPORTED_ID_COUNTRY"
	JumioIdVerificationStatusErrorNotReadableId         JumioIdVerificationStatus = "ERROR_NOT_READABLE_ID"
	JumioIdVerificationStatusNoIdUploaded               JumioIdVerificationStatus = "NO_ID_UPLOADED"
)

type KycResponseStatus string

const (
	KycResponseStatusSuccess KycResponseStatus = "Success"
)

type KycRegistrationRecommendation string

const (
	KycRegistrationRecommendationApprove KycRegistrationRecommendation = "Approve"
	KycRegistrationRecommendationReject  KycRegistrationRecommendation = "Reject"
)

// KYC structure
type KYC struct {
	ID                uint64            `sql:"type: bigint" gorm:"primary_key"`
	RegistrationID    int64             `json:"registration_id"`
	StepOneResponse   string            `json:"step_one_response"`
	StepTwoResponse   string            `json:"step_two_response"`
	StepThreeResponse string            `json:"step_three_response"`
	CallbackResponse  string            `json:"callback_response"`
	Status            KYCStatus         `json:"status"`
	StatusReason      string            `json:"status_reason"`
	Level2Step        uint64            `gorm:"column:level2_step" json:"level2_step"`
	ReferenceID       string            `gorm:"column:reference_id" json:"reference_id"`
	CallbackStatus    KYCCallbackStatus `gorm:"column:callback_status" json:"-"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type StepOneKycResponse struct {
	ConfidenceLevel float64         `json:"confidence_level"`
	Description     string          `json:"description"`
	Id              string          `json:"id"`
	Rec             string          `json:"rec"`
	RulesTriggered  []RuleTriggered `json:"rules_triggered"`
}

type RuleTriggered struct {
	DisplayToMerchant uint64 `json:"display_to_merchant"`
	Name              string `json:"name"`
	Score             string `json:"score"`
}

type StepTwoKycResponse struct {
	KycSource   string `json:"kyc_source"`
	Status      int64  `json:"status"`
	Description string `json:"description"`
	ReferenceId string `json:"reference_id"`
}

type StepTwoKycResponseV3 struct {
	JumioIdScanReference string    `json:"jumioIdScanReference"`
	Timestamp            time.Time `json:"timestamp"`
}

func (stepOneResponse *StepOneKycResponse) IsSuccess() bool {
	return stepOneResponse.Description == string(KycResponseStatusSuccess)
}

func (stepTwoResponse *StepTwoKycResponse) IsSuccess() bool {
	return stepTwoResponse.Description == string(KycResponseStatusSuccess)
}

func (kycCallbackStatus KYCCallbackStatus) IsSuccess() bool {
	return kycCallbackStatus == KYCCallbackStatusSuccess
}

func (stepOneResponse *StepOneKycResponse) IsRegistrationApproved() bool {
	return stepOneResponse.Rec == string(KycRegistrationRecommendationApprove)
}

type KYCCallbackResponse struct {
	Data          DataCallbackResponse `form:"data" json:"data" binding:"required"`
	KycSource     string               `form:"kyc_source" json:"kyc_source" binding:"required"`
	ReferenceID   string               `form:"reference_id" json:"reference_id" binding:"required"`
	ScoreComplete int                  `form:"score_complete" json:"score_complete" binding:"required"`
}

type ShuftiProKYCRequest struct {
	ShuftiProAuthRequest
	ShuftiProFaceRequest     `json:"face"`
	ShuftiProDocumentRequest `json:"document"`
}

type ShuftiProFaceRequest struct {
	Selfie string `json:"proof"`
}

type ShuftiProName struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type ShuftiProKYCResponse struct {
	Reference          string          `json:"reference"`
	Event              ShuftiProEvent  `json:"event"`
	Error              ShuftiProError  `json:"error,omitempty"`
	VerificationResult json.RawMessage `json:"verification_result"`
	DeclinedReason     string          `json:"declined_reason"`
}

type ShuftiProKYCVerificationResult []byte

// MarshalJSON returns m as the JSON encoding of m.
func (m ShuftiProKYCVerificationResult) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return m, nil
}

// UnmarshalJSON sets *m to a copy of data.
func (m *ShuftiProKYCVerificationResult) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.New("ShuftiProKYCVerificationResult: UnmarshalJSON on nil pointer")
	}
	*m = append((*m)[0:0], data...)
	return nil
}

type ShuftiProDocumentRequest struct {
	Proof                 string        `json:"proof"`
	AdditionalProof       string        `json:"additional_proof"`
	SupportedTypes        []string      `json:"supported_types"`
	Name                  ShuftiProName `json:"name"`
	Dob                   string        `json:"dob"`
	Gender                string        `json:"gender"`
	FetchEnhancedData     string        `json:"fetch_enhanced_data"`
	BacksideProofRequired string        `json:"backside_proof_required"`
}

type KYCCallbackResponseV3 struct {
	CallBackType       string    `form:"callBackType" json:"callBackType"`
	JumioScanReference string    `form:"jumioIdScanReference" json:"jumioIdScanReference"`
	VerificationStatus string    `form:"verificationStatus" json:"verificationStatus"`
	IdScanStatus       string    `form:"idScanStatus" json:"idScanStatus"`
	TransactionDate    time.Time `form:"transactionDate" json:"transactionDate"`
	CallbackDate       time.Time `form:"callbackDate" json:"callbackDate"`
}

type DataCallbackResponse struct {
	Decision       string `form:"decision" json:"decision"`
	ControlsResult string `form:"controls_result" json:"controls_result"`
}

type JumioRequest struct {
	FrontsideImage          string `json:"frontsideImage,omitempty"`
	FaceImage               string `json:"faceImage,omitempty"`
	BacksideImage           string `json:"backsideImage,omitempty"`
	FirstName               string `json:"firstName,omitempty"`
	LastName                string `json:"lastName,omitempty"`
	DOB                     string `json:"dob,omitempty"`
	Country                 string `json:"country,omitempty"`
	MerchantIdScanReference string `json:"merchantIdScanReference,omitempty"`
	CallbackUrl             string `json:"callbackUrl,omitempty"`
	CustomerId              string `json:"customerId,omitempty"`
	IdType                  string `json:"idType,omitempty"`
}

type JumioIdVerificationResult struct {
	Timestamp     time.Time `json:"timestamp"`
	ScanReference string    `json:"scanReference"`
	Document      struct {
		Type           string `json:"type"`
		LastName       string `json:"lastName"`
		FirstName      string `json:"firstName"`
		Gender         string `json:"gender"`
		Dob            string `json:"dob"`
		IssuingCountry string `json:"issuingCountry"`
		Status         string `json:"status"`
	} `json:"document"`
	Verification struct {
		IdentityVerification struct {
			Similarity string `json:"similarity"`
			Validity   string `json:"validity"`
			Reason     string `json:"reason"`
		} `json:"identityVerification"`
	} `json:"verification"`
	RejectReason struct {
		RejectReasonCode        string `json:"rejectReasonCode"`
		RejectReasonDescription string `json:"rejectReasonDescription"`
		RejectReasonDetails     string `json:"rejectReasonDetails"`
	} `json:"rejectReason"`
}

// NewKYC creates a new kyc structure
func NewKYC(status KYCStatus) *KYC {
	return &KYC{
		CallbackStatus: KYCCallbackStatusPending,
		Status:         status,
	}
}

// GetKycStatusFromString returns the KYCStatus for a string
func GetKycStatusFromString(s string) (KYCStatus, error) {
	switch s {
	case "none":
		return KYCStatusNone, nil
	case "step_one_fail":
		return KYCStatusStepOneFail, nil
	case "step_two_fail":
		return KYCStatusStepTwoFail, nil
	case "step_three_fail":
		return KYCStatusStepThreeFail, nil
	case "step_two_pending":
		return KYCStatusStepTwoPending, nil
	case "step_three_pending":
		return KYCStatusStepThreePending, nil
	case "step_one_success":
		return KYCStatusStepOneSuccess, nil
	case "step_two_success":
		return KYCStatusStepTwoSuccess, nil
	case "step_three_success":
		return KYCStatusStepThreeSuccess, nil
	default:
		return KYCStatusNone, errors.New("Status is not valid")
	}
}

// KYCDocument describes a set of files of a single KYC document to verify
type KYCDocument struct {
	Type       JumioKycDocumentType    `json:"documentType"`
	Files      []*multipart.FileHeader `json:"files"`
	SavedFiles []string                `json:"-"`
}

// KYCDocuments describes a set of KYC documents of a single user to verify
type KYCDocuments struct {
	UserID    uint64        `json:"userId"`
	Documents []KYCDocument `json:"documents"`
}

type genderType4stop string

const (
	Male   genderType4stop = "M"
	Female genderType4stop = "F"
)

func (s genderType4stop) String() string {
	return string(s)
}

type KycCustomerRegistrationRequest struct {
	FirstName  string     `form:"first_name" binding:"required"`
	LastName   string     `form:"last_name" binding:"required"`
	Gender     GenderType `form:"gender" binding:"required"`
	DOB        time.Time  `form:"date_of_birth" binding:"required" time_format:"02/01/2006"`
	Country    string     `form:"country" binding:"required"`
	Address    string     `form:"address" binding:"required"`
	City       string     `form:"city" binding:"required"`
	PostalCode string     `form:"postal_code" binding:"required"`
	IP         string
	UserID     uint64
}

type KycCustomerInfo struct {
	KycID      uint64
	UserID     uint64
	FirstName  string     `form:"first_name" binding:"required"`
	LastName   string     `form:"last_name" binding:"required"`
	Gender     GenderType `form:"gender" binding:"required"`
	DOB        time.Time  `form:"date_of_birth" binding:"required" time_format:"02/01/2006"`
	Country    string     `form:"country" binding:"required"`
	PostalCode string     `form:"postal_code" binding:"required"`
}
