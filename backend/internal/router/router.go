package router

import (
	"github.com/gin-gonic/gin"
	"github.com/j1udu/cloud-storage-system/backend/internal/handler"
	"github.com/j1udu/cloud-storage-system/backend/internal/middleware"
)

func Setup(r *gin.Engine, userHandler *handler.UserHandler, fileHandler *handler.FileHandler, shareHandler *handler.ShareHandler, quotaHandler *handler.QuotaHandler, jwtSecret string) {
	r.Use(middleware.CORSMiddleware())

	v1 := r.Group("/api/v1")

	auth := v1.Group("/auth")
	{
		auth.POST("/register", userHandler.Register)
		auth.POST("/login", userHandler.Login)
	}

	v1.GET("/public/shares/:token", shareHandler.PublicInfo)
	v1.POST("/public/shares/:token/download", shareHandler.PublicDownload)

	authRequired := v1.Group("")
	authRequired.Use(middleware.AuthMiddleware(jwtSecret))
	{
		authRequired.GET("/auth/profile", userHandler.GetProfile)
		authRequired.POST("/auth/logout", userHandler.Logout)

		authRequired.POST("/files/upload", fileHandler.Upload)
		authRequired.GET("/files", fileHandler.List)
		authRequired.GET("/files/:id/download", fileHandler.Download)
		authRequired.DELETE("/files/:id", fileHandler.Delete)
		authRequired.PUT("/files/:id/rename", fileHandler.Rename)
		authRequired.PUT("/files/:id/move", fileHandler.Move)

		authRequired.GET("/recycle", fileHandler.ListRecycle)
		authRequired.PUT("/recycle/:id/restore", fileHandler.Restore)
		authRequired.DELETE("/recycle/:id", fileHandler.PermanentDelete)

		authRequired.POST("/shares", shareHandler.Create)
		authRequired.GET("/shares", shareHandler.List)
		authRequired.POST("/shares/:token/save", shareHandler.Save)
		authRequired.DELETE("/shares/:id", shareHandler.Cancel)

		authRequired.GET("/storage/quota", quotaHandler.Get)

		authRequired.POST("/folders", fileHandler.CreateFolder)
		authRequired.GET("/folders/path", fileHandler.GetPath)
	}
}
