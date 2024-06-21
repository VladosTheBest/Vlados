package auth_service

type PreAuthTokenStages string

const (
	PreAuthTokenStageUnApprovedIP    PreAuthTokenStages = "unapproved-ip"
	PreAuthTokenStageUnApprovedEmail PreAuthTokenStages = "unapproved-email"
	PreAuthTokenStageUnApprovedPhone PreAuthTokenStages = "unapproved-phone"
)

func (s PreAuthTokenStages) String() string {
	return string(s)
}

func IsValidPreAuthStage(stage string) bool {
	switch stage {
	case PreAuthTokenStageUnApprovedIP.String():
		return true
	case PreAuthTokenStageUnApprovedEmail.String():
		return true
	default:
		return false
	}
}
