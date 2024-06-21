package model

import (
	"encoding/json"
	"errors"
	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	postgresDialects "github.com/jinzhu/gorm/dialects/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"time"
)

// LaunchpadStatus godoc
type LaunchpadStatus string

const (
	// LaunchpadStatus active
	LaunchpadStatusActive LaunchpadStatus = "active"
	// LaunchpadStatus disabled
	LaunchpadStatusInactive LaunchpadStatus = "inactive"
	// LaunchpadStatus pending
	LaunchpadStatusPending LaunchpadStatus = "pending"
	// LaunchpadStatus deleted
	LaunchpadStatusDeleted LaunchpadStatus = "deleted"
)

func GetLaunchpadStatusFromString(s string) (LaunchpadStatus, error) {
	switch s {
	case "active":
		return LaunchpadStatusActive, nil
	case "inactive":
		return LaunchpadStatusInactive, nil
	case "deleted":
		return LaunchpadStatusDeleted, nil
	default:
		return LaunchpadStatusPending, errors.New("Status is not valid")
	}
}

type Launchpad struct {
	ID               uint64                 `sql:"type: bigint" gorm:"primary_key"`
	Logo             string                 `json:"logo"`
	Title            string                 `json:"title"`
	CoinSymbol       string                 `json:"coin_symbol"`
	ContributionsCap *postgres.Decimal      `json:"contributions_cap"`
	LineLevels       postgresDialects.Jsonb `json:"line_levels"`
	Details          string                 `json:"details"`
	Blockchain       string                 `json:"blockchain"`
	TokenDetails     string                 `json:"token_details"`
	ProjectInfo      string                 `json:"project_info"`
	SocialMediaLinks postgresDialects.Jsonb `json:"social_media_links"`
	StartDate        time.Time              `json:"start_date"`
	EndDate          time.Time              `json:"end_date"`
	Timezone         string                 `json:"timezone"`
	PresalePrice     *postgres.Decimal      `json:"presale_price"`
	Status           LaunchpadStatus        `sql:"not null;type:launchpad_status_t;default:'active'" json:"status"`
	ShortInfo        string                 `json:"short_info"`
}

type GetLaunchpadResponse struct {
	Launchpad                Launchpad
	TotalContribution        *decimal.Big        `json:"total_contribution"`
	TotalUserContribution    *decimal.Big        `json:"user_contribution"`
	BoughtTokensByLineLevels LaunchpadLineLevels `json:"bought_tokens_by_line_levels"`
}

type LaunchpadRequest struct {
	Logo             string                 `form:"-" json:"logo"`
	Title            string                 `form:"title" json:"title"`
	CoinSymbol       string                 `form:"coin_symbol" json:"coin_symbol" binding:"required,lowercase,alphanum"`
	ContributionsCap *decimal.Big           `form:"contributions_cap,float64" json:"contributions_cap" binding:"required,numeric"`
	LineLevels       postgresDialects.Jsonb `form:"line_levels" json:"line_levels" binding:"json"`
	Details          string                 `form:"details" json:"details"`
	Blockchain       string                 `form:"blockchain" json:"blockchain"`
	TokenDetails     string                 `form:"token_details" json:"token_details"`
	ProjectInfo      string                 `form:"project_info" json:"project_info"`
	SocialMediaLinks postgresDialects.Jsonb `form:"social_media_links" json:"social_media_links" binding:"json"`
	StartDate        time.Time              `form:"start_date" json:"start_date"`
	EndDate          time.Time              `form:"end_date" json:"end_date"`
	Timezone         string                 `form:"timezone" json:"timezone"`
	PresalePrice     *decimal.Big           `form:"presale_price,float64" json:"presale_price" binding:"required,numeric"`
	Status           string                 `form:"status" json:"status"`
	ShortInfo        string                 `form:"short_info" json:"short_info"`
}

type LaunchpadOrder struct {
	ID            uint64            `sql:"type: bigint" gorm:"primary_key"`
	LaunchpadId   uint64            `gorm:"column:launchpad_id"`
	UserID        uint64            `gorm:"column:user_id"`
	RefId         string            `gorm:"column:ref_id"`
	TokenAmount   *postgres.Decimal `json:"token_amount"`
	SpentAmount   *postgres.Decimal `json:"spent_amount"`
	CreatedAt     time.Time         `gorm:"column:created_at"`
	UpdatedAt     time.Time         `gorm:"column:updated_at"`
	UserLineLevel string            `json:"user_line_level"`
}

type AdminLaunchpadList struct {
	Launchpads []Launchpad
	Meta       PagingMeta
}

type LaunchpadList struct {
	Launchpads []Launchpad
}

type LaunchpadFullInfoList struct {
	Launchpads []GetLaunchpadResponse
}

type LaunchpadMakePaymentRequest struct {
	Launchpad     *Launchpad
	UserId        uint64
	Amount        *decimal.Big
	UserLineLevel string
}

type LaunchpadLineLevels struct {
	Silver   *decimal.Big `json:"silver"`
	Gold     *decimal.Big `json:"gold"`
	Platinum *decimal.Big `json:"platinum"`
	Black    *decimal.Big `json:"black"`
}

