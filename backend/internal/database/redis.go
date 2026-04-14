package database

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/j1udu/cloud-storage-system/backend/internal/config"
)

// InitRedis 创建并返回一个 Redis 客户端
func InitRedis(cfg config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// 验证连接是否通了
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}

	return rdb, nil
}
