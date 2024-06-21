package actions

import (
	"context"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/config"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service"
)

// Actions structure
type Actions struct {
	ctx               context.Context
	cfg               config.Config
	service           *service.Service
	jwtTokenSecret    string
	jwt2FATokenSecret string
}

// NewActions constructor
func NewActions(cfg config.Config, srv *service.Service, jwtTokenSecret, jwt2FATokenSecret string, ctx context.Context) *Actions {
	return &Actions{
		ctx:               ctx,
		cfg:               cfg,
		service:           srv,
		jwtTokenSecret:    jwtTokenSecret,
		jwt2FATokenSecret: jwt2FATokenSecret,
	}
}
