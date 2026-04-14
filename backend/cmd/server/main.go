package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/j1udu/cloud-storage-system/backend/internal/config"
	"github.com/j1udu/cloud-storage-system/backend/internal/database"
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

	// 4. 创建 Gin 路由
	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "success",
			"data": gin.H{
				"message": "pong",
			},
		})
	})

	// 5. 启动服务器
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("服务器启动在 %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
