package main

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	gin "github.com/gin-gonic/gin"

	"github.com/example/kup-piksel/internal/storage/sqlite"
)

//go:embed frontend_dist/*
var frontendFS embed.FS

const defaultDBPath = "data/pixels.db"

type UpdatePixelRequest struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	Color  string `json:"color"`
	URL    string `json:"url"`
}

type Server struct {
	store *sqlite.Store
}

func main() {
	dbPath := os.Getenv("PIXEL_DB_PATH")
	if dbPath == "" {
		dbPath = defaultDBPath
	}

	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("create database directory: %v", err)
		}
	}

	store, err := sqlite.Open(dbPath)
	if err != nil {
		log.Fatalf("open sqlite store: %v", err)
	}
	defer func() {
		if cerr := store.Close(); cerr != nil {
			log.Printf("close store: %v", cerr)
		}
	}()

	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}
	seedDemoPixels(ctx, store)

	router := gin.Default()
	server := &Server{store: store}

	router.GET("/api/pixels", server.handleGetPixels)
	router.POST("/api/pixels", server.handleUpdatePixel)

	if assets := embedSub("frontend_dist/assets"); assets != nil {
		router.StaticFS("/assets", http.FS(assets))
	}

	router.NoRoute(func(c *gin.Context) {
		serveIndex(c)
	})

	log.Println("Kup Piksel backend listening on :3000")
	if err := router.Run(":3000"); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func (s *Server) handleGetPixels(c *gin.Context) {
	state, err := s.store.GetAllPixels(c.Request.Context())
	if err != nil {
		log.Printf("get pixels: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load pixels"})
		return
	}
	c.JSON(http.StatusOK, state)
}

func (s *Server) handleUpdatePixel(c *gin.Context) {
	var req UpdatePixelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.ID < 0 || req.ID >= sqlite.TotalPixels {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pixel id"})
		return
	}

	if strings.ToLower(req.Status) == "taken" {
		if req.Color == "" || req.URL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "taken pixels require color and url"})
			return
		}
		req.Status = "taken"
	} else {
		req.Status = "free"
		req.Color = ""
		req.URL = ""
	}

	updated, err := s.store.UpdatePixel(c.Request.Context(), sqlite.Pixel{
		ID:     req.ID,
		Status: req.Status,
		Color:  req.Color,
		URL:    req.URL,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "pixel not found"})
			return
		}
		log.Printf("update pixel %d: %v", req.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update pixel"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func seedDemoPixels(ctx context.Context, store *sqlite.Store) {
	demo := []sqlite.Pixel{
		{ID: 500500, Status: "taken", Color: "#ff4d4f", URL: "https://example.com"},
		{ID: 250250, Status: "taken", Color: "#36cfc9", URL: "https://minecraft.net"},
		{ID: 750750, Status: "taken", Color: "#722ed1", URL: "https://github.com"},
	}

	for _, pixel := range demo {
		if _, err := store.UpdatePixel(ctx, pixel); err != nil {
			log.Printf("seed pixel %d: %v", pixel.ID, err)
		}
	}
}

func serveIndex(c *gin.Context) {
	requestPath := c.Request.URL.Path
	if strings.HasPrefix(requestPath, "/api/") {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	cleaned := strings.TrimPrefix(requestPath, "/")
	if cleaned != "" {
		if file, err := frontendFS.ReadFile(filepath.Join("frontend_dist", cleaned)); err == nil {
			http.ServeContent(c.Writer, c.Request, cleaned, time.Now(), bytes.NewReader(file))
			return
		}
	}

	data, err := frontendFS.ReadFile("frontend_dist/index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("frontend build missing: %v", err))
		return
	}
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = c.Writer.Write(data)
}

func embedSub(path string) fs.FS {
	sub, err := fs.Sub(frontendFS, path)
	if err != nil {
		return nil
	}
	return sub
}
