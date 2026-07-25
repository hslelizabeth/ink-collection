package main

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:web/dist
var webFS embed.FS

// registerWeb 提供前端 SPA 静态服务与 fallback。
// 若 WEB_DIR 环境变量有值则从磁盘目录提供，否则使用内嵌产物。
func registerWeb(r *gin.Engine, webDir string) {
	var fsys http.FileSystem
	var indexBytes []byte

	if webDir != "" {
		fsys = http.Dir(webDir)
	} else {
		sub, err := fs.Sub(webFS, "web/dist")
		if err != nil {
			panic(err)
		}
		fsys = http.FS(sub)
	}

	readIndex := func() []byte {
		if indexBytes != nil {
			return indexBytes
		}
		f, err := fsys.Open("index.html")
		if err != nil {
			return nil
		}
		defer f.Close()
		buf := make([]byte, 0, 4096)
		tmp := make([]byte, 4096)
		for {
			n, err := f.Read(tmp)
			buf = append(buf, tmp[:n]...)
			if err != nil {
				break
			}
		}
		indexBytes = buf
		return buf
	}

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/uploads/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// 尝试按静态文件提供
		clean := strings.TrimPrefix(path.Clean("/"+p), "/")
		if clean != "" && clean != "." && !strings.Contains(clean, "..") {
			if f, err := fsys.Open(clean); err == nil {
				st, err := f.Stat()
				f.Close()
				if err == nil && !st.IsDir() {
					if strings.HasPrefix(clean, "assets/") {
						c.Header("Cache-Control", "public, max-age=31536000, immutable")
					}
					c.FileFromFS(clean, fsys)
					return
				}
			}
		}

		// SPA fallback
		if webDir != "" {
			c.File(path.Join(webDir, "index.html"))
			return
		}
		data := readIndex()
		if data == nil {
			c.String(http.StatusNotFound, "frontend not built")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
}
