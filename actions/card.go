package actions

import (
	"github.com/rs/zerolog/log"
	"net/http"
	"strconv"
	"strings"

	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/solaris"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

func (actions *Actions) ListAvailableCardTypes(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	cardsData, err := actions.service.GetAvailableCardsList(user.ID)
	if err != nil {
		abortWithError(c, 500, "Unable to get list of cards")
		return
	}

	c.JSON(http.StatusOK, cardsData)
}

func (actions *Actions) GetUserCurrentCardPaymentBalance(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	totalBalance, err := actions.service.GetCurrentUserCardPaymentBalance(user.ID)
	if err != nil {
		abortWithError(c, 500, "Unable to get total balance")
		return
	}

	c.JSON(http.StatusOK, totalBalance.String())
}

func (actions *Actions) DepositToCardAccount(c *gin.Context) {
	//todo: uncommit and add flag
	//if !featureflags.IsEnabled("api.cards.enable-deposit") {
	//	abortWithError(c, http.StatusBadRequest, "service temporary unavailable")
	//	return
	//}

	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	coin, ok := c.GetPostForm("coin")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "empty coin key provided")
		return
	}

	// todo: remove in future
	if coin != "EUR" {
		abortWithError(c, http.StatusBadRequest, "only eur supported")
		return
	}

	amount, ok := c.GetPostForm("amount")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "empty amount key provided")
		return
	}

	decAmount, ok := (&decimal.Big{}).SetString(amount)
	if !ok {
		abortWithError(c, http.StatusBadRequest, "Invalid amount provided")
		return
	}

	err := actions.service.DepositToCardAccount(user.ID, decAmount, strings.ToLower(coin))
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Can not deposit EUR to card account")
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) WithdrawFromCardAccount(c *gin.Context) {
	//todo: uncommit and add flag
	//if !featureflags.IsEnabled("api.cards.enable-withdraw") {
	//	abortWithError(c, http.StatusBadRequest, "service temporary unavailable")
	//	return
	//}

	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	coin, ok := c.GetPostForm("coin")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "empty coin key provided")
		return
	}

	// todo: remove in future
	if coin != "EUR" {
		abortWithError(c, http.StatusBadRequest, "only eur supported")
		return
	}

	amount, ok := c.GetPostForm("amount")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "empty amount key provided")
		return
	}

	decAmount, ok := (&decimal.Big{}).SetString(amount)
	if !ok {
		abortWithError(c, http.StatusBadRequest, "Invalid amount provided")
		return
	}

	err := actions.service.WithdrawFromCardAccount(user.ID, decAmount, strings.ToLower(coin))
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Can not deposit EUR to card account")
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) AddConsumer(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	kyc, err := actions.service.GetKycByID(user.KycID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Can not get user kyc details")
		return
	}

	if kyc.Status != model.KYCStatusStepTwoSuccess {
		abortWithError(c, http.StatusBadRequest, "Invalid kyc status for card issuance")
		return
	}

	cardIDStr, ok := c.GetPostForm("card_id")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "Invalid card_id provided")
		return
	}

	userPin, ok := c.GetPostForm("user_pin")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "Invalid PIN provided")
		return
	}

	cardID, err := strconv.ParseUint(cardIDStr, 10, 64)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid card_id provided")
		return
	}

	availableCards, err := actions.service.GetAvailableCardsList(user.ID)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Can not get available cards for user")
		return
	}

	var cardType model.CardType
	isValidCardID := false
	for _, card := range availableCards {
		if card.Card.Id == cardID && card.IsAllowed {
			isValidCardID = true
			cardType = card.Card
		}
	}

	if !isValidCardID {
		abortWithError(c, http.StatusInternalServerError, "Incorrect card id. Not enough balance")
		return
	}

	request := solaris.AddConsumerRequest{}

	err = c.ShouldBind(&request)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid request provided")
		return
	}

	cardDesignCode, ok := c.GetPostForm("card_design_code")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "Invalid card design code provided")
		return
	}

	coinSymbol, ok := c.GetPostForm("coin")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "Invalid PIN provided")
		return
	}

	userDetails, err := actions.service.GetUserDetails(user.ID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid request provided")
		return
	}

	// Settings for request to solaris, for further explanation see https://docs.solarisgroup.co.uk/api-reference/consumer/#tag/Consumer/paths/~1Consumer~1AddConsumers/post
	for _, consumer := range request.ConsumerRequestList {
		consumer.FirstName = user.FirstName
		consumer.LastName = user.LastName
		consumer.DOB = *userDetails.DOB
		consumer.Gender = solaris.NewGender(userDetails.Gender.String())
		consumer.EncryptedPIN, err = solaris.EncodeAse256(userPin, actions.cfg.Server.Card.Key)
		if err != nil {
			abortWithError(c, http.StatusBadRequest, "Error while encoding pin")
			return
		}
		consumer.CardDesignCode = cardDesignCode
		consumer.IsSkipKYC = false
		consumer.IsSkipCardIssuance = false
		consumer.IsPrimaryConsumer = false
		consumer.Relationship = 0
	}
	request.ClientRequestReference = solaris.GenerateClientRequestReference(30)
	request.AgreementCode = actions.cfg.Server.Card.AgreementCode
	request.Language = 1
	request.CultureID = 0

	err = actions.service.AddConsumer(user.ID, cardType, coinSymbol, &request)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Error occurred while adding consumer")
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) ActivateCard(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	code, ok := c.GetPostForm("user_pan")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "Incorrect code provided")
		return
	}

	encodedCode, err := solaris.EncodeAse256(code, actions.cfg.Server.Card.Key)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Can not encode code")
		return
	}

	cvv, ok := c.GetPostForm("user_cvv")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "Incorrect cvv provided")
		return
	}

	encodedCVV, err := solaris.EncodeAse256(cvv, actions.cfg.Server.Card.Key)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Can not encode code")
		return
	}

	userDetails, err := actions.service.GetUserDetails(user.ID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid request provided")
		return
	}

	request := solaris.ActivateCardRequest{
		EncryptedPAN:           encodedCode,
		EncryptedCVV:           encodedCVV,
		DOB:                    *userDetails.DOB,
		ClientRequestReference: solaris.GenerateClientRequestReference(30),
	}

	err = actions.service.ActivateCard(&request)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Can not activate card")
		return
	}

}

func (actions *Actions) AddToCardWaitList(c *gin.Context) {
	request := model.CardWaitList{}
	err := c.ShouldBind(&request)
	if err != nil {
		log.Error().Err(err).Msg("Error in binding request data")
		abortWithError(c, http.StatusInternalServerError, "Error binding data")
		return
	}

	request.Email = strings.TrimSpace(strings.ToLower(request.Email))

	_, err = actions.service.GetUserByEmail(request.Email)
	if err == nil {
		request.IsRegisteredUser = true
	}

	err = actions.service.AddToCardWaitList(&request)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Error adding to card wait-list")
		return
	}

	c.JSON(http.StatusOK, "OK")
}
