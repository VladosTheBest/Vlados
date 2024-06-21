package actions

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"mime/multipart"
	"net/http"
)

// SendSupportRequestEmail godoc
// swagger:route POST /support/:type support request
// Send opportunities request email
//
// The user can send an email to support using this endpoint.

// Consumes:
// - multipart/form-data
//
// Responses:
//
//	200: StringResp
//	400: RequestErrorResp
func (actions *Actions) SendSupportRequestEmail(c *gin.Context) {

	emailRequestType := service.SupportRequestEmailType(c.Param("type"))

	if !emailRequestType.IsValid() {
		abortWithError(c, http.StatusBadRequest, "Something wrong")
		return
	}
	name, _ := c.GetPostForm("name")
	email, _ := c.GetPostForm("email")
	message, _ := c.GetPostForm("message")

	var files []*multipart.FileHeader

	form, _ := c.MultipartForm()
	files = form.File["file"]

	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}

	if totalSize > utils.MaxEmailFilesSize {
		msg := fmt.Sprintf("Total size of attachements should be <= %s",
			utils.HumaneFileSize(utils.MaxEmailFilesSize),
		)
		abortWithError(c, http.StatusBadRequest, msg)
		return
	}

	err := actions.service.SendSupportRequestEmail(emailRequestType, email, name, message, files, "")
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Unable to send request")
		return
	}

	c.JSON(http.StatusOK, map[string]string{"message": "Support request sent successfully"})
}
