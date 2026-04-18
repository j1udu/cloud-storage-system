package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/j1udu/cloud-storage-system/backend/internal/handler"
	"github.com/j1udu/cloud-storage-system/backend/internal/pkg/errcode"
	pkgjwt "github.com/j1udu/cloud-storage-system/backend/internal/pkg/jwt"
)

// AuthMiddleware JWT 认证中间件
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从请求头取 Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			handler.Fail(c, errcode.ErrInvalidToken, "缺少认证令牌")
			c.Abort()
			return
		}

		// 2. 去掉 "Bearer " 前缀
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			handler.Fail(c, errcode.ErrInvalidToken, "令牌格式错误")
			c.Abort()
			return
		}

		// 3. 解析验证令牌
		claims, err := pkgjwt.ParseToken(tokenString, jwtSecret)
		if err != nil {
			handler.Fail(c, errcode.ErrInvalidToken, "令牌无效或已过期")
			c.Abort()
			return
		}

		// 4. 把 user_id 注入上下文
		c.Set("user_id", claims.UserID)

		// 5. 放行
		c.Next()
	}
}
