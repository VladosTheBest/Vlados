package actions

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

func (actions *Actions) CreateAnnouncements(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	images := form.File["image"]
	var image *multipart.FileHeader
	if len(images) == 1 {
		image = images[0]
	}

	for _, documentFile := range images {
		documentFile.Filename = strings.ToLower(documentFile.Filename)
		if documentFile.Size > utils.MaxEmailFilesSize {
			msg := fmt.Sprintf("Size of attachements should be <= %s",
				utils.HumaneFileSize(utils.MaxEmailFilesSize),
			)
			c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
				"error": msg,
			})
			return
		}
	}

	announcementUpdates := model.Announcement{}
	if err := c.ShouldBind(&announcementUpdates); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	err = actions.service.ValidateAnnouncement(&announcementUpdates)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	if announcementUpdates.ID == 0 {
		_, err = actions.service.SendAnnouncement(announcementUpdates, image)
		if err != nil {
			abortWithError(c, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		announcement := model.Announcement{}
		if err := actions.service.GetRepo().FindByID(&announcement, uint(announcementUpdates.ID)); err != nil {
			abortWithError(c, http.StatusBadRequest, "Announcement not found")
			return
		}

		if err := actions.service.UpdateAnnouncement(&announcement, &announcementUpdates, image); err != nil {
			abortWithError(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) GetAnnouncements(c *gin.Context) {
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)

	announcements, err := actions.service.GetAnnouncements(page, limit)

	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, announcements)
}

func (actions *Actions) GetAdminAnnouncements(c *gin.Context) {
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	status := c.Query("status")

	announcements, err := actions.service.GetAdminAnnouncements(page, limit, status)

	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, announcements)
}

func (actions *Actions) ChangeAnnouncementsStatus(c *gin.Context) {
	announcementId, announcementIdExist := c.GetPostForm("id")
	statusStr, _ := c.GetPostForm("status")

	status := model.AnnouncementStatus(statusStr)
	if !status.IsValid() {
		abortWithError(c, http.StatusBadRequest, "Status parameter is wrong")
		return
	}

	if !announcementIdExist {
		abortWithError(c, http.StatusBadRequest, "id parameter is wrong")
		return
	}

	id, err := strconv.ParseUint(announcementId, 10, 64)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Unable to parse id")
		return
	}

	announcement := model.Announcement{}

	if err := actions.service.GetRepo().FindByID(&announcement, uint(id)); err != nil {
		abortWithError(c, http.StatusBadRequest, "Announcement not found")
		return
	}

	if err := actions.service.ChangeAnnouncementStatus(&announcement, status); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, status)
}

func (actions *Actions) GetAdminAnnouncementByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "announcement not found")
		return
	}

	announcements, err := actions.service.GetAdminAnnouncementByID(uint64(id))
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, announcements)
}

func (actions *Actions) GetAnnouncementByTopicAndID(c *gin.Context) {
	idStr := c.Query("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "announcement not found")
		return
	}

	topic := c.Query("topic") // Получить значение параметра "topic" из запроса

	current, previous, next, err := actions.service.GetAnnouncementByTopicAndID(topic, uint64(id))
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	response := struct {
		Current  *model.Announcement `json:"current"`
		Previous *model.Announcement `json:"previous"`
		Next     *model.Announcement `json:"next"`
	}{
		Current:  current,
		Previous: previous,
		Next:     next,
	}

	c.JSON(http.StatusOK, response)
}

func (actions *Actions) DeleteAdminAnnouncementByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "announcement not found")
		return
	}

	err = actions.service.DeleteAdminAnnouncementByID(uint64(id))
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) GetAnnouncementsByTopic(c *gin.Context) {
	topic := c.Query("topic")
	if topic == "" {
		abortWithError(c, http.StatusBadRequest, "Topic parameter is missing")
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		abortWithError(c, http.StatusBadRequest, "Invalid page number")
		return
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize <= 0 {
		abortWithError(c, http.StatusBadRequest, "Invalid page size")
		return
	}

	offset := (page - 1) * pageSize

	announcements, totalCount, err := actions.service.GetAnnouncementsByTopic(topic, offset, pageSize)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalPages := totalCount / pageSize
	if totalCount%pageSize != 0 {
		totalPages++
	}

	response := struct {
		Data []model.Announcement `json:"data"`
		Meta struct {
			TotalCount  int `json:"total_count"`
			TotalPages  int `json:"total_pages"`
			PageSize    int `json:"page_size"`
			CurrentPage int `json:"current_page"`
		} `json:"meta"`
	}{
		Data: announcements,
		Meta: struct {
			TotalCount  int `json:"total_count"`
			TotalPages  int `json:"total_pages"`
			PageSize    int `json:"page_size"`
			CurrentPage int `json:"current_page"`
		}{
			TotalCount:  totalCount,
			TotalPages:  totalPages,
			PageSize:    pageSize,
			CurrentPage: page,
		},
	}

	c.JSON(http.StatusOK, response)
}
