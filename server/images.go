package main

import (
	"crypto/rand"
	"encoding/hex"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	// 注册 webp 解码器，imaging.Decode 依赖 image.Decode 的注册表
	_ "golang.org/x/image/webp"
)

const maxUploadSize = 20 << 20 // 20MB

var allowedExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
}

func randomName(ext string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) + ext
}

func (s *Server) uploadImages(c *gin.Context) {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if it, _ := s.getItemByID(itemID); it == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "藏品不存在"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart 解析失败: " + err.Error()})
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 files 字段"})
		return
	}

	uploadsDir := filepath.Join(s.cfg.DataDir, "uploads")
	thumbsDir := filepath.Join(uploadsDir, "thumbs")

	var maxSort int
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(sort), -1) FROM images WHERE item_id = ?`, itemID).Scan(&maxSort)

	uploaded := []gin.H{}
	for _, fh := range files {
		if fh.Size > maxUploadSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "文件超过 20MB: " + fh.Filename})
			return
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if !allowedExts[ext] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 jpg/png/webp: " + fh.Filename})
			return
		}

		src, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		img, err := imaging.Decode(src, imaging.AutoOrientation(true))
		src.Close()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "图片解码失败: " + fh.Filename})
			return
		}

		filename := randomName(ext)
		dst := filepath.Join(uploadsDir, filename)
		if err := c.SaveUploadedFile(fh, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		thumb := strings.TrimSuffix(filename, ext) + ".jpg"
		t := img
		if t.Bounds().Dx() > 400 {
			t = imaging.Resize(t, 400, 0, imaging.Lanczos)
		}
		if err := imaging.Save(t, filepath.Join(thumbsDir, thumb), imaging.JPEGQuality(jpeg.DefaultQuality)); err != nil {
			os.Remove(dst)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "缩略图生成失败: " + err.Error()})
			return
		}

		maxSort++
		res, err := s.db.Exec(`INSERT INTO images (item_id, filename, thumb, sort) VALUES (?,?,?,?)`,
			itemID, filename, thumb, maxSort)
		if err != nil {
			os.Remove(dst)
			os.Remove(filepath.Join(thumbsDir, thumb))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		id, _ := res.LastInsertId()
		uploaded = append(uploaded, gin.H{
			"id":        id,
			"url":       "/uploads/" + filename,
			"thumb_url": "/uploads/thumbs/" + thumb,
			"sort":      maxSort,
		})
	}
	c.JSON(http.StatusCreated, uploaded)
}

func (s *Server) deleteImage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var filename, thumb string
	err = s.db.QueryRow(`SELECT filename, thumb FROM images WHERE id = ?`, id).Scan(&filename, &thumb)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "图片不存在"})
		return
	}
	if _, err := s.db.Exec(`DELETE FROM images WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.removeImageFiles([][2]string{{filename, thumb}})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) setCover(c *gin.Context) {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var body struct {
		ImageID int64 `json:"image_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.ImageID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 image_id"})
		return
	}
	var exists int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM images WHERE id = ? AND item_id = ?`, body.ImageID, itemID).Scan(&exists)
	if err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "图片不存在"})
		return
	}
	// 把目标图排最前：sort 置为当前最小值 - 1
	if _, err := s.db.Exec(
		`UPDATE images SET sort = (SELECT COALESCE(MIN(sort), 0) - 1 FROM images WHERE item_id = ?) WHERE id = ?`,
		itemID, body.ImageID,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) removeImageFiles(files [][2]string) {
	uploadsDir := filepath.Join(s.cfg.DataDir, "uploads")
	for _, f := range files {
		if f[0] != "" {
			os.Remove(filepath.Join(uploadsDir, filepath.Base(f[0])))
		}
		if f[1] != "" {
			os.Remove(filepath.Join(uploadsDir, "thumbs", filepath.Base(f[1])))
		}
	}
}
