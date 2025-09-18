package main

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gin "github.com/gin-gonic/gin"
)

//go:embed frontend_dist/*
var frontendFS embed.FS

const (
	gridWidth  = 1000
	gridHeight = 1000
)

type Pixel struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	Color  string `json:"color,omitempty"`
	URL    string `json:"url,omitempty"`
}

type PixelState struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Pixels []Pixel `json:"pixels"`
}

type UpdatePixelRequest struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	Color  string `json:"color"`
	URL    string `json:"url"`
}

var (
	pixels     []Pixel
	pixelsLock sync.RWMutex
)

func main() {
	initPixels()

	router := gin.Default()

	router.GET("/api/pixels", handleGetPixels)
	router.POST("/api/pixels", handleUpdatePixel)

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

func initPixels() {
	total := gridWidth * gridHeight
	pixels = make([]Pixel, total)
	for i := 0; i < total; i++ {
		pixels[i] = Pixel{ID: i, Status: "free"}
	}

	demo := []struct {
		ID    int
		Color string
		URL   string
	}{
		{ID: 500500, Color: "#ff4d4f", URL: "https://example.com"},
		{ID: 250250, Color: "#36cfc9", URL: "https://minecraft.net"},
		{ID: 750750, Color: "#722ed1", URL: "https://github.com"},
	}

	for _, d := range demo {
		if d.ID >= 0 && d.ID < total {
			pixels[d.ID].Status = "taken"
			pixels[d.ID].Color = d.Color
			pixels[d.ID].URL = d.URL
		}
	}
}

func handleGetPixels(c *gin.Context) {
	pixelsLock.RLock()
	defer pixelsLock.RUnlock()

	response := PixelState{
		Width:  gridWidth,
		Height: gridHeight,
		Pixels: pixels,
	}
	c.JSON(http.StatusOK, response)
}

func handleUpdatePixel(c *gin.Context) {
	var req UpdatePixelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.ID < 0 || req.ID >= len(pixels) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pixel id"})
		return
	}

	pixelsLock.Lock()
	defer pixelsLock.Unlock()

	target := &pixels[req.ID]
	if strings.ToLower(req.Status) == "taken" {
		if req.Color == "" || req.URL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "taken pixels require color and url"})
			return
		}
		target.Status = "taken"
		target.Color = req.Color
		target.URL = req.URL
	} else {
		target.Status = "free"
		target.Color = ""
		target.URL = ""
	}

	c.JSON(http.StatusOK, target)
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
