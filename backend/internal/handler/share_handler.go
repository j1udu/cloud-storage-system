package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/j1udu/cloud-storage-system/backend/internal/model"
	"github.com/j1udu/cloud-storage-system/backend/internal/pkg/errcode"
	"github.com/j1udu/cloud-storage-system/backend/internal/service"
)

type ShareHandler struct {
	shareService *service.ShareService
}

func NewShareHandler(shareService *service.ShareService) *ShareHandler {
	return &ShareHandler{shareService: shareService}
}

func (h *ShareHandler) Create(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "invalid user id")
		return
	}

	var req model.ShareCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, errcode.ErrParamInvalid, "invalid param")
		return
	}

	share, err := h.shareService.Create(userID.(uint64), &req)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, share)
}

func (h *ShareHandler) Cancel(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "invalid user id")
		return
	}

	shareID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, "invalid share id")
		return
	}

	if err := h.shareService.Cancel(userID.(uint64), shareID); err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, nil)
}

func (h *ShareHandler) List(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "invalid user id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.shareService.List(userID.(uint64), page, pageSize)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, resp)
}

func (h *ShareHandler) PublicInfo(c *gin.Context) {
	token := c.Param("token")

	resp, err := h.shareService.GetPublicInfo(token)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, resp)
}

func (h *ShareHandler) PublicDownload(c *gin.Context) {
	token := c.Param("token")

	var req model.ShareDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, errcode.ErrParamInvalid, "invalid param")
		return
	}

	url, err := h.shareService.Download(c.Request.Context(), token, &req)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, gin.H{"url": url})
}
