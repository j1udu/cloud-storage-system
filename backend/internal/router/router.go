package router

import (
	"github.com/gin-gonic/gin"
	"github.com/j1udu/cloud-storage-system/backend/internal/handler"
	"github.com/j1udu/cloud-storage-system/backend/internal/middleware"
)

// Setup 注册所有路由
func Setup(r *gin.Engine, userHandler *handler.UserHandler, jwtSecret string) {
	// 全局中间件
	r.Use(middleware.CORSMiddleware())

	// API v1 路由组
	v1 := r.Group("/api/v1")

	// 用户认证路由（不需要登录）
	auth := v1.Group("/auth")
	{
		auth.POST("/register", userHandler.Register)
		auth.POST("/login", userHandler.Login)
	}

	// 需要登录的路由
	authRequired := v1.Group("/auth")
	authRequired.Use(middleware.AuthMiddleware(jwtSecret))
	{
		authRequired.GET("/profile", userHandler.GetProfile)
	}
}
