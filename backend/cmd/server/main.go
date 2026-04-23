package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/j1udu/cloud-storage-system/backend/internal/config"
	"github.com/j1udu/cloud-storage-system/backend/internal/database"
	"github.com/j1udu/cloud-storage-system/backend/internal/handler"
	"github.com/j1udu/cloud-storage-system/backend/internal/repository"
	"github.com/j1udu/cloud-storage-system/backend/internal/router"
	"github.com/j1udu/cloud-storage-system/backend/internal/service"
	"github.com/j1udu/cloud-storage-system/backend/internal/storage"
)

func main() {
	// 1. 加载配置
	cfg, err := config.Load("internal/config/config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 2. 连接 MySQL
	db, err := database.InitMySQL(cfg.MySQL)
	if err != nil {
		log.Fatalf("初始化 MySQL 失败: %v", err)
	}
	defer db.Close()
	fmt.Println("MySQL 连接成功")

	// 3. 连接 Redis
	rdb, err := database.InitRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("初始化 Redis 失败: %v", err)
	}
	defer rdb.Close()
	fmt.Println("Redis 连接成功")

	// 4. 连接 MinIO
	minioClient, err := storage.InitMinIO(cfg.MinIO)
	if err != nil {
		log.Fatalf("初始化 MinIO 失败: %v", err)
	}
	objStorage := storage.NewObjectStorage(minioClient, cfg.MinIO.Bucket)
	fmt.Println("MinIO 连接成功")

	// 5. 依赖注入：创建 Repo → Service → Handler
	userRepo := repository.NewUserRepo(db)
	userService := service.NewUserService(userRepo, cfg.JWT.Secret, cfg.JWT.ExpireHour)
	userHandler := handler.NewUserHandler(userService)

	fileRepo := repository.NewFileRepo(db)
	fileService := service.NewFileService(fileRepo, objStorage)
	fileHandler := handler.NewFileHandler(fileService)

	// 5. 创建 Gin 引擎，注册路由
	r := gin.Default()
	router.Setup(r, userHandler, fileHandler, cfg.JWT.Secret)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "success",
			"data": gin.H{
				"message": "pong",
			},
		})
	})

	// 6. 启动服务器
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("服务器启动在 %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