func NewLaunchpadMakePaymentRequest(launchpad *Launchpad, userId uint64, amount *decimal.Big, userLineLevel string) *LaunchpadMakePaymentRequest {
	return &LaunchpadMakePaymentRequest{Launchpad: launchpad, UserId: userId, Amount: amount, UserLineLevel: userLineLevel}
}

func (launchpadLineLevels *LaunchpadLineLevels) GetLaunchpadLineLevelByUserLevel(userLineLevel string) (*decimal.Big, error) {
	switch userLineLevel {
	case "silver":
		return launchpadLineLevels.Silver, nil
	case "gold":
		return launchpadLineLevels.Gold, nil
	case "platinum":
		return launchpadLineLevels.Platinum, nil
	case "black":
		return launchpadLineLevels.Black, nil
	}

	return nil, errors.New("cannot get Launchpad line Level")
}

func NewLaunchpad(
	launchpadRequest *LaunchpadRequest,
	logoBase64 string,
	status LaunchpadStatus,
) *Launchpad {
	return &Launchpad{
		Logo:             logoBase64,
		Title:            launchpadRequest.Title,
		CoinSymbol:       launchpadRequest.CoinSymbol,
		ContributionsCap: &postgres.Decimal{V: launchpadRequest.ContributionsCap},
		LineLevels:       postgresDialects.Jsonb{RawMessage: launchpadRequest.LineLevels.RawMessage},
		Details:          launchpadRequest.Details,
		TokenDetails:     launchpadRequest.TokenDetails,
		ProjectInfo:      launchpadRequest.ProjectInfo,
		SocialMediaLinks: postgresDialects.Jsonb{RawMessage: launchpadRequest.SocialMediaLinks.RawMessage},
		StartDate:        launchpadRequest.StartDate,
		EndDate:          launchpadRequest.EndDate,
		Timezone:         launchpadRequest.Timezone,
		PresalePrice:     &postgres.Decimal{V: launchpadRequest.PresalePrice},
		Blockchain:       launchpadRequest.Blockchain,
		Status:           status,
		ShortInfo:        launchpadRequest.ShortInfo,
	}
}

func NewLaunchpadOrder(
	userId uint64,
	launchpadId uint64,
	refId string,
	tokenAmount *decimal.Big,
	spentAmount *decimal.Big,
	userLineLevel string,
) *LaunchpadOrder {
	return &LaunchpadOrder{
		UserID:        userId,
		LaunchpadId:   launchpadId,
		RefId:         refId,
		TokenAmount:   &postgres.Decimal{V: tokenAmount},
		SpentAmount:   &postgres.Decimal{V: spentAmount},
		UserLineLevel: userLineLevel,
	}
}

func (lineLevels *LaunchpadLineLevels) GetLineLevels(launchpad *Launchpad) error {
	err := json.Unmarshal(launchpad.LineLevels.RawMessage, &lineLevels)

	return err
}

func (launchpad *Launchpad) UpdateLaunchpad(
	launchpadRequest *LaunchpadRequest,
	logoBase64 string,
	status LaunchpadStatus) {
	launchpad.Logo = logoBase64
	launchpad.Title = launchpadRequest.Title
	launchpad.CoinSymbol = launchpadRequest.CoinSymbol
	launchpad.ContributionsCap = &postgres.Decimal{V: launchpadRequest.ContributionsCap}
	launchpad.LineLevels = postgresDialects.Jsonb{RawMessage: launchpadRequest.LineLevels.RawMessage}
	launchpad.Details = launchpadRequest.Details
	launchpad.TokenDetails = launchpadRequest.TokenDetails
	launchpad.ProjectInfo = launchpadRequest.ProjectInfo
	launchpad.SocialMediaLinks = postgresDialects.Jsonb{RawMessage: launchpadRequest.SocialMediaLinks.RawMessage}
	launchpad.StartDate = launchpadRequest.StartDate
	launchpad.EndDate = launchpadRequest.EndDate
	launchpad.Timezone = launchpadRequest.Timezone
	launchpad.PresalePrice.V = launchpadRequest.PresalePrice
	launchpad.Blockchain = launchpadRequest.Blockchain
	launchpad.Status = status
	launchpad.ShortInfo = launchpadRequest.ShortInfo
}

// MarshalJSON JSON encoding of a liability entry
func (launchpad Launchpad) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                 launchpad.ID,
		"logo":               launchpad.Logo,
		"title":              launchpad.Title,
		"coin_symbol":        launchpad.CoinSymbol,
		"contributions_cap":  utils.Fmt(launchpad.ContributionsCap.V),
		"line_levels":        launchpad.LineLevels,
		"details":            launchpad.Details,
		"blockchain":         launchpad.Blockchain,
		"token_details":      launchpad.TokenDetails,
		"project_info":       launchpad.ProjectInfo,
		"social_media_links": launchpad.SocialMediaLinks,
		"start_date":         launchpad.StartDate.Unix(),
		"end_date":           launchpad.EndDate.Unix(),
		"timezone":           launchpad.Timezone,
		"presale_price":      utils.Fmt(launchpad.PresalePrice.V),
		"status":             launchpad.Status,
		"short_info":         launchpad.ShortInfo,
	})
}
