package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// SessionCache 登录会话缓存，用于登出时让 token 失效
type SessionCache struct {
	rdb *redis.Client
}

func NewSessionCache(rdb *redis.Client) *SessionCache {
	return &SessionCache{rdb: rdb}
}

func sessionKey(userID uint64) string {
	return fmt.Sprintf("cloud:session:%d", userID)
}

// Set 登录成功后写入会话
func (c *SessionCache) Set(ctx context.Context, userID uint64, token string, expiresAt int64) error {
	data := map[string]interface{}{
		"token":     token,
		"expiresAt": expiresAt,
	}
	return c.rdb.HSet(ctx, sessionKey(userID), data).Err()
}

// IsValid 校验 token 是否仍然有效（登出后会被删除）
func (c *SessionCache) IsValid(ctx context.Context, userID uint64, token string) (bool, error) {
	stored, err := c.rdb.HGet(ctx, sessionKey(userID), "token").Result()
	if err == redis.Nil {
		// 没有会话记录，说明没有登出过，token 本身有效就放行
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return stored == token, nil
}

// Delete 登出时删除会话，使当前 token 失效
func (c *SessionCache) Delete(ctx context.Context, userID uint64) error {
	return c.rdb.Del(ctx, sessionKey(userID)).Err()
}
