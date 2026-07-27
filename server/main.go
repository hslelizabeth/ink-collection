package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type Config struct {
	Port                 string
	DataDir              string
	WebDir               string
	AdminInitialPassword string
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
		Port:                 port,
		DataDir:              dataDir,
		WebDir:               os.Getenv("WEB_DIR"),
		AdminInitialPassword: os.Getenv("ADMIN_INITIAL_PASSWORD"),
	}
}

type Server struct {
	cfg  Config
	db   *DB
	auth *authState
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
	if err := initializeAdminPassword(db, cfg.AdminInitialPassword); err != nil {
		log.Fatalf("initialize admin access: %v", err)
	}

	s := &Server{cfg: cfg, db: db, auth: newAuthState()}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Logger(), gin.Recovery())
	r.MaxMultipartMemory = 32 << 20

	api := r.Group("/api")
	{
		api.GET("/auth/status", s.authStatus)
		api.POST("/auth/login", s.login)
		api.POST("/auth/logout", s.logout)

		api.GET("/categories", s.listCategories)
		api.GET("/items", s.listItems)
		api.GET("/items/:id", s.getItem)
		api.GET("/filters", s.getFilters)
		api.GET("/stats", s.getStats)

		admin := api.Group("")
		admin.Use(s.requireAdmin)
		{
			admin.POST("/auth/password", s.changeAdminPassword)

			admin.POST("/categories", s.createCategory)
			admin.PUT("/categories/:id", s.updateCategory)
			admin.DELETE("/categories/:id", s.deleteCategory)

			admin.POST("/items", s.createItem)
			admin.PUT("/items/:id", s.updateItem)
			admin.DELETE("/items/:id", s.deleteItem)

			admin.POST("/items/:id/images", s.uploadImages)
			admin.PUT("/items/:id/cover", s.setCover)
			admin.DELETE("/images/:id", s.deleteImage)
			admin.GET("/backup", s.backup)
		}
	}

	r.StaticFS("/uploads", http.Dir(filepath.Join(cfg.DataDir, "uploads")))

	registerWeb(r, cfg.WebDir)

	log.Printf("listening on :%s, data dir: %s", cfg.Port, cfg.DataDir)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
