package cache

import (
	"context"
	"fmt"
	"time"

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

// Set 登录成功后写入会话，TTL 与 JWT 过期时间一致
func (c *SessionCache) Set(ctx context.Context, userID uint64, token string, expiresAt int64) error {
	data := map[string]interface{}{
		"token":     token,
		"expiresAt": expiresAt,
	}
	ttl := time.Until(time.Unix(expiresAt, 0))
	if ttl <= 0 {
		ttl = time.Minute
	}
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, sessionKey(userID), data)
	pipe.Expire(ctx, sessionKey(userID), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// IsValid 校验 token 是否仍然有效（白名单模式：必须命中且 token 匹配才放行）
func (c *SessionCache) IsValid(ctx context.Context, userID uint64, token string) (bool, error) {
	stored, err := c.rdb.HGet(ctx, sessionKey(userID), "token").Result()
	if err == redis.Nil {
		return false, nil
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
