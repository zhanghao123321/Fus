package cleanup

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Fus/backend/config"
)

func StartCleanup() {
	for {
		err := filepath.Walk(config.UploadPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && strings.HasSuffix(path, ".meta") {
				f, err := os.Open(path)
				if err != nil {
					return nil
				}

				var meta struct {
					Filename string    `json:"filename"`
					Expiry   time.Time `json:"expiry"`
				}
				if err := json.NewDecoder(f).Decode(&meta); err != nil {
					f.Close()
					return nil
				}
				f.Close()

				if !meta.Expiry.IsZero() && time.Now().After(meta.Expiry) {
					filePath := strings.TrimSuffix(path, ".meta")
					os.Remove(filePath)
					os.Remove(path)
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
