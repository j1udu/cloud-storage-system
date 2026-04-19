package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
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
		msg := parseBindError(err)
		Fail(c, errcode.ErrParamInvalid, msg)
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
		msg := parseBindError(err)
		Fail(c, errcode.ErrParamInvalid, msg)
		return
	}

	resp, err := h.userService.Login(&req)
	if err != nil {
		Fail(c, errcode.ErrPasswordWrong, err.Error())
		return
	}

	Success(c, resp)
}

// parseBindError 解析参数校验错误，返回具体的中文提示
func parseBindError(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, e := range ve {
			switch e.Field() {
			case "Username":
				switch e.Tag() {
				case "required":
					return "用户名不能为空"
				case "min":
					return "用户名长度不能少于3个字符"
				case "max":
					return "用户名长度不能超过64个字符"
				}
			case "Password":
				switch e.Tag() {
				case "required":
					return "密码不能为空"
				case "min":
					return "密码长度不能少于6个字符"
				case "max":
					return "密码长度不能超过128个字符"
				}
			case "Nickname":
				if e.Tag() == "max" {
					return "昵称长度不能超过128个字符"
				}
			}
		}
	}
	return "参数错误"
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
