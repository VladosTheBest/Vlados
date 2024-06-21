package model

type UIType string

const (
	UIType_WebSimple UIType = "web-simple"
	UIType_WebBasic  UIType = "web-basic"
	UIType_WebATC    UIType = "web-atc"
	UIType_AppMobile UIType = "app-mobile"
	UIType_Api       UIType = "api"
)

func (ui UIType) String() string {
	return string(ui)
}

func (ui UIType) IsValidUIType() bool {
	switch ui {
	case UIType_WebSimple,
		UIType_WebBasic,
		UIType_WebATC,
		UIType_Api,
		UIType_AppMobile:
		return true
	default:
		return false
	}
}
