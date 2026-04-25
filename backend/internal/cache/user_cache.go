package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// UserCache 用户信息缓存
type UserCache struct {
	rdb *redis.Client
}

func NewUserCache(rdb *redis.Client) *UserCache {
	return &UserCache{rdb: rdb}
}

func userCacheKey(userID uint64) string {
	return fmt.Sprintf("cloud:userinfo:%d", userID)
}

// Get 取缓存，命中返回数据，未命中返回 nil
func (c *UserCache) Get(ctx context.Context, userID uint64) (map[string]string, error) {
	val, err := c.rdb.HGetAll(ctx, userCacheKey(userID)).Result()
	if err != nil {
		return nil, err
	}
	if len(val) == 0 {
		return nil, nil
	}
	return val, nil
}

// Set 写缓存，TTL 10分钟
func (c *UserCache) Set(ctx context.Context, userID uint64, data map[string]interface{}) error {
	return c.rdb.HSet(ctx, userCacheKey(userID), data).Err()
}

// Delete 删缓存（修改用户信息后调用）
func (c *UserCache) Delete(ctx context.Context, userID uint64) error {
	return c.rdb.Del(ctx, userCacheKey(userID)).Err()
}
