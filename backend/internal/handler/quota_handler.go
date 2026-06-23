package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/j1udu/cloud-storage-system/backend/internal/pkg/errcode"
	"github.com/j1udu/cloud-storage-system/backend/internal/service"
)

type QuotaHandler struct {
	quotaService *service.QuotaService
}

func NewQuotaHandler(quotaService *service.QuotaService) *QuotaHandler {
	return &QuotaHandler{quotaService: quotaService}
}

func (h *QuotaHandler) Get(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "invalid user id")
		return
	}

	resp, err := h.quotaService.Get(userID.(uint64))
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, resp)
}
