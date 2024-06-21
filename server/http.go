package server

import (
	"fmt"
	"net/http"
	"net/http/pprof"

	limit "github.com/bu/gin-access-limit"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/actions"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/logger"
)

func (srv *server) ListenToRequests() {
	log.Info().Str("worker", "http_listen_to_requests").Str("action", "start").Msg("HTTP Listen to requests - started")
	defer log.Info().Str("worker", "http_listen_to_requests").Str("action", "stop").Msg("1 => HTTP Listen to requests - stopped")

	a := srv.actions

	r := gin.New()

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowCredentials = true
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = []string{"Origin", "X-Requested-With", "Content-Length", "Content-Type", "Accept", "X-Api-Key", "Authorization"}
	corsConfig.AllowMethods = []string{"GET", "PUT", "POST", "DELETE", "PATCH", "OPTIONS"}

	r.Use(cors.New(corsConfig)) // Allow requests from anywhere
	r.Use(gin.Recovery())       // Recovery middleware recovers from any panics and writes a 500 if there was one.

	r.Use(logger.SetLogger())
	r.Use(a.CheckMaintenanceMode())

	// serve static file spec
	r.StaticFile("/swagger/rest.json", "./docs/swagger/rest.json")
	r.StaticFile("/swagger/admin.json", "./docs/swagger/admin.json")

	// setup all routes
	{
		r.GET("/ping", actions.Ping)

		r.GET("/depth/level1/:market", a.GetActiveMarket("market"), a.DepthLevel1)
		r.GET("/depth/level2/:market", a.GetActiveMarket("market"), a.DepthLevel2)
		r.GET("/public/v2/orderbook/:market", a.GetActiveMarket("market"), a.DepthLevel2)
		r.GET("/public/v2/cgk/orderbook", a.GetActiveMarket("market"), a.DepthLevel2GTK)
	}

	// handle authentication requests
	auth := r.Group("/auth")
	{
		// swagger:route POST /auth/login auth login
		// Login user
		//
		// Login a user based on credentials and generate a user token
		//
		//     Consumes:
		//     - multipart/form-data
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http, https
		//
		//     Responses:
		//       200: AuthLoginResp
		//       404: RequestErrorResp
		//       500: RequestErrorResp
		auth.POST("/login", a.CheckPartialToken(), a.CheckGeetest(), a.Auth(), a.HasApprovedIP(), a.LoginOTP(), a.GenerateJWTToken(), a.TrackActivity("login"), a.LoginResp())
		auth.POST("/login-phone", a.CheckPartialToken(), a.CheckGeetest(), a.AuthByPhone(), a.HasApprovedIP(), a.LoginOTP(), a.GenerateJWTToken(), a.TrackActivity("login"), a.LoginResp())
		auth.POST("/register", a.CheckGeetest(), a.Register)
		auth.POST("/register-phone", a.CheckGeetest(), a.RegisterByPhone)
		auth.POST("/email/confirm/:token", a.ConfirmUserEmail)
		auth.POST("/register-phone/sms", a.CheckPartialToken(), a.CheckGeetest(), a.SendSmsForRegistration)
		auth.PUT("/register-phone/sms", a.CheckPartialToken(), a.ConfirmSmsRegistration)
		auth.POST("/forgot-password", a.CheckGeetest(), a.RequestForgotPassword)
		auth.POST("/reset-password", a.CheckGeetest(), a.ResetPassword)

		auth.POST("/reset-2fa/recovery/send", a.CheckPrePartialToken(), a.TwoFactorRecoverySendCode)
		auth.POST("/reset-2fa/recovery/verify", a.CheckPrePartialToken(), a.TwoFactorRecoveryVerifyCode)
		auth.POST("/reset-2fa/recovery/send-double", a.CheckPrePartialToken(), a.TwoFactorRecoverySendCodes)
		auth.POST("/reset-2fa/recovery/verify-double", a.CheckPrePartialToken(), a.TwoFactorRecoveryVerifyCodes)

		auth.DELETE("/logout", a.RemoveToken, a.LogoutResp())
		auth.GET("/geetest/register", a.GeetestRegister)
		auth.GET("/geetest4/register", a.GeetestRegisterV4)
	}

	//security settings & profile page requests
	{
		r.GET("/profile", a.Restrict(true), a.HasPerm("profile.view"), a.GetProfile)
		r.GET("/profile/logs", a.Restrict(false), a.HasPerm("profile.view"), a.GetProfileLoginLogs)

		r.POST("/profile/activate", a.Restrict(true), a.HasPerm("profile.view"), a.SendActivationEmail)
		r.POST("/profile/update-avatar", a.Restrict(true), a.HasPerm("profile.view"), a.UpdateUserAvatar)
		r.POST("/profile/update-nickname", a.Restrict(true), a.HasPerm("profile.view"), a.UpdateUserNickname)
		r.POST("/profile/password", a.Restrict(false), a.HasPerm("profile.edit"), a.SetPassword)

		r.GET("/profile/details", a.Restrict(true), a.HasPerm("profile.view"), a.GetProfileDetails)
		r.POST("/profile/details", a.Restrict(false), a.HasPerm("profile.edit"), a.SetProfileDetails)

		r.GET("/profile/lock-amount", a.Restrict(false), a.HasPerm("profile.view"), a.GetLockAmount)

		r.GET("/profile/settings", a.Restrict(true), a.HasPerm("profile.view"), a.GetProfileSettings)
		r.POST("/profile/settings", a.Restrict(false), a.HasPerm("profile.edit"), a.SetProfileSettings)
		r.GET("/profile/push-notification-settings", a.Restrict(true), a.HasPerm("profile.view"), a.GetPushNotificationSettings)
		r.POST("/profile/push-notification-settings", a.Restrict(true), a.HasPerm("profile.view"), a.SetPushNotificationSettings)
		r.POST("/profile/favourites", a.Restrict(false), a.SetMarketPairFavorite)
		r.GET("/profile/favourites", a.Restrict(false), a.GetMarketPairFavorites)

		r.POST("/profile/check-approve-ip", a.CheckPreAuthToken(), a.CheckApprovedIP)
		r.POST("/profile/check-approve-email", a.CheckPreAuthToken(), a.CheckApprovedEmail)
	}

	r.GET("frontend-features/:feature", a.PermissionsForFrontendDebug("admin.debug.frontend-features"), a.AdminDebugFeatures)
	r.POST("frontend-features", a.PermissionsForFrontendDebugBulk("admin.debug.frontend-features"))

	r.GET("system-info/maintenance", a.SystemInfoMaintenance)

	r.GET("country-code-by-ip", a.GetCountryCodeByIP)

	//support
	support := r.Group("/support")
	{
		support.POST("/:type", a.SendSupportRequestEmail)
	}

	//referrals
	referrals := r.Group("/referrals", a.Restrict(false))
	{
		referrals.GET("/", a.HasPerm("profile.view"), a.GetReferrals)
		referrals.GET("/earnings/total", a.HasPerm("profile.view"), a.GetReferralEarningsTotalByUser)
		//referrals.GET("/topinviters", a.HasPerm("profile.view"), a.GetTopInviters)
	}

	// API keys requests
	{
		r.GET("/apikeys", a.Restrict(false), a.HasPerm("profile.view"), a.GetAPIKeys)
		r.POST("/apikeys", a.Restrict(true), a.HasPerm("profile.edit"), a.OTP(false), a.GenerateAPIKey)
		r.DELETE("/apikeys", a.Restrict(false), a.HasPerm("profile.edit"), a.OTP(), a.RemoveAPIKey)
		r.GET("/apikeys/roles", a.Restrict(false), a.GetAPIRoles) // @todo this should be moved to roles instead
	}

	apiKeysV2 := r.Group("/apikeys-v2", a.Restrict(false))
	{
		apiKeysV2.GET("", a.Restrict(false), a.HasPerm("profile.view"), a.GetAPIKeysV2)
		apiKeysV2.POST("", a.Restrict(true), a.HasPerm("profile.edit"), a.OTP(false), a.GenerateAPIKeyV2)
		apiKeysV2.DELETE("", a.Restrict(false), a.HasPerm("profile.edit"), a.OTP(), a.RemoveAPIKeyV2)
		apiKeysV2.POST("/allowed-ip", a.Restrict(true), a.HasPerm("profile.edit"), a.OTP(false), a.AddAPIKeyV2AllowedIp)
		apiKeysV2.DELETE("/allowed-ip", a.Restrict(false), a.HasPerm("profile.edit"), a.OTP(), a.RemoveAPIKeyV2AllowedIp)
		apiKeysV2.GET("/permissions", a.HasPerm("profile.view"), a.GetApiKeysPermissions)

	}
	//security keys
	//generate a secret key if one is not set, otherwise fail
	{
		// google auth
		r.POST("/security/2fa/google", a.Restrict(false), a.HasPerm("profile.view"), a.CheckOrGenerateGoogleSecretKey)
		r.PUT("/security/2fa/google/on", a.Restrict(true), a.HasPerm("profile.edit"), a.ValidateOTP("google"), a.EnableGoogleAuth)
		r.PUT("/security/2fa/google/off", a.Restrict(true), a.HasPerm("profile.edit"), a.ValidateOTP("google"), a.DisableGoogleAuth)

		// trade password
		r.POST("/security/trade-password/on", a.Restrict(false), a.HasPerm("profile.edit"), a.EnableTradePassword)
		r.PUT("/security/trade-password/off", a.Restrict(false), a.HasPerm("profile.edit"), a.DisableTradePassword)

		// detect IP
		r.POST("/security/detect-ip/on", a.Restrict(false), a.HasPerm("profile.edit"), a.EnableDetectIP)
		r.PUT("/security/detect-ip/off", a.Restrict(false), a.HasPerm("profile.edit"), a.DisableDetectIP)

		// anti phishing code
		r.POST("/security/anti-phishing/on", a.Restrict(false), a.HasPerm("profile.edit"), a.EnableAntiPhishingCode)
		r.PUT("/security/anti-phishing/off", a.Restrict(false), a.HasPerm("profile.edit"), a.DisableAntiPhishingCode)

		// sms auth
		r.POST("/security/2fa/sms", a.Restrict(true), a.HasPerm("profile.edit"), a.SmsAuthInitBindPhone)
		r.PUT("/security/2fa/sms", a.Restrict(false), a.HasPerm("profile.edit"), a.SmsAuthBindPhone)
		r.DELETE("/security/2fa/sms", a.Restrict(false), a.HasPerm("profile.edit"), a.ValidateOTP("sms"), a.SmsAuthUnbindPhone)
		r.POST("/security/2fa/sms/send", a.Restrict(false), a.HasPerm("profile.edit"), a.SendSmsWithCode)

	}

	chains := r.Group("/chains", a.Restrict(false))
	{
		chains.GET("", a.HasPerm("chain.view"), a.GetChains)
		chains.POST("", a.HasPerm("chain.add"), a.TrackAdminActivity(), a.AddChain)
		chains.PUT("/:chain_symbol", a.HasPerm("chain.edit"), a.TrackAdminActivity(), a.UpdateChain)
		chains.DELETE("/:chain_symbol", a.HasPerm("chain.remove"), a.TrackAdminActivity(), a.DeleteChain)
		chains.GET("/:chain_symbol", a.HasPerm("chain.view"), a.GetChain)
	}
	coins := r.Group("/coins")
	{
		coins.GET("", a.GetCoins)
		coins.GET("/:coin_symbol", a.GetCoin)
		coins.POST("", a.Restrict(false), a.HasPerm("coin.add"), a.TrackAdminActivity(), a.AddCoin)
		coins.PUT("/:coin_symbol", a.Restrict(false), a.HasPerm("coin.edit"), a.TrackAdminActivity(), a.UpdateCoin)
		coins.DELETE("/:coin_symbol", a.Restrict(false), a.HasPerm("coin.remove"), a.TrackAdminActivity(), a.DeleteCoin)
		coins.GET("/exchange_rate", a.GetCoinRate)
	}

	launchpads := r.Group("/launchpad", a.Restrict(true))
	{
		launchpads.GET("", a.GetLaunchpadList)
		launchpad := launchpads.Group("/:launchpad_id", a.Restrict(true))
		{
			launchpad.GET("", a.GetLaunchpad)
			launchpad.POST("/buy", a.LaunchpadMakePayment)
		}
	}

	bots := r.Group("/bots", a.Restrict(true))
	{
		bots.GET("/", a.BotsGetList)
		bots.GET("/settings", a.BotsSettings)
		bots.POST("/create", a.BotCreateSeparate)
		bots.POST("/liquidate/:bot_id", a.BotLoadingMiddleware(true), a.BotSeparateLiquidate)
		bots.POST("/", a.BotChangeSettings)
		bots.POST("/status", a.BotChangeStatus)
		bots.GET("/pnl", a.BotsGetPnlForUser)
		bots.GET("/analytics/all/:status", a.BotLoadingMiddleware(true), a.BotsGetAllAnalytics)
		botsAnalytics := bots.Group("/analytics/:bot_id", a.BotLoadingMiddleware(true))
		{
			botsAnalytics.GET("/version/:version", a.BotGetAnalytics)
			botsAnalytics.GET("/numbers", a.BotGetAnalyticsNumbers)
		}
	}

	spotSubAcc := r.Group("/accounts", a.Restrict(true))
	{
		spotSubAcc.PUT("", a.EditSubAccount)
		spotSubAcc.GET("", a.GetUserSubAccounts)
		spotSubAcc.GET("/default", a.GetDefaultUserSubAccounts)
		spotSubAcc.POST("", a.CreateSubAccount)
		spotSubAcc.POST("/transfer", a.TransferSubAccounts)
	}

	bonusAccountInfo := r.Group("/bonus-account")
	{
		bonusAccountInfo.GET("/settings", a.GetBonusAccountSettings)
		bonusAccountInfoLanding := bonusAccountInfo.Group("/landing")
		{
			bonusAccountInfoLanding.GET("/settings", a.GetBonusAccountLandingSettings)
			bonusAccountInfoLanding.GET("/chart", a.GetBonusAccountLandingChart)
		}
	}

	stakingInfo := r.Group("/staking")
	{
		stakingInfo.GET("/settings", a.GetStakingSettings)
		stakingInfo.POST("/create", a.Restrict(true), a.HasPerm("wallet.view"), a.CreateStaking)
		stakingInfo.GET("/list", a.Restrict(true), a.HasPerm("wallet.view"), a.GetStakingList)
		stakingInfo.GET("/earnings/:staking_id", a.Restrict(true), a.HasPerm("wallet.view"), a.GetStakingEarnings)
	}

	// wallets API
	wallets := r.Group("/wallets")
	{
		bonusAccountWallets := wallets.Group("/bonus-account")
		{
			bonusAccountWallets.POST("/deposit", a.Restrict(true), a.HasPerm("withdraw.request") /*a.OTP(),*/, a.BonusAccountDeposit)
			bonusAccountWallets.GET("/contracts", a.Restrict(true), a.HasPerm("wallet.view"), a.GetBonusAccountContractsList)
			bonusAccountWallets.GET("/settings", a.Restrict(true), a.HasPerm("wallet.view"), a.GetBonusAccountSettings)
			bonusAccountWallets.GET("/contracts/history", a.Restrict(true), a.HasPerm("wallet.view"), a.GetBonusAccountContractsHistoryList)
			bonusAccountWallets.GET("/contracts/history/export/:contract_id", a.Restrict(true), a.HasPerm("wallet.view"), a.ExportBonusAccountContractsHistoryList)
		}

		pnlWallets := wallets.Group("/pnl")
		{
			pnlWallets.GET("/24h", a.Restrict(true), a.HasPerm("wallet.view"), a.GetPnl24h)
			pnlWallets.GET("/weekly", a.Restrict(true), a.HasPerm("wallet.view"), a.GetPnlWeek)
		}
		wallets.GET("/balances", a.Restrict(false), a.HasPerm("wallet.view"), a.MetricsMiddleware(), a.WalletGetBalances)
		wallets.GET("/deposits", a.Restrict(false), a.HasPerm("wallet.view"), a.WalletGetDeposits)
		wallets.GET("/deposit/export", a.Restrict(false), a.HasPerm("wallet.view"), a.ExportWalletDeposit)

		walletsAddresses := wallets.Group("/addresses", a.Restrict(false), a.HasPerm("wallet.view"))
		{
			walletsAddresses.GET("/:symbol", a.WalletGetDepositAddress)
			walletsAddresses.POST("/:symbol", a.CreateAddressForUser)
		}

		wallets.POST("/withdraw/:symbol", a.Restrict(true), a.HasPerm("withdraw.request"), a.OTP(), a.Withdraw)
		wallets.DELETE("/withdraw/:id", a.Restrict(true), a.HasPerm("withdraw.request"), a.CancelWithdrawRequest)
		wallets.GET("/withdrawals", a.Restrict(false), a.HasPerm("wallet.view"), a.WalletGetWithdrawals)
		wallets.GET("/withdraw-total", a.Restrict(false), a.HasPerm("wallet.view"), a.Get24HWithdrawals)
		wallets.POST("/withdraw-total", a.Restrict(false), a.HasPerm("wallet.view"), a.Get24HWithdrawalsOld)
		wallets.GET("/withdraw-limits", a.GetWithdrawLimits)
		wallets.GET("/withdraw-limits/user", a.Restrict(true), a.GetWithdrawLimitsByUser)
		wallets.GET("/withdraw/fiat/fees", a.WithdrawFiatFees)
		wallets.POST("/address", a.Restrict(true), a.SaveUserAddress)
		wallets.GET("/addresses", a.Restrict(true), a.UserAddresses)
		wallets.DELETE("/address", a.Restrict(true), a.DeleteUserAddress)
	}

	card := r.Group("/card", a.Restrict(true))
	{
		card.GET("/list-available-cards", a.ListAvailableCardTypes)
		card.GET("/user-current-balance", a.GetUserCurrentCardPaymentBalance)

		card.POST("/deposit", a.DepositToCardAccount)
		card.POST("/withdraw", a.WithdrawFromCardAccount)
		card.POST("/add-consumer", a.AddConsumer)
		card.POST("/activate-card", a.ActivateCard)
	}
	// api for maintaining record for visa card wait list
	r.POST("/card/join-waitlist", a.AddToCardWaitList)

	// Actions
	r.PUT("/actions/:action_id/approve/:key", a.GetActionByUUID(), a.ApproveAction)

	//get user's orders / trades / fees
	users := r.Group("/users", a.Restrict(false))
	{
		users.GET("/orders", a.HasPerm("order.view"), a.GetUserOrdersWithTrades)
		users.GET("/orders/export", a.HasPerm("order.view"), a.ExportUserOrders)
		users.DELETE("/orders/cancelAll", a.HasPerm("order.cancel"), a.CancelUserOrdersForAllMarketsByUser)
		users.DELETE("/orders/cancelAll/:market", a.HasPerm("order.cancel"), a.GetActiveMarket("market"), a.CancelUserOrdersByMarketsByUser)
		//get user's transactions data
		users.GET("/transactions", a.HasPerm("transaction.view"), a.GetUserTransactions)
		users.GET("/transactions/export", a.HasPerm("transaction.view"), a.ExportUserTransactions)
		users.GET("/trades", a.HasPerm("order.view"), a.GetUserTradesOrFees)
		users.GET("/trades/all", a.HasPerm("order.view"), a.GetUserTradesWithOrders)
		users.GET("/fees", a.HasPerm("order.view"), a.GetUserTradesOrFees)
		users.GET("/fees/export", a.HasPerm("order.view"), a.ExportUserFees)
		// distributionf
		users.GET("/distributions", a.HasPerm("distribution.view"), a.GetUserDistributions)
		users.GET("/distributions/export", a.HasPerm("distribution.view"), a.GetUserDistributionsExport)

		users.GET("/manual-distributions", a.HasPerm("distribution.view"), a.GetUserManualDistributions)
		users.GET("/manual-distributions/export", a.HasPerm("distribution.view"), a.GetUserManualDistributionsExport)
		users.GET("/manual-distributions/get-bonus", a.HasPerm("distribution.view"), a.GetManualDistributionGetBonus)
		users.GET("/manual-distributions/info", a.HasPerm("distribution.view"), a.GetManualDistributionInfo)

		usersLayout := users.Group("/layout")
		{
			usersLayout.GET("", a.GetLayouts)
			usersLayout.POST("/save", a.SaveLayout)
			usersLayout.PATCH("/update", a.UpdateLayout)
			usersLayout.DELETE("/delete", a.DeleteLayout)
			usersLayout.PUT("/setActive", a.SetActiveLayout)
			usersLayout.PUT("/sort", a.SortLayouts)
		}
	}

	// orders API
	orders := r.Group("/orders", a.Restrict(false))
	{
		orders.GET("/:market", a.HasPerm("order.view"), a.ListOrders)
		orders.GET("/:market/open", a.HasPerm("order.view"), a.ListOpenOrders)
		orders.GET("/:market/closed", a.HasPerm("order.view"), a.ListClosedOrders)
		orders.POST("/:market/:side", a.HasPerm("order.add"), a.RestrictByApiKeyPermissions("trading_allowed"), a.GetActiveMarket("market"), a.ValidateTradePassword(), a.CreateOrder)
		orders.POST("/:market/:side/:order_id", a.HasPerm("order.add"), a.RestrictByApiKeyPermissions("trading_allowed"), a.GetActiveMarket("market"), a.GetOrder("user_id"), a.ValidateTradePassword(), a.CreateOrder)
		orders.DELETE("/:market/:order_id", a.HasPerm("order.cancel"), a.RestrictByApiKeyPermissions("trading_allowed"), a.GetActiveMarket("market"), a.GetOrder("user_id"), a.CancelOrder)
	}
	bulkOrders := r.Group("/bulk/orders", a.Restrict(false))
	{
		bulkOrders.POST("/:market", a.HasPerm("order.add"), a.RestrictByApiKeyPermissions("trading_allowed"), a.GetActiveMarket("market"), a.CreateOrderBulk)
		bulkOrders.DELETE("/:market", a.HasPerm("order.cancel"), a.RestrictByApiKeyPermissions("trading_allowed"), a.GetActiveMarket("market"), a.CancelOrderBulk)
	}

	// OTC API
	otcOrders := r.Group("otc", a.Restrict(false))
	{
		otcOrders.POST("/quote", a.HasPerm("order.add"), a.RestrictByApiKeyPermissions("trading_allowed"), a.ValidateTradePassword(), a.CreateOTCOrderQuote)
	}

	// internal API
	internal := r.Group("/internal", a.Restrict(true))
	{
		// orders
		internalOrders := internal.Group("/orders")
		{
			internalOrders.POST("/:market/:side", a.HasPerm("internal.order.add"), a.GetActiveMarket("market"), a.ValidateTradePassword(), a.CreateOrderInternal)
			internalOrders.DELETE("/:market/:order_id", a.HasPerm("internal.order.cancel"), a.GetActiveMarket("market"), a.GetOrder("order_id"), a.CancelOrder)
		}

		// bots
		internalBots := internal.Group("/bots", a.Restrict(true), a.HasPerm("internal.bots-system.master-key"))
		{
			internalBots.GET("/", a.BotsGetListAll)

			internalBot := internalBots.Group("/:bot_id", a.BotLoadingMiddleware(false))
			{
				internalBot.POST("/notify", a.BotNotify)
				internalBot.GET("/wallet", a.BotGetWalletBalances)
				internalBot.POST("/wallet/rebalance", a.BotWalletReBalance)

				internalBotOrders := internalBot.Group("/order/:market", a.GetActiveMarket("market"))
				{
					internalBotOrders.POST("/:side", a.BotCreateOrder)
					internalBotOrders.DELETE("/:order_id", a.GetOrder("order_id"), a.CancelOrder)
				}

				internalBotOrdersHistory := internalBot.Group("/orders")
				{
					internalBotOrdersHistory.GET("/:market/open", a.BotGetListOpenOrders)
					internalBotOrdersHistory.GET("/:market/closed", a.BotGetListClosedOrders)
				}

				internalBotsAnalytics := internalBot.Group("/analytics")
				{
					internalBotsAnalytics.POST("/", a.BotAddStatistics)
					internalBotsAnalytics.GET("/:version", a.BotGetAnalytics)
				}
			}
		}
	}

	//markets API
	markets := r.Group("/markets")
	{
		markets.GET("", a.GetMarkets)
		markets.POST("", a.Restrict(false), a.HasPerm("market.add"), a.TrackAdminActivity(), a.AddMarket)
		markets.GET("/:market_id", a.Restrict(false), a.HasPerm("market.view"), a.GetMarket)
		markets.PUT("/:market_id", a.Restrict(false), a.HasPerm("market.edit"), a.TrackAdminActivity(), a.UpdateMarket)
		markets.DELETE("/:market_id", a.Restrict(false), a.HasPerm("market.remove"), a.TrackAdminActivity(), a.DeleteMarket)
		// @todo add swagger docs
		markets.GET("/:market_id/history", a.GetActiveMarket("market_id"), a.GetTradeHistory)
		markets.PUT("/highlight/:market_id/:switcher", a.Restrict(false), a.HasPerm("market.edit"), a.TrackAdminActivity(), a.SetMarketHighlight)
	}

	r.GET("/v2/cgk/markets", a.GetMarketsCGK)

	//events API
	burnEvents := r.Group("/burn-events")
	{
		burnEvents.GET("", a.GetAllBurnEvents)
	}

	// get data about the user trades
	trades := r.Group("/trades")
	{
		trades.GET("/:market_id", a.Restrict(false), a.GetActiveMarket("market_id"), a.GetTrades)
	}

	// kyc API
	kyc := r.Group("/kyc")
	{
		kyc.POST("/customer-registration", a.Restrict(true), a.KycCustomerRegistrationV3)
		kyc.POST("/document-verification", a.Restrict(true), a.KycDocumentVerificationV3)
		kyc.POST("/payment-document-verification", a.Restrict(true), a.KycPaymentDocumentVerificationV3)
		kyc.POST("/callback", a.KycCallbackV3)
		kyc.GET("/status", a.Restrict(true), a.KycGetStatusForAnUserV3)
	}

	//r.POST("/kyc/customer-registration", a.Restrict(true), a.KycCustomerRegistration)
	//r.POST("/kyc/document-verification", a.Restrict(true), a.KycDocumentVerification)
	//r.POST("/kyc/payment-document-verification", a.Restrict(true), a.KycPaymentDocumentVerification)
	//r.POST("/kyc/callback", a.KycCallback)
	//r.GET("/kyc/status", a.Restrict(true), a.KycGetStatusForAnUser)

	// kyb API
	kyb := r.Group("/kyb")
	{
		kyb.POST("/basic-information", a.Restrict(true), a.KYBBasicInformation)
		// Step 1: Account Information
		kyb.POST("/account-information", a.Restrict(true), a.KYBAccountInformation)
		// Step 2: Registration Address
		kyb.POST("/registration-address", a.Restrict(true), a.KYBRegistrationAddress)
		// Step 3: Operational Address
		kyb.POST("/operational-address", a.Restrict(true), a.KYBOperationalAddress)
		// Step 4: Source of Funds
		kyb.POST("/source-of-funds", a.Restrict(true), a.KYBSourceOfFunds)
		// Step 5: Additional Information
		kyb.POST("/additional-information", a.Restrict(true), a.KYBAdditionalInformation)

		kyb.POST("/basic-verification", a.Restrict(true), a.KYBDocumentsRegistration)
		kyb.GET("/user-kyb", a.Restrict(true), a.GetKYB)
		kyb.POST("/verify-directors", a.Restrict(true), a.KYBVerifyDirectors)
		kyb.POST("/jumio-callback", a.KYBCallbackFromJumio)
		kyb.POST("/callback", a.KYBCallback)
	}

	r.GET("/distributed-bonus", a.GetDistributedBonus)
	r.GET("/distributed-percent", a.GetLastMarketMakerPercentValue)
	r.GET("/volume-distributed-percent", a.GetMarketMakerVolumePercentValue)

	//endpoint for landing page 'paramountdax.com'
	r.GET("/referral-earnings-all", a.GetReferralEarningsTotalAll)

	// adv cash
	advCash := r.Group("/advCash", a.Restrict(true))
	{
		advCash.POST("/deposit/request/:asset", a.AdvCashDepositRequest)
	}

	// clearJunction
	clearJunction := r.Group("/cj")
	{
		clearJunction.GET("/deposit/request", a.Restrict(true), a.ClearJunctionDepositRequest)
		clearJunction.POST("/deposit/request/sepa", a.Restrict(true), a.ClearJunctionDepositRequest)
		clearJunction.GET("/withdraw/settings", a.Restrict(true), a.ClearJunctionWithdrawalSettings)
	}

	notifications := r.Group("/notifications", a.Restrict(true))
	{
		notifications.GET("", a.GetNotifications)
		notifications.DELETE("/:notificationID", a.DeleteNotification)
		notifications.DELETE("/range", a.DeleteNotificationsInRange)
		notifications.POST("/status", a.ChangeNotificationsStatus)
		notifications.GET("/unread/total", a.GetUnreadNotifications)

		notifications.POST("/pushToken", a.PushToken)
		notifications.DELETE("/pushToken", a.DeletePushToken)
	}

	r.GET("/fee", a.Restrict(true), a.GetUserFeesForUser)

	r.GET("/announcements", a.GetAnnouncements)

	// admin functionality
	admin := r.Group("/admin", a.Restrict(false), a.HasPerm("admin.view"), a.TrackAdminActivity())
	{
		admin.GET("/activity/users", a.HasPerm("user.view"), a.GetActiveUsers)
		admin.GET("/statistics/users", a.HasPerm("user.view"), a.GetInfoAboutUsers)
		admin.GET("/statistics/trades", a.HasPerm("order.view.all"), a.GetInfoAboutTrades)
		admin.GET("/statistics/general", a.HasPerm("order.view.all"), a.GetUsersAndOrdersCount)
		admin.GET("/statistics/coins", a.HasPerm("coin.view"), a.GetCoinStatistics)
		admin.GET("/statistics/fees", a.HasPerm("order.view.all"), a.GetFeesAdmin)
		admin.GET("/statistics/bots", a.GetTotalBots)
		admin.GET("/statistics/bots/:user_id", a.GetTotalBotsByUserId)
		admin.GET("/users/deposits/:user_id", a.HasPerm("order.view.all"), a.GetAllUserDeposits)
		admin.GET("/users", a.HasPerm("user.view"), a.GetUsers)
		admin.GET("/downloads-emails", a.HasPerm("user.view"), a.DownloadUsersEmails)
		admin.GET("/total-users-line-levels", a.HasPerm("user.view"), a.GetTotalUserLineLevels)

		admin.GET("/users/:user_id/loginlog", a.HasPerm("user.view"), a.GetUserLoginLogByID)
		admin.GET("/users/:user_id/orders", a.HasPerm("order.view.all"), a.GetUserOrdersByID)
		admin.DELETE("/users/:user_id/orders", a.HasPerm("order.cancel.all"), a.CancelUserOrdersForAllMarkets)
		admin.DELETE("/users/:user_id/orders/:market_id/:status", a.HasPerm("order.cancel.all"), a.CancelUserOrders)
		admin.GET("/users/:user_id/trades", a.HasPerm("order.view.all"), a.GetUserTradesByID)
		admin.GET("/users/:user_id/trades/export", a.HasPerm("order.view.all"), a.ExportUserTradesByID)
		admin.GET("/users/:user_id/trades/fees", a.HasPerm("order.view.all"), a.GetUserFeesAdmin)
		admin.GET("/users/:user_id/wallet", a.HasPerm("transaction.view.all"), a.GetUserFromParam("user_id"), a.GetUserWalletBalances)
		admin.GET("/users/:user_id/wallet/addresses", a.HasPerm("user.view"), a.GetUserFromParam("user_id"), a.WalletGetAllDepositAddresses)
		admin.GET("/users/:user_id/distributions", a.HasPerm("distribution.get.all"), a.GetUserDistributionsByID)
		admin.GET("/users/:user_id/withdrawals", a.HasPerm("withdraw.view.all"), a.GetUserWithdrawsByID)
		admin.GET("/users/:user_id/deposits", a.HasPerm("transaction.view.all"), a.GetUserTransactionsById)
		admin.GET("/users/:user_id", a.HasPerm("user.view"), a.GetUser)
		admin.GET("/users/:user_id/fees", a.HasPerm("user.view"), a.GetUserFees)
		admin.GET("/users/:user_id/bonus-contracts", a.HasPerm("user.view"), a.GetAdminBonusAccountContractsList)
		admin.GET("/user-balances", a.HasPerm("user.view"), a.ExportUserBalances)

		// generate missing deposit addresses for user
		admin.POST("/users/:user_id/wallet", a.HasPerm("user.edit"), a.GenerateMissingAddressesForUser)

		// edit user data
		admin.PUT("/users/:user_id/status", a.GetUserFromParam("user_id"), a.HasPerm("user.edit"), a.ChangeUserStatus)
		admin.PUT("/users/:user_id/settings", a.HasPerm("user.edit"), a.UpdateSettings)
		admin.PUT("/users/:user_id/fees", a.HasPerm("user.edit"), a.UpdateUserFees)
		admin.DELETE("/users/:user_id/withdraw/:id", a.HasPerm("withdraw.request"), a.CancelUserWithdraw)
		// disable google auth
		admin.DELETE("/users/:user_id/settings/2fa/google/off", a.HasPerm("user.edit"), a.AdminDisableGoogleAuth)
		// disable sms auth
		admin.DELETE("/users/:user_id/settings/2fa/sms/off", a.HasPerm("profile.edit"), a.AdminDisableSmsAuth)
		// disable trade password
		admin.DELETE("/users/:user_id/settings/trade-password/off", a.HasPerm("user.edit"), a.AdminDisableTradePassword)
		// detect IP
		admin.POST("/users/:user_id/settings/detect-ip/on", a.HasPerm("user.edit"), a.AdminEnableDetectIP)
		admin.PUT("/users/:user_id/settings/detect-ip/off", a.HasPerm("user.edit"), a.AdminDisableDetectIP)
		// disable anti phishing code
		admin.DELETE("/users/:user_id/settings/anti-phishing/off", a.HasPerm("user.edit"), a.AdminDisableAntiPhishingCode)

		admin.PUT("/users/:user_id/password", a.GetUserFromParam("user_id"), a.HasPerm("user.edit"), a.UpdateUserPassword)
		admin.PUT("/users/:user_id", a.GetUserFromParam("user_id"), a.HasPerm("user.edit"), a.UpdateUser)

		admin.GET("users/:user_id/bots", a.AdminBotsGetList)
		admin.GET("users/:user_id/bots/analytics/:bot_id/version/:version", a.AdminBotGetAnalytics)
		admin.GET("users/:user_id/bots/analytics/:bot_id/numbers", a.AdminBotGetAnalyticsNumbers)

		admin.GET("/withdrawals", a.HasPerm("transaction.view.all"), a.GetWithdrawals)
		admin.GET("/manual-withdrawal", a.HasPerm("transaction.view.all"), a.GetManualWithdrawals)
		admin.POST("/manual-withdrawals/create", a.Restrict(true), a.HasPerm("transaction.view.all"), a.AdminCreateManualWithdrawal)
		admin.PUT("/manual-withdrawals/:manual_transaction_id/confirm", a.Restrict(true), a.HasPerm("transaction.view.all"), a.AdminConfirmManualWithdrawal)

		admin.GET("/deposits", a.HasPerm("transaction.view.all"), a.GetDeposits)
		admin.GET("/manual-deposits/confirming-users", a.HasPerm("transaction.view.all"), a.GetManualDepositConfirmingUsers)
		admin.GET("/manual-deposits", a.HasPerm("transaction.view.all"), a.GetManualDeposits)
		admin.POST("/manual-deposits/create", a.Restrict(true), a.HasPerm("transaction.view.all"), a.AdminCreateManualDeposit)
		admin.PUT("/manual-deposits/:manual_transaction_id/confirm", a.Restrict(true), a.HasPerm("transaction.view.all"), a.AdminConfirmManualTransaction)
		admin.PUT("/deposits/:deposit_id/confirm", a.HasPerm("transaction.view.all"), a.PreloadTx("deposit_id"), a.AdminConfirmDeposit)
		admin.GET("/operations", a.HasPerm("transaction.view.all"), a.GetOperations)
		admin.GET("/markets", a.HasPerm("market.view"), a.GetMarketsDetailed)
		admin.GET("/orders", a.HasPerm("order.view.all"), a.ListAllOrders)
		admin.GET("/orders/:market", a.HasPerm("transaction.view.all"), a.ListMarketOrders)
		admin.DELETE("/orders/:market", a.HasPerm("order.cancel.all"), a.GetActiveMarket("market"), a.CancelMarketOrders)
		admin.DELETE("/orders/:market/:order_id", a.HasPerm("order.cancel.all"), a.GetActiveMarket("market"), a.GetOrder("order_id"), a.CancelOrder)

		// revert frozen order with pending status
		admin.DELETE("/tools/market/:market/order/:order_id/revert", a.HasPerm("order.cancel.all"), a.GetActiveMarket("market"), a.GetOrder("order_id"), a.ToolRevertOrder)

		admin.GET("/profile", a.HasPerm("profile.view"), a.GetAdminProfile)
		admin.PUT("/profile", a.HasPerm("profile.edit"), a.UpdateAdminProfile)

		admin.GET("/permissions", a.HasPerm("role.view"), a.GetPermissions)
		admin.GET("/permissions/user", a.HasPerm("role.view"), a.GetUserPermissions)
		admin.GET("/roles", a.HasPerm("role.view"), a.GetUserRoles)
		admin.POST("/roles", a.HasPerm("role.add"), a.AddUserRole)
		admin.GET("/roles/:role_alias", a.HasPerm("role.view"), a.GetUserRole)
		admin.PUT("/roles/:role_alias", a.HasPerm("role.edit"), a.UpdateUserRole)
		admin.DELETE("/roles/:role_alias", a.HasPerm("role.remove"), a.RemoveUserRole)

		admin.GET("/distribution", a.HasPerm("distribution.get.all"), a.GetDistributionEvents)
		admin.GET("/distribution/:distribution_id", a.HasPerm("distribution.get"), a.GetDistributionOrders)
		admin.GET("/distribution/:distribution_id/entries", a.HasPerm("distribution.get"), a.GetDistributionEntries)

		admin.GET("/manual-distribution", a.HasPerm("distribution.get.all"), a.GetAdminDistributions)
		admin.GET("/manual-distribution/:distribution_id", a.HasPerm("distribution.get"), a.GetAdminDistribution)
		admin.POST("/manual-distribution/:distribution_id/complete", a.HasPerm("distribution.get"), a.CompleteAdminDistribution)
		admin.GET("/manual-distribution/:distribution_id/funds", a.HasPerm("distribution.get"), a.GetAdminDistributionFunds)
		admin.POST("/manual-distribution/:distribution_id/funds", a.HasPerm("distribution.get"), a.UpdateAdminDistributionFunds)
		admin.GET("/manual-distribution/:distribution_id/balances", a.HasPerm("distribution.get"), a.GetAdminDistributionBalances)

		admin.GET("/referrals", a.HasPerm("referral.get.all"), a.GetReferralsWithEarnings)

		admin.POST("/burn-events", a.Restrict(false), a.HasPerm("burn-events.add"), a.AddBurnEvent)
		admin.DELETE("/burn-events/:event_id", a.Restrict(false), a.HasPerm("burn-events.remove"), a.RemoveBurnEvent)

		admin.GET("/total-distributed-prdx-line/:distribution_id", a.GetTotalDistributedOfPRDXLine)
		admin.GET("/total-prdx-line", a.GetTotalOfPRDXLine)
		//Announcements
		{
			admin.POST("/support-notification", a.CreateNotifications)
			admin.POST("/announcements", a.CreateAnnouncements)
			admin.GET("/announcements", a.GetAdminAnnouncements)
			admin.GET("/announcement/:id", a.GetAdminAnnouncementByID)
			admin.DELETE("/announcement/:id", a.DeleteAdminAnnouncementByID)
			admin.POST("/announcements/status", a.ChangeAnnouncementsStatus)
			admin.POST("/announcements/update-settings", a.UpdateAnnouncementsSettings)
			admin.GET("/announcements/settings", a.GetAnnouncementsSettings)

			r.GET("/announcements/topics", a.GetAnnouncementsSettings)
			r.GET("/announcements/by-topics", a.GetAnnouncementsByTopic)
			r.GET("/announcements/list/by-topics", a.GetAnnouncementByTopicAndID)
		}

		admin.GET("/launchpad", a.GetAdminLaunchpadList)
		admin.POST("/launchpad", a.CreateLaunchpad)
		admin.GET("/launchpad/:launchpad_id", a.GetAdminLaunchpad)
		admin.PUT("/launchpad/:launchpad_id", a.UpdateLaunchpad)
		admin.POST("/launchpad/:launchpad_id/end_presale", a.LaunchpadEndPresale)

		admin.POST("/withdraw/fee", a.WithdrawFeeAdmin)

		admin.POST("/withdraw-settings", a.UpdateFeatures)
		admin.GET("/withdraw-limits", a.GetWithdrawLimits)
		admin.PUT("/withdraw-limits/user/:user_id", a.UpdateWithdrawLimitsByUser)
		admin.GET("/withdraw-limits/user/:user_id", a.GetWithdrawLimitsByUser)

		admin.PUT("/block/all/:action_type/:switcher", a.Block)
		admin.PUT("/block/coin/:coin_symbol/:action_type/:switcher", a.BlockByCoin)
		admin.PUT("/block/user/:user_id/:action_type/:switcher", a.BlockByUser)

		admin.GET("/block/user/:user_id", a.IsPaymentBlock)

		coinsAdmin := admin.Group("/coin")
		{
			coinsAdmin.PUT("/highlight/:coin_symbol/:switcher", a.Restrict(false), a.HasPerm("coin.edit"), a.TrackAdminActivity(), a.SetCoinHighlight)
			coinsAdmin.PUT("/newListing/:coin_symbol/:switcher", a.Restrict(false), a.HasPerm("coin.edit"), a.TrackAdminActivity(), a.SetCoinNewListing)
		}

		botsAdmin := admin.Group("/bots")
		{
			botsAdmin.GET("", a.GetInfoAboutBots)
			botsAdmin.GET("/pnl", a.GetBotsPnlForAdmin)
			botsAdmin.GET("/total_lock_funds", a.GetTotalOfLockedFunds)
			botsAdmin.GET("/statistics/:bot_type", a.GetBotsStatistics)
			botsAdmin.GET("/statistics/:bot_type/export", a.ExportBotsStatistics)
		}

		admin.GET("/prdx-circulation", a.GetPRDXCirculation)
		admin.GET("/price-limits/", a.GetPriceLimits)
		admin.POST("/price-limits/", a.SetPriceLimits)

		maintenance := admin.Group("system-info/maintenance")
		{
			maintenance.POST("", a.CreateSystemInfoMaintenance)
			maintenance.PUT("/status", a.ChangeStatusSystemInfoMaintenance)
		}
		admin.GET("/activity-monitor", a.GetActivityMonitorList)

		admin.GET("/contracts/history", a.GetBonusAccountContractsHistoryListAdmin)
		admin.GET("/contracts/history/export/:contract_id", a.ExportBonusAccountContractsHistoryListAdmin)

		admin.PUT("/volume-distributed-percent", a.UpdateVolumeDistributedPercent)
		admin.GET("/volume-distributed-percent", a.GetMarketMakerVolumePercentValue)

		admin.PUT("/user/kyb/:user_id/update_step_two", a.SetUserKYBStepTwoStatus)
		admin.GET("/user/kyb/:user_id/download_document", a.DownloadKYBDocument)
		admin.GET("/user/kyb/:user_id", a.GetUserKYBStatusByID)
	}

	//admin features
	r.POST("/features", a.Restrict(false), a.HasPerm("feature.edit"), a.UpdateFeatures)
	r.PUT("/features/:feature_name", a.Restrict(false), a.HasPerm("feature.edit"), a.UpdateFeature)
	r.GET("/features/:feature_name", a.GetFeatureValue)

	r.GET("/helpers/city", a.Restrict(false), a.GetCityHelper)

	withdrawRequests := r.Group("/withdraw_requests", a.Restrict(false), a.RestrictByApiKeyPermissions("withdrawal_allowed"))
	{
		withdrawRequests.POST("/:withdraw_request_id", a.HasPerm("withdraw.manage"), a.GetWithdrawRequest(), a.ProcessWithdrawRequest)
	}

	r.GET("/probe/live", a.SystemInfoProbeLive)

	debug := r.Group("/debug")
	{
		limit.TrustedHeaderField = "X-Forwarded-For"
		debug.Use(limit.CIDR(srv.config.Server.Debug.AllowedIPs))

		debug.GET("/pprof/:name", func(context *gin.Context) {
			pprof.Handler(context.Param("name")).ServeHTTP(context.Writer, context.Request)
		})
	}

	r.GET("/socket", a.AuthMiddleware(a.NewWebsocketHandler()))
	r.GET("/ws/private", a.AuthMiddleware(a.NewWebsocketHandler()))

	srv.HTTP = &http.Server{
		Addr:    fmt.Sprintf(":%d", srv.config.Server.API.Port),
		Handler: r,
	}

	srv.HTTP.SetKeepAlivesEnabled(srv.config.Server.API.KeepAlive)

	port := srv.config.Server.API.Port
	httpServer := srv.HTTP
	if err := httpServer.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			log.Error().Err(err).Str("section", "server").Str("action", "ListenToRequests").Msgf("Unable to listen %d port", port)
		}
	}
}
