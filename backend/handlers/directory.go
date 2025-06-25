package handlers

import (
	"Fus/backend/utils"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

type DirItem struct {
	Name    string
	Size    string
	IsDir   bool
	URL     string
	ModTime string
}

func generateDirectoryListing(c *gin.Context, dirPath string, urlPath string) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		c.String(http.StatusInternalServerError, "无法读取目录")
		return
	}

	urlPath = strings.Trim(urlPath, "/")
	pathParts := strings.Split(urlPath, "/")
	username := pathParts[0]

	baseURL := "/" + urlPath
	if baseURL != "/" {
		baseURL += "/"
	}

	var dirItems []DirItem
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
			URL:     baseURL + file.Name(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		}

		if file.IsDir() {
			item.Size = "文件夹"
			item.URL += "/"
		} else {
			item.Size = utils.FormatFileSize(info.Size())
		}

		dirItems = append(dirItems, item)
	}

	sort.Slice(dirItems, func(i, j int) bool {
		if dirItems[i].IsDir && !dirItems[j].IsDir {
			return true
		}
		if !dirItems[i].IsDir && dirItems[j].IsDir {
			return false
		}
		return dirItems[i].Name < dirItems[j].Name
	})

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
