package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"qinjiagateway/config"
	"qinjiagateway/model"

	"github.com/golang-jwt/jwt/v5"
)

// HashPassword 密码哈希（使用SHA256+盐）
func HashPassword(password string) string {
	h := sha256.New()
	h.Write([]byte("qinjia_salt_" + password))
	return hex.EncodeToString(h.Sum(nil))
}

// CheckPassword 校验密码
func CheckPassword(password, hashed string) bool {
	return HashPassword(password) == hashed
}

// GenerateToken 生成JWT Token
func GenerateToken(user *model.User) (string, int64, error) {
	expire := time.Now().Add(time.Duration(config.Global.JWT.Expire) * time.Second).Unix()
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      expire,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString([]byte(config.Global.JWT.Secret))
	return t, expire, err
}

// ParseToken 解析Token
func ParseToken(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Global.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}
