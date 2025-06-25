package main

import (
	"Fus/backend/cleanup"
	"Fus/backend/config"
	"Fus/backend/handlers"
	"Fus/backend/middleware"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
)

func main() {
	if err := config.LoadConfig(); err != nil {
		log.Fatal("配置加载失败:", err)
	}

	// 确保存储目录存在
	if err := os.MkdirAll(config.UploadPath, os.ModePerm); err != nil {
		log.Fatal("创建上传目录失败:", err)
	}

	r := gin.Default()

	r.LoadHTMLGlob("./frontend/templates/*")
	r.Static("/static", "./frontend/static")

	// 中间件
	r.Use(middleware.AuthMiddleware())

	// 特殊路由处理
	r.Handle("HEAD", "/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// 路由设置
	r.GET("/login", handlers.LoginPage)
	r.POST("/login", handlers.Login)
	r.GET("/", handlers.HomePage)
	r.POST("/upload", handlers.Upload)
	r.GET("/:username/*filepath", handlers.FileAccess)
	r.GET("/logout", handlers.Logout)

	// 启动清理任务
	go cleanup.StartCleanup()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("服务启动: http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("服务器启动失败:", err)
	}
}
