package middleware

import (
	"net/http"
	"strings"

	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
)

// JWTAuth JWT认证中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusOK, utils.Response{Code: 401, Msg: "未登录"})
			c.Abort()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusOK, utils.Response{Code: 401, Msg: "认证格式错误"})
			c.Abort()
			return
		}
		claims, err := utils.ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusOK, utils.Response{Code: 401, Msg: "Token无效或已过期"})
			c.Abort()
			return
		}
		c.Set("user_id", uint(claims["user_id"].(float64)))
		c.Set("username", claims["username"].(string))
		c.Set("role", claims["role"].(string))
		c.Next()
	}
}

// AdminAuth 管理员权限校验
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" {
			c.JSON(http.StatusOK, utils.Response{Code: 403, Msg: "无管理员权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}
