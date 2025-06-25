package handlers

import (
	"encoding/json"
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

	if _, exists := config.Credentials[username]; !exists {
		c.String(http.StatusNotFound, "用户不存在")
		return
	}

	filePath = strings.Trim(filePath, "/")
	if strings.Contains(filePath, "..") || strings.Contains(filePath, "//") {
		c.String(http.StatusForbidden, "无效的文件路径")
		return
	}

	fullPath := filepath.Join(config.UploadPath, username, filePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		c.String(http.StatusNotFound, "文件不存在")
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
			for _, defaultFile := range defaultFiles {
				indexPath := filepath.Join(fullPath, defaultFile)
				if _, err := os.Stat(indexPath); err == nil {
					processFileAccess(c, indexPath)
					return
				}
			}
		}

		generateDirectoryListing(c, fullPath, username+"/"+filePath)
		return
	}

	processFileAccess(c, fullPath)
}

func processFileAccess(c *gin.Context, filePath string) {
	if strings.HasSuffix(filePath, ".meta") {
		c.String(http.StatusNotFound, "文件不存在")
		return
	}

	metaPath := filePath + ".meta"
	if _, err := os.Stat(metaPath); err == nil {
		metaFile, err := os.Open(metaPath)
		if err != nil {
			c.String(http.StatusInternalServerError, "无法读取文件元数据")
			return
		}
		defer metaFile.Close()

		var meta FileMeta
		if err := json.NewDecoder(metaFile).Decode(&meta); err != nil {
			c.String(http.StatusInternalServerError, "元数据解析错误")
			return
		}

		if !meta.Expiry.IsZero() && time.Now().After(meta.Expiry) {
			os.Remove(filePath)
			os.Remove(metaPath)
			c.String(http.StatusForbidden, "文件已过期")
			return
		}
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.String(http.StatusNotFound, "文件不存在")
		return
	}

	c.File(filePath)
}
