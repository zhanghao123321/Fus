package handlers

import (
	"crypto/subtle"
	"net/http"

	"Fus/backend/config"

	"github.com/gin-gonic/gin"
)

func LoginPage(c *gin.Context) {
	if sessionToken, err := c.Cookie("session_token"); err == nil && sessionToken == "authenticated" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "login.html", nil)
}

func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	storedPass, exists := config.Credentials[username]
	if !exists || subtle.ConstantTimeCompare([]byte(password), []byte(storedPass)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "无效的用户名或密码",
			"details": gin.H{
				"username_exists": exists,
				"password_match":  exists && subtle.ConstantTimeCompare([]byte(password), []byte(storedPass)) == 1,
			},
		})
		return
	}

	c.SetCookie("session_token", "authenticated", 3600, "/", "", false, false)
	c.SetCookie("username", username, 3600, "/", "", false, false)

	c.JSON(http.StatusOK, gin.H{
		"message":  "登录成功",
		"redirect": "/",
	})
}

func Logout(c *gin.Context) {
	c.SetCookie("session_token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

func HomePage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}
