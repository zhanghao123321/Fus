package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joho/godotenv"
)

var (
	Credentials = make(map[string]string)
	UploadPath  = "./storage/data"
)

func LoadConfig() error {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	envPath := filepath.Join(dir, "../.env") // 向上退一级到backend目录

	if err := godotenv.Load(envPath); err != nil {
		log.Println("未找到 .env 文件，将使用系统环境变量")
	}

	// 加载用户凭证
	users := os.Getenv("AUTH_USERS")
	if users == "" {
		return fmt.Errorf("未配置用户凭据")
	}

	pairs := strings.Split(users, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			username := strings.TrimSpace(parts[0])
			password := strings.TrimSpace(parts[1])
			if username != "" && password != "" {
				Credentials[username] = password
			}
		}
	}

	if len(Credentials) == 0 {
		return fmt.Errorf("未找到有效的用户凭据")
	}

	// 加载上传路径配置
	if path := os.Getenv("UPLOAD_PATH"); path != "" {
		UploadPath = path
	}

	log.Printf("已加载 %d 个用户凭据", len(Credentials))
	log.Printf("上传路径: %s", UploadPath)
	return nil
}
