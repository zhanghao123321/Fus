package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"Fus/backend/config"
	"Fus/backend/utils"

	"github.com/gin-gonic/gin"
)

type FileMeta struct {
	Filename string    `json:"filename"`
	Expiry   time.Time `json:"expiry"`
}

func Upload(c *gin.Context) {
	// 优先从cookie获取用户名
	user := ""
	if username, err := c.Cookie("username"); err == nil {
		user = username
	}

	// 如果cookie中没有，再尝试从Authorization头获取
	if user == "" {
		if authHeader := c.GetHeader("Authorization"); authHeader != "" {
			if strings.HasPrefix(authHeader, "Basic ") {
				decoded, err := base64.StdEncoding.DecodeString(authHeader[6:])
				if err == nil {
					credParts := strings.SplitN(string(decoded), ":", 2)
					if len(credParts) == 2 {
						user = credParts[0]
					}
				}
			}
		}
	}

	// 如果还没有用户名，使用默认值
	if user == "" {
		user = "anonymous"
	}

	// 创建用户目录
	userDir := utils.SanitizeUsername(user)
	userPath := filepath.Join(config.UploadPath, userDir)
	if err := os.MkdirAll(userPath, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法创建用户目录"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效请求"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有文件"})
		return
	}

	var paths []string
	if form.Value["paths"] != nil {
		paths = form.Value["paths"]
	}

	expireUnit := c.PostForm("expireUnit")
	expireValue := c.PostForm("expireValue")

	expiry := time.Time{}
	if expireUnit != "forever" {
		value, err := strconv.Atoi(expireValue)
		if err == nil {
			switch expireUnit {
			case "minute":
				expiry = time.Now().Add(time.Duration(value) * time.Minute)
			case "hour":
				expiry = time.Now().Add(time.Duration(value) * time.Hour)
			case "day":
				expiry = time.Now().AddDate(0, 0, value)
			case "month":
				expiry = time.Now().AddDate(0, value, 0)
			case "year":
				expiry = time.Now().AddDate(value, 0, 0)
			}
		}
	}

	folder := ""
	if form.Value["folder"] != nil && len(form.Value["folder"]) > 0 {
		folder = form.Value["folder"][0]
	}

	// 处理所有文件
	var uploadedFiles []string
	originalFolder := folder
	fileRenameMap := make(map[string]string)

	// 检查文件夹是否需要重命名
	if folder != "" {
		folderPath := filepath.Join(userPath, folder)
		if _, err := os.Stat(folderPath); err == nil {
			folder = utils.GenerateUniqueFoldername(folder)
			log.Printf("文件夹已存在，重命名为: %s", folder)
		}
	}

	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("无法打开文件: %v", err)
			continue
		}

		originalName := fileHeader.Filename
		baseName := filepath.Base(originalName)

		var targetPath, relativePath, finalFilename string

		if folder != "" && len(paths) > i {
			relativePath = paths[i]
			finalFilename = filepath.Base(relativePath)

			filePath := filepath.Join(userPath, folder, relativePath)
			if _, err := os.Stat(filePath); err == nil {
				newName := utils.GenerateUniqueFilename(finalFilename)
				fileRenameMap[finalFilename] = newName
				dirPart := filepath.Dir(relativePath)
				relativePath = filepath.Join(dirPart, newName)
				finalFilename = newName
				log.Printf("文件已存在，重命名为: %s", newName)
			}

			targetPath = filepath.Join(userPath, folder, relativePath)
		} else {
			finalFilename = baseName
			tempPath := filepath.Join(userPath, baseName)
			if _, err := os.Stat(tempPath); err == nil {
				newName := utils.GenerateUniqueFilename(baseName)
				fileRenameMap[baseName] = newName
				finalFilename = newName
				log.Printf("文件已存在，重命名为: %s", newName)
			}
			relativePath = finalFilename
			targetPath = filepath.Join(userPath, finalFilename)
		}

		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
			log.Printf("无法创建目录 %s: %v", targetDir, err)
			continue
		}

		out, err := os.Create(targetPath)
		if err != nil {
			log.Printf("无法创建文件 %s: %v", targetPath, err)
			continue
		}

		if _, err := io.Copy(out, file); err != nil {
			log.Printf("文件保存失败 %s: %v", targetPath, err)
			continue
		}

		file.Close()
		out.Close()

		meta := FileMeta{
			Filename: finalFilename,
			Expiry:   expiry,
		}
		metaPath := targetPath + ".meta"
		metaFile, err := os.Create(metaPath)
		if err != nil {
			log.Printf("无法创建元数据文件 %s: %v", metaPath, err)
			continue
		}

		if err := json.NewEncoder(metaFile).Encode(meta); err != nil {
			log.Printf("元数据写入失败 %s: %v", metaPath, err)
		}
		metaFile.Close()

		uploadedFiles = append(uploadedFiles, relativePath)
		log.Printf("文件保存成功: %s", targetPath)
	}

	if len(uploadedFiles) > 0 {
		var url string
		if folder != "" {
			url = fmt.Sprintf("/%s/%s", userDir, folder)
		} else if len(uploadedFiles) > 0 {
			finalFilename := filepath.Base(uploadedFiles[0])
			url = fmt.Sprintf("/%s/%s", userDir, finalFilename)
		}

		responseData := gin.H{
			"url":    url,
			"expiry": expiry.Format("2006-01-02 15:04:05"),
			"folder": folder,
		}

		if len(fileRenameMap) > 0 || originalFolder != folder {
			renamedInfo := gin.H{}
			for orig, newName := range fileRenameMap {
				renamedInfo[orig] = newName
			}
			if originalFolder != folder {
				renamedInfo[originalFolder] = folder
			}
			responseData["renamed"] = renamedInfo
		}

		c.JSON(http.StatusOK, responseData)
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件保存失败"})
	}
}
