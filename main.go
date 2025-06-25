package main

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	uploadPath = "./data"
)

var credentials = make(map[string]string)

type FileMeta struct {
	Filename string    `json:"filename"`
	Expiry   time.Time `json:"expiry"`
}

func loadConfig() error {
	if err := godotenv.Load(); err != nil {
		log.Println("未找到 .env 文件，将使用系统环境变量")
	}

	users := os.Getenv("AUTH_USERS")
	if users == "" {
		return fmt.Errorf("未配置用户凭据，请设置 AUTH_USERS 环境变量")
	}

	pairs := strings.Split(users, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			username := strings.TrimSpace(parts[0])
			password := strings.TrimSpace(parts[1])
			if username != "" && password != "" {
				credentials[username] = password
			}
		}
	}

	if len(credentials) == 0 {
		return fmt.Errorf("未找到有效的用户凭据")
	}

	log.Printf("已加载 %d 个用户凭据", len(credentials))
	return nil
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		if strings.HasPrefix(c.Request.URL.Path, "/static") ||
			strings.HasPrefix(c.Request.URL.Path, "/files") ||
			c.Request.URL.Path == "/login" {
			c.Next()
			return
		}

		sessionToken, err := c.Cookie("session_token")
		if err == nil && sessionToken == "authenticated" {
			c.Next()
			return
		}

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

		storedPass, exists := credentials[user]
		if !exists || subtle.ConstantTimeCompare([]byte(pass), []byte(storedPass)) != 1 {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Next()
	}
}

func generateUniqueFilename(original string) string {
	ext := filepath.Ext(original)
	name := strings.TrimSuffix(original, ext)

	// 使用时间戳 + 随机数确保唯一性
	timestamp := time.Now().Format("20060102150405")
	random := rand.Intn(10000) // 4位随机数
	return fmt.Sprintf("%s_%s_%04d%s", name, timestamp, random, ext)
}

func generateUniqueFoldername(original string) string {
	// 使用时间戳 + UUID 确保唯一性
	timestamp := time.Now().Format("20060102150405")
	uuidPart := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%s_%s", original, timestamp, uuidPart)
}

