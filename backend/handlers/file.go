package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Fus/backend/config"

	"github.com/gin-gonic/gin"
)

func FileAccess(c *gin.Context) {
	username := c.Param("username")
	filePath := c.Param("filepath")

	// 对于匿名用户，不检查其是否在 config.Credentials 中
	if username != "anonymous" {
		if _, exists := config.Credentials[username]; !exists {
			c.String(http.StatusNotFound, "用户不存在")
			return
		}
	}

	// 安全检查：防止路径遍历攻击
	filePath = strings.Trim(filePath, "/")
	if strings.Contains(filePath, "..") || strings.Contains(filePath, "//") {
		c.String(http.StatusForbidden, "无效的文件路径")
		return
	}

	fullPath := filepath.Join(config.UploadPath, username, filePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.String(http.StatusNotFound, "文件或目录不存在")
		} else {
			c.String(http.StatusInternalServerError, "服务器错误")
		}
		return
	}

	if info.IsDir() {
		if !strings.HasSuffix(c.Request.URL.Path, "/") {
			c.Redirect(http.StatusMovedPermanently, c.Request.URL.Path+"/")
			return
		}
		isRoot := strings.Trim(filePath, "/") == ""
		defaultFiles := []string{"index.html", "home.html"}

		if !isRoot {
			parts := strings.Split(filePath, "/")
			if len(parts) > 1 {
				for _, defaultFile := range defaultFiles {
					indexPath := filepath.Join(fullPath, defaultFile)
					if _, err := os.Stat(indexPath); err == nil {
						processFileAccess(c, indexPath)
						return
					}
				}
			}
		}

		// 如果没有默认HTML文件，则生成目录列表
		generateDirectoryListing(c, fullPath, username+"/"+filePath)
		return
	}

	// 如果是文件，则直接处理文件访问（包括过期检查）
	processFileAccess(c, fullPath)
}

func processFileAccess(c *gin.Context, filePath string) {
	// 防止直接访问 .meta 文件
	if strings.HasSuffix(filePath, ".meta") {
		c.String(http.StatusNotFound, "文件不存在")
		return
	}

	metaPath := filePath + ".meta"
	// 检查是否存在元数据文件
	if _, err := os.Stat(metaPath); err == nil {
		metaFile, err := os.Open(metaPath)
		if err != nil {
			log.Printf("Error opening meta file %s: %v", metaPath, err)
			c.String(http.StatusInternalServerError, "无法读取文件元数据")
			return
		}
		defer metaFile.Close()

		var meta FileMeta
		if err := json.NewDecoder(metaFile).Decode(&meta); err != nil {
			log.Printf("Error decoding meta file %s: %v", metaPath, err)
			c.String(http.StatusInternalServerError, "元数据解析错误")
			return
		}

		// 检查文件是否过期
		if !meta.Expiry.IsZero() && time.Now().After(meta.Expiry) {
			// 删除过期文件和元数据文件
			os.Remove(filePath)
			os.Remove(metaPath)
			c.String(http.StatusForbidden, "文件已过期")
			return
		}
	}

	// 最终检查文件是否存在，防止元数据文件存在但实际文件被手动删除的情况
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.String(http.StatusNotFound, "文件不存在")
		return
	}

	// 提供文件下载/预览
	c.File(filePath)
}
