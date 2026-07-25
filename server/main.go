package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type Config struct {
	Port    string
	DataDir string
	WebDir  string
}

func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	return Config{
		Port:    port,
		DataDir: dataDir,
		WebDir:  os.Getenv("WEB_DIR"),
	}
}

type Server struct {
	cfg Config
	db  *DB
}

func main() {
	cfg := loadConfig()
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "uploads", "thumbs"), 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	db, err := OpenDB(filepath.Join(cfg.DataDir, "collection.db"))
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := db.Seed(); err != nil {
		log.Fatalf("seed: %v", err)
	}

	s := &Server{cfg: cfg, db: db}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.MaxMultipartMemory = 32 << 20

	api := r.Group("/api")
	{
		api.GET("/categories", s.listCategories)
		api.POST("/categories", s.createCategory)
		api.PUT("/categories/:id", s.updateCategory)
		api.DELETE("/categories/:id", s.deleteCategory)

		api.GET("/items", s.listItems)
		api.GET("/items/:id", s.getItem)
		api.POST("/items", s.createItem)
		api.PUT("/items/:id", s.updateItem)
		api.DELETE("/items/:id", s.deleteItem)

		api.POST("/items/:id/images", s.uploadImages)
		api.PUT("/items/:id/cover", s.setCover)
		api.DELETE("/images/:id", s.deleteImage)

		api.GET("/filters", s.getFilters)
		api.GET("/stats", s.getStats)
		api.GET("/backup", s.backup)
	}

	r.StaticFS("/uploads", http.Dir(filepath.Join(cfg.DataDir, "uploads")))

	registerWeb(r, cfg.WebDir)

	log.Printf("listening on :%s, data dir: %s", cfg.Port, cfg.DataDir)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
