package actions

import (
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/auth_service"
	"strconv"
	"strings"
)

func (actions *Actions) CheckPreAuthToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		parsedToken, err := ParseToken(token, actions.jwt2FATokenSecret)
		if err != nil {
			abortWithError(c, Unauthorized, err.Error())
			return
		}

		stage := parsedToken["stage"].(string)
		if !auth_service.IsValidPreAuthStage(stage) {
			abortWithError(c, Unauthorized, "Invalid credentials")
			return
		}

		userID, err := strconv.Atoi(parsedToken["key"].(string))
		if err != nil {
			abortWithError(c, Unauthorized, "Invalid credentials")
			return
		}
		user, err := actions.service.GetUserByID(uint(userID))
		if err != nil {
			abortWithError(c, Unauthorized, "Invalid credentials")
			return
		}

		c.Set("preauth_user", user)
		c.Set("preauth_stage", stage)
		c.Next()

	}
}

func (actions *Actions) CheckPrePartialToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		parsedToken, err := ParseToken(token, actions.jwt2FATokenSecret)
		if err != nil {
			abortWithError(c, Unauthorized, err.Error())
			return
		}
		userID, err := strconv.Atoi(parsedToken["key"].(string))
		if err != nil {
			abortWithError(c, Unauthorized, "Invalid credentials")
			return
		}
		user, err := actions.service.GetUserByID(uint(userID))
		if err != nil {
			abortWithError(c, Unauthorized, "Invalid credentials")
			return
		}
		authType := parsedToken["twoFAType"].(string)

		if user.Status != model.UserStatusActive {
			abortWithError(c, Unauthorized, "Your account is not active. Please contact support.")
			return
		}

		c.Set("preauth_user", user)
		c.Set("preauth_type", authType)
		c.Next()

	}
}
