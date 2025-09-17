package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"Fus/backend/config"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// OPTIONS 请求通常用于CORS预检，不进行认证
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// 检查会话Cookie
		sessionToken, err := c.Cookie("session_token")
		if err == nil && sessionToken == "authenticated" {
			c.Next()
			return
		}

		// Basic认证
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Basic ") {
			// Authorization头格式不正确
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// 解码Basic认证的凭据
		decoded, err := base64.StdEncoding.DecodeString(authHeader[len("Basic "):])
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		credParts := strings.SplitN(string(decoded), ":", 2)
		if len(credParts) != 2 {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		user := credParts[0]
		pass := credParts[1]

		// 验证用户名和密码
		storedPass, exists := config.Credentials[user]
		if !exists || subtle.ConstantTimeCompare([]byte(pass), []byte(storedPass)) != 1 {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// 认证通过
		c.Next()
	}
}