func main() {
	if err := loadConfig(); err != nil {
		log.Fatal("配置加载失败:", err)
	}

	if err := os.MkdirAll(uploadPath, os.ModePerm); err != nil {
		log.Fatal("Failed to create upload directory:", err)
	}

	r := gin.Default()

	r.Use(authMiddleware())

	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./static")

	r.Handle("HEAD", "/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	r.GET("/login", func(c *gin.Context) {
		if sessionToken, err := c.Cookie("session_token"); err == nil && sessionToken == "authenticated" {
			c.Redirect(http.StatusFound, "/")
			return
		}

		c.HTML(http.StatusOK, "login.html", nil)
	})

	r.POST("/login", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		storedPass, exists := credentials[username]
		if !exists || subtle.ConstantTimeCompare([]byte(password), []byte(storedPass)) != 1 {
			// 返回详细的错误信息
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "无效的用户名或密码",
				"details": gin.H{
					"username_exists": exists,
					"password_match":  exists && subtle.ConstantTimeCompare([]byte(password), []byte(storedPass)) == 1,
				},
			})
			return
		}

		// 设置用户名cookie
		c.SetCookie("session_token", "authenticated", 3600, "/", "", false, false)
		c.SetCookie("username", username, 3600, "/", "", false, false)

		c.JSON(http.StatusOK, gin.H{
			"message":  "登录成功",
			"redirect": "/",
		})
	})

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// 文件上传处理
	r.POST("/upload", func(c *gin.Context) {
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
		userDir := ""
		if user != "" {
			// 清理用户名，防止路径注入
			userDir = strings.ReplaceAll(user, "..", "")
			userDir = strings.ReplaceAll(userDir, "/", "")
			userDir = strings.ReplaceAll(userDir, "\\", "")
			userDir = strings.TrimSpace(userDir)

			if userDir == "" {
				userDir = "anonymous"
			}
		} else {
			userDir = "anonymous"
		}

		// 确保用户目录存在
		userPath := filepath.Join(uploadPath, userDir)
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

		// 检查文件夹是否需要重命名
		originalFolder := folder
		if folder != "" {
			folderPath := filepath.Join(userPath, folder)
			if _, err := os.Stat(folderPath); err == nil {
				// 文件夹已存在，生成唯一名称
				folder = generateUniqueFoldername(folder)
				log.Printf("文件夹已存在，重命名为: %s", folder)
			}
		}

		// 记录需要重命名的文件
		fileRenameMap := make(map[string]string)

		for i, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("无法打开文件: %v", err)
				continue
			}

			// 获取原始文件名
			originalName := fileHeader.Filename
			baseName := filepath.Base(originalName)

			// 修改目标路径计算 - 使用用户目录
			var targetPath string
			var relativePath string
			var finalFilename string

			if folder != "" && len(paths) > i {
				// 文件夹上传处理
				relativePath = paths[i]
				finalFilename = filepath.Base(relativePath) // 获取文件名部分

				// 检查文件是否需要重命名
				filePath := filepath.Join(userPath, folder, relativePath)
				if _, err := os.Stat(filePath); err == nil {
					// 文件已存在，生成唯一文件名
					newName := generateUniqueFilename(finalFilename)
					fileRenameMap[finalFilename] = newName

					// 更新相对路径
					dirPart := filepath.Dir(relativePath)
					relativePath = filepath.Join(dirPart, newName)
					finalFilename = newName
					log.Printf("文件已存在，重命名为: %s", newName)
				}

				targetPath = filepath.Join(userPath, folder, relativePath)
			} else {
				// 单个/多个文件上传处理
				finalFilename = baseName

				// 检查文件是否需要重命名
				tempPath := filepath.Join(userPath, baseName)
				if _, err := os.Stat(tempPath); err == nil {
					// 文件已存在，生成唯一文件名
					newName := generateUniqueFilename(baseName)
					fileRenameMap[baseName] = newName
					finalFilename = newName
					log.Printf("文件已存在，重命名为: %s", newName)
				}

				relativePath = finalFilename
				targetPath = filepath.Join(userPath, finalFilename)
			}

			// 创建必要的目录
			targetDir := filepath.Dir(targetPath)
			if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
				log.Printf("无法创建目录 %s: %v", targetDir, err)
				continue
			}

			// 创建并写入文件
			out, err := os.Create(targetPath)
			if err != nil {
				log.Printf("无法创建文件 %s: %v", targetPath, err)
				continue
			}

			if _, err := io.Copy(out, file); err != nil {
				log.Printf("文件保存失败 %s: %v", targetPath, err)
				continue
			}

			err = file.Close()
			if err != nil {
				return
			}
			err = out.Close()
			if err != nil {
				return
			}

			// 保存元数据
			meta := FileMeta{
				Filename: finalFilename, // 使用最终文件名
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
			err = metaFile.Close()
			if err != nil {
				return
			}

			uploadedFiles = append(uploadedFiles, relativePath)

			// 添加日志以便调试
			log.Printf("文件保存成功: %s", targetPath)
			log.Printf("最终文件名: %s", finalFilename)
		}

		if len(uploadedFiles) > 0 {
			var url string
			if folder != "" {
				// 文件夹访问URL - 包含用户名
				url = fmt.Sprintf("/%s/%s", userDir, folder)
			} else if len(uploadedFiles) > 0 {
				// 单个文件访问URL - 包含用户名
				// 使用最终文件名而不是原始文件名
				finalFilename := filepath.Base(uploadedFiles[0])
				url = fmt.Sprintf("/%s/%s", userDir, finalFilename)
			}

			responseData := gin.H{
				"url":    url,
				"expiry": expiry.Format("2006-01-02 15:04:05"),
				"folder": folder,
			}

			// 添加重命名信息
			if len(fileRenameMap) > 0 || originalFolder != folder {
				renamedInfo := gin.H{}

				// 添加文件重命名信息
				for orig, newName := range fileRenameMap {
					renamedInfo[orig] = newName
				}

				// 添加文件夹重命名信息
				if originalFolder != folder {
					renamedInfo[originalFolder] = folder
				}

				responseData["renamed"] = renamedInfo
			}

			c.JSON(http.StatusOK, responseData)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "文件保存失败"})
		}
	})

	// 文件访问路由
	r.GET("/:username/*filepath", func(c *gin.Context) {
		username := c.Param("username")
		filePath := c.Param("filepath")

		// 验证用户名是否存在
		if _, exists := credentials[username]; !exists {
			c.String(http.StatusNotFound, "用户不存在")
			return
		}

		filePath = strings.Trim(filePath, "/")
		fullPath := filepath.Join(uploadPath, username, filePath)

		// 安全检查
		if strings.Contains(filePath, "..") || strings.Contains(filePath, "//") {
			c.String(http.StatusForbidden, "无效的文件路径")
			return
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			c.String(http.StatusNotFound, "文件不存在")
			return
		}

		// 处理目录请求
		if info.IsDir() {
			// 确保URL以斜杠结尾
			if !strings.HasSuffix(c.Request.URL.Path, "/") {
				c.Redirect(http.StatusMovedPermanently, c.Request.URL.Path+"/")
				return
			}

			// 检查是否是用户根目录
			isRoot := strings.Trim(filePath, "/") == ""

			// 非根目录才检查默认文件
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

			// 没有默认文件，生成目录列表
			generateDirectoryListing(c, fullPath, username+"/"+filePath)
			return
		}

		processFileAccess(c, fullPath)
	})

	r.GET("/logout", func(c *gin.Context) {
		c.SetCookie("session_token", "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, "/login")
	})

	go cleanupExpiredFiles()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("服务启动: http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("服务器启动失败:", err)
	}
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
		defer func(metaFile *os.File) {
			err := metaFile.Close()
			if err != nil {

			}
		}(metaFile)

		var meta FileMeta
		if err := json.NewDecoder(metaFile).Decode(&meta); err != nil {
			c.String(http.StatusInternalServerError, "元数据解析错误")
			return
		}

		if !meta.Expiry.IsZero() && time.Now().After(meta.Expiry) {
			err := os.Remove(filePath)
			if err != nil {
				return
			}
			err = os.Remove(metaPath)
			if err != nil {
				return
			}
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

func cleanupExpiredFiles() {
	for {
		err := filepath.Walk(uploadPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && strings.HasSuffix(path, ".meta") {
				f, err := os.Open(path)
				if err != nil {
					return nil
				}

				var meta FileMeta
				if err := json.NewDecoder(f).Decode(&meta); err != nil {
					err := f.Close()
					if err != nil {
						return err
					}
					return nil
				}
				err = f.Close()
				if err != nil {
					return err
				}

				if !meta.Expiry.IsZero() && time.Now().After(meta.Expiry) {
					filePath := strings.TrimSuffix(path, ".meta")
					err := os.Remove(filePath)
					if err != nil {
						return err
					}
					err = os.Remove(path)
					if err != nil {
						return err
					}
					log.Printf("已清理过期文件: %s", meta.Filename)
				}
			}
			return nil
		})

		if err != nil {
			log.Printf("清理文件时出错: %v", err)
		}

		time.Sleep(5 * time.Minute)
	}
}

// 生成目录列表
func generateDirectoryListing(c *gin.Context, dirPath string, urlPath string) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		c.String(http.StatusInternalServerError, "无法读取目录")
		return
	}

	// 准备目录项
	type DirItem struct {
		Name    string
		Size    string
		IsDir   bool
		URL     string
		ModTime string
	}

	var dirItems []DirItem

	// 1. 处理URL路径
	urlPath = strings.Trim(urlPath, "/")

	pathParts := strings.Split(strings.Trim(urlPath, "/"), "/")
	username := pathParts[0]

	// 2. 构造基础URL
	baseURL := "/" + strings.Trim(urlPath, "/")
	if baseURL != "/" {
		baseURL += "/"
	}

	// 3. 处理文件和目录
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".meta") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		item := DirItem{
			Name:    file.Name(),
			IsDir:   file.IsDir(),
			URL:     strings.TrimSuffix(baseURL, "/") + "/" + file.Name(), // 确保正确拼接
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		}

		if file.IsDir() {
			item.Size = "文件夹"
			// 修复2：确保目录URL结尾只有一个斜杠
			item.URL = strings.TrimSuffix(item.URL, "/") + "/"
		} else {
			item.Size = formatFileSize(info.Size())
			// 修复3：确保文件URL没有结尾斜杠
			item.URL = strings.TrimSuffix(item.URL, "/")
		}

		dirItems = append(dirItems, item)
	}

	// 4. 排序：文件夹在前，文件在后
	sort.Slice(dirItems, func(i, j int) bool {
		if dirItems[i].IsDir && !dirItems[j].IsDir {
			return true
		}
		if !dirItems[i].IsDir && dirItems[j].IsDir {
			return false
		}
		return dirItems[i].Name < dirItems[j].Name
	})

	// 5. 准备面包屑导航
	breadcrumbs := []struct {
		Name string
		URL  string
	}{
		{Name: "根目录", URL: "/"},
		{Name: username, URL: "/" + username + "/"},
	}

	currentPath := username + "/"
	for _, part := range pathParts[1:] {
		currentPath += part + "/"
		breadcrumbs = append(breadcrumbs, struct {
			Name string
			URL  string
		}{
			Name: part,
			URL:  "/" + currentPath,
		})
	}

	// 使用HTML模板呈现目录列表
	tmplContent := `<!DOCTYPE html>
<html>
<head>
    <title>目录列表 - {{.Path}}</title>
    <meta charset="utf-8">
    <base href="/"> 
    <link rel="icon" href="/static/ico.svg" type="image/x-icon">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css">
<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		h1 { color: #333; margin-bottom: 20px; padding-bottom: 10px; border-bottom: 1px solid #eee; }
		table { width: 100%; border-collapse: collapse; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
		th, td { padding: 12px 15px; text-align: left; border-bottom: 1px solid #ddd; }
		th { background-color: #f8f9fa; font-weight: 600; }
		a { text-decoration: none; color: #0366d6; }
		a:hover { text-decoration: underline; }
		.dir-icon { color: #ffc107; margin-right: 8px; }
		.file-icon { color: #6c757d; margin-right: 8px; }
		tr:hover { background-color: #f8f9fa; }
		.size-col { width: 120px; }
		.type-col { width: 100px; }
		.time-col { width: 180px; }
		.breadcrumb { margin-bottom: 20px; font-size: 14px; color: #6c757d; }
		.breadcrumb a { color: #0366d6; }
		.breadcrumb span { color: #495057; }
		.breadcrumb .separator { margin: 0 5px; }
	</style>
</head>
<body>
	<div class="breadcrumb">
		{{range $index, $crumb := .Breadcrumbs}}
			{{if gt $index 0}}<span class="separator">/</span>{{end}}
			<a href="{{$crumb.URL}}">{{$crumb.Name}}</a>
		{{end}}
	</div>
	
	<h1><i class="fas fa-folder-open"></i> /{{.Path}}</h1>
	
	{{if .Items}}
	<table>
		<thead>
			<tr>
				<th>名称</th>
				<th class="type-col">类型</th>
				<th class="size-col">大小</th>
				<th class="time-col">修改时间</th>
			</tr>
		</thead>
		<tbody>
			{{range .Items}}
			<tr>
				<td>

					{{if .IsDir}}
						<i class="fas fa-folder dir-icon"></i>
					{{else}}
						<i class="fas fa-file file-icon"></i>
					{{end}}
					<a href="{{.URL}}">{{.Name}}{{if .IsDir}}/{{end}}</a>
				</td>
				<td>{{if .IsDir}}文件夹{{else}}文件{{end}}</td>
				<td>{{.Size}}</td>
				<td>{{.ModTime}}</td>
			</tr>
			{{end}}
		</tbody>
	</table>
	{{else}}
	<div style="text-align: center; padding: 40px; color: #6c757d;">
		<i class="fas fa-folder-open" style="font-size: 48px;"></i>
		<p style="margin-top: 20px; font-size: 18px;">此文件夹为空</p>
	</div>
	{{end}}
</body>
</html>
`

	// 创建模板
	tmpl, err := template.New("directory").Parse(tmplContent)
	if err != nil {
		c.String(http.StatusInternalServerError, "模板解析失败: "+err.Error())
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	err = tmpl.Execute(c.Writer, gin.H{
		"Path":        urlPath,
		"Items":       dirItems,
		"Breadcrumbs": breadcrumbs,
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "无法生成目录列表: "+err.Error())
	}
}

// 格式化文件大小
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
