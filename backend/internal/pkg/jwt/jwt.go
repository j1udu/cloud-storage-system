package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 是 JWT 令牌里存的数据
type Claims struct {
	UserID uint64 `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT 令牌
func GenerateToken(userID uint64, secret string, expireHour int) (string, int64, error) {
	expiresAt := time.Now().Add(time.Duration(expireHour) * time.Hour)

	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "cloud-storage-system",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))

	return tokenString, expiresAt.Unix(), err
}

// ParseToken 解析并验证 JWT 令牌
func ParseToken(tokenString string, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrTokenInvalidClaims
}
