package utils

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func GenerateUniqueFilename(original string) string {
	ext := filepath.Ext(original)
	name := strings.TrimSuffix(original, ext)
	timestamp := time.Now().Format("20060102150405")
	random := rand.Intn(10000)
	return fmt.Sprintf("%s_%s_%04d%s", name, timestamp, random, ext)
}

func GenerateUniqueFoldername(original string) string {
	timestamp := time.Now().Format("20060102150405")
	uuidPart := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%s_%s", original, timestamp, uuidPart)
}

func FormatFileSize(bytes int64) string {
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

func SanitizeUsername(username string) string {
	username = strings.ReplaceAll(username, "..", "")
	username = strings.ReplaceAll(username, "/", "")
	username = strings.ReplaceAll(username, "\\", "")
	username = strings.TrimSpace(username)
	if username == "" {
		return "anonymous"
	}
	return username
}
