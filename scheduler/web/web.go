// Package web provides embedded static web assets.
package web

import (
	"embed"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed embed.go
var embedFallback embed.FS

// GetStaticFS returns a http.FileSystem for the static files
func GetStaticFS() (http.FileSystem, error) {
	// 首先检查是否有本地构建的文件（从 scheduler 目录的 web 子目录）
	if _, err := os.Stat("web/index.html"); err == nil {
		return http.Dir("web"), nil
	}

	// 检查是否是从其他目录运行的，尝试找到正确的路径
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		webPath := filepath.Join(execDir, "web")
		if _, err := os.Stat(filepath.Join(webPath, "index.html")); err == nil {
			return http.Dir(webPath), nil
		}
	}

	// 返回 fallback FS
	return http.FS(embedFallback), nil
}
