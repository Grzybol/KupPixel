package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gin "github.com/gin-gonic/gin"

	"github.com/example/kup-piksel/internal/storage/sqlite"
	"golang.org/x/crypto/bcrypt"
)

//go:embed frontend_dist/*
var frontendFS embed.FS

type UpdatePixelRequest struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	Color  string `json:"color"`
	URL    string `json:"url"`
}

type Server struct {
	store    *sqlite.Store
	sessions *SessionManager
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]int64
}

func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]int64)}
}

func (m *SessionManager) Create(userID int64) (string, error) {
	if userID <= 0 {
		return "", errors.New("invalid user id")
	}

	for i := 0; i < 5; i++ {
		id, err := generateSessionID()
		if err != nil {
			return "", err
		}

		m.mu.Lock()
		if _, exists := m.sessions[id]; exists {
			m.mu.Unlock()
			continue
		}
		m.sessions[id] = userID
		m.mu.Unlock()
		return id, nil
	}

	return "", errors.New("failed to generate unique session id")
}

func (m *SessionManager) Get(id string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	userID, ok := m.sessions[id]
	return userID, ok
}

func (m *SessionManager) Delete(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

const (
	defaultDBPath       = "data/pixels.db"
	sessionCookieName   = "kup_pixel_session"
	sessionCookieMaxAge = 7 * 24 * 60 * 60
)

func generateSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func setSessionCookie(c *gin.Context, sessionID string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   sessionCookieMaxAge,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
}

func readSessionCookie(r *http.Request) (string, bool, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", false, nil
		}
		return "", false, err
	}
	if cookie.Value == "" {
		return "", false, nil
	}
	return cookie.Value, true, nil
}

func sanitizeUser(user sqlite.User) userResponse {
	return userResponse{ID: user.ID, Email: user.Email}
}

func (s *Server) getSessionUser(c *gin.Context) (sqlite.User, string, bool) {
	sessionID, ok, err := readSessionCookie(c.Request)
	if err != nil {
		log.Printf("read session cookie: %v", err)
		return sqlite.User{}, "", false
	}
	if !ok {
		return sqlite.User{}, "", false
	}

	userID, exists := s.sessions.Get(sessionID)
	if !exists {
		return sqlite.User{}, sessionID, false
	}

	user, err := s.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlite.User{}, sessionID, false
		}
		log.Printf("load user %d: %v", userID, err)
		return sqlite.User{}, sessionID, false
	}

	return user, sessionID, true
}

func (s *Server) requireUser(c *gin.Context) (sqlite.User, bool) {
	user, sessionID, ok := s.getSessionUser(c)
	if !ok {
		if sessionID != "" {
			s.sessions.Delete(sessionID)
			clearSessionCookie(c)
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return sqlite.User{}, false
	}
	return user, true
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
	server := &Server{store: store, sessions: NewSessionManager()}

	router.POST("/api/register", server.handleRegister)
	router.POST("/api/login", server.handleLogin)
	router.POST("/api/logout", server.handleLogout)
	router.GET("/api/session", server.handleSession)

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

func (s *Server) handleRegister(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := strings.TrimSpace(req.Password)
	if email == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("hash password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	user, err := s.store.CreateUser(c.Request.Context(), email, string(hash))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "email already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
			return
		}
		log.Printf("create user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	sessionID, err := s.sessions.Create(user.ID)
	if err != nil {
		log.Printf("create session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	setSessionCookie(c, sessionID)

	c.JSON(http.StatusCreated, gin.H{"user": sanitizeUser(user)})
}

func (s *Server) handleLogin(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := strings.TrimSpace(req.Password)
	if email == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	user, err := s.store.GetUserByEmail(c.Request.Context(), email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		log.Printf("get user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	sessionID, err := s.sessions.Create(user.ID)
	if err != nil {
		log.Printf("create session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	setSessionCookie(c, sessionID)

	c.JSON(http.StatusOK, gin.H{"user": sanitizeUser(user)})
}

func (s *Server) handleLogout(c *gin.Context) {
	sessionID, ok, err := readSessionCookie(c.Request)
	if err != nil {
		log.Printf("read session cookie: %v", err)
	}
	if ok {
		s.sessions.Delete(sessionID)
	}
	clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

func (s *Server) handleSession(c *gin.Context) {
	user, sessionID, ok := s.getSessionUser(c)
	if !ok {
		if sessionID != "" {
			s.sessions.Delete(sessionID)
			clearSessionCookie(c)
		}
		c.JSON(http.StatusOK, gin.H{"user": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": sanitizeUser(user)})
}

func (s *Server) handleUpdatePixel(c *gin.Context) {
	user, ok := s.requireUser(c)
	if !ok {
		return
	}

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

	updated, err := s.store.UpdatePixelForUser(c.Request.Context(), user.ID, sqlite.Pixel{
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
		if errors.Is(err, sqlite.ErrPixelOwnedByAnotherUser) {
			c.JSON(http.StatusForbidden, gin.H{"error": "pixel already owned"})
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
