package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/gin-gonic/gin"
	gouuid "github.com/nu7hatch/gouuid"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"net/http"
	"strconv"
)

func (actions *Actions) AdvCashDepositRequest(c *gin.Context) {
	userID, _ := getUserID(c)
	assetId, _ := c.Params.Get("asset")
	amount := c.PostForm("amount")
	amountAsDecimal := conv.NewDecimalWithPrecision()
	amountAsDecimal.SetString(amount)
	advId, _ := gouuid.NewV4()

	advAssetId, found := actions.cfg.AdvCash.GetAssetsMap()[assetId]
	if !found {
		abortWithError(c, 400, "Unknown asset")
		return
	}

	rawSignature := actions.cfg.AdvCash.Email + ":" + actions.cfg.AdvCash.SciName + ":" + amount + ":" + advAssetId + ":" + actions.cfg.AdvCash.Password + ":" + advId.String()
	hash := sha256.Sum256([]byte(rawSignature))
	signature := hex.EncodeToString(hash[:])

	advCashRequest := model.AdvDepositRequests{
		AdvId:             advId.String(),
		UserID:            userID,
		CoinSymbol:        assetId,
		Signature:         signature,
		Amount:            &postgres.Decimal{V: amountAsDecimal},
		Status:            model.AdvRequestStatus_New,
		TransactionStatus: model.AdvTransactionStatus_Pending,
	}

	tx := actions.service.GetRepo().Conn.Begin()

	db := tx.Create(&advCashRequest)
	if db.Error != nil {
		tx.Rollback()
		abortWithError(c, 400, db.Error.Error())
		return
	}
	db = tx.Commit()
	if db.Error != nil {
		tx.Rollback()
		abortWithError(c, 400, db.Error.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"ac_account_email": actions.cfg.AdvCash.Email,
		"ac_sci_name":      actions.cfg.AdvCash.SciName,
		"ac_amount":        amount,
		"ac_currency":      advAssetId,
		"ac_order_id":      advId.String(),
		"ac_sign":          signature,
		"custom_fields":    "userId:" + strconv.FormatUint(userID, 10) + ";request_id:" + strconv.FormatUint(advCashRequest.ID, 10),
	})
}
