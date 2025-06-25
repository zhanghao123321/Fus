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
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// 公开路径不需要认证
		if strings.HasPrefix(c.Request.URL.Path, "/static") ||
			strings.HasPrefix(c.Request.URL.Path, "/files") ||
			c.Request.URL.Path == "/login" {
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
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(authHeader[6:])
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

		storedPass, exists := config.Credentials[user]
		if !exists || subtle.ConstantTimeCompare([]byte(pass), []byte(storedPass)) != 1 {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Next()
	}
}
