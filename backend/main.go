package main

import (
	"Fus/backend/cleanup"
	"Fus/backend/config"
	"Fus/backend/handlers"
	"Fus/backend/middleware"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
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

	publicRoutes := r.Group("/")
	{
		// 文件访问路由
		publicRoutes.GET("/:username/*filepath", handlers.FileAccess)

		publicRoutes.GET("/login", handlers.LoginPage)
		publicRoutes.POST("/login", handlers.Login)

		publicRoutes.Handle("HEAD", "/login", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
	}

	authRoutes := r.Group("/")
	authRoutes.Use(middleware.AuthMiddleware())
	{
		authRoutes.GET("/", handlers.HomePage)
		authRoutes.POST("/upload", handlers.Upload)
		authRoutes.GET("/logout", handlers.Logout)
	}

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
