package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/j1udu/cloud-storage-system/backend/internal/model"
	"github.com/j1udu/cloud-storage-system/backend/internal/pkg/errcode"
	"github.com/j1udu/cloud-storage-system/backend/internal/service"
)

// UserHandler 用户接口，持有 UserService
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler 创建 UserHandler 实例
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// Register 注册接口
func (h *UserHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, errcode.ErrParamInvalid, "参数错误")
		return
	}

	user, err := h.userService.Register(&req)
	if err != nil {
		Fail(c, errcode.ErrUserExists, err.Error())
		return
	}

	Success(c, user)
}

// Login 登录接口
func (h *UserHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, errcode.ErrParamInvalid, "参数错误")
		return
	}

	resp, err := h.userService.Login(&req)
	if err != nil {
		Fail(c, errcode.ErrPasswordWrong, err.Error())
		return
	}

	Success(c, resp)
}

// GetProfile 获取用户信息接口
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "无效的用户ID")
		return
	}

	user, err := h.userService.GetProfile(userID.(uint64))
	if err != nil {
		Fail(c, errcode.ErrUserNotFound, "用户不存在")
		return
	}

	Success(c, user)
}
