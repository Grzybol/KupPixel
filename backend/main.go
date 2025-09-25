package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	gin "github.com/gin-gonic/gin"

	"github.com/example/kup-piksel/internal/config"
	"github.com/example/kup-piksel/internal/email"
	"github.com/example/kup-piksel/internal/logger/elastic"
	"github.com/example/kup-piksel/internal/storage"
	"github.com/example/kup-piksel/internal/storage/mysql"
	"github.com/example/kup-piksel/internal/storage/sqlite"
	"golang.org/x/crypto/bcrypt"
)

//go:embed frontend_dist/*
var frontendFS embed.FS

type PixelUpdate struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	Color  string `json:"color"`
	URL    string `json:"url"`
}

type UpdatePixelRequest struct {
	Pixels []PixelUpdate `json:"pixels"`
}

type PixelUpdateResult struct {
	ID    int            `json:"id"`
	Pixel *storage.Pixel `json:"pixel,omitempty"`
	Error string         `json:"error,omitempty"`
}

type Server struct {
	store                    storage.Store
	sessions                 *SessionManager
	mailer                   email.Mailer
	verificationBaseURL      string
	verificationTokenTTL     time.Duration
	passwordResetBaseURL     string
	passwordResetTokenTTL    time.Duration
	disableVerificationEmail bool
	pixelCostPoints          int64
	turnstileSecret          string
	turnstileVerify          turnstileVerifier
	logger                   *elastic.Logger
	stdlog                   *log.Logger
	pixelURLBlacklist        []string
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]int64
}

type turnstileResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

type turnstileVerifier func(ctx context.Context, secret, token, remoteIP string) (turnstileResponse, error)

type logFields map[string]any

const turnstileRequestIDKey = "turnstile.request_id"

type contextKey string

var turnstileRequestIDContextKey = contextKey(turnstileRequestIDKey)

func (s *Server) logWithFields(level, message string, fields logFields) {
	if s == nil {
		return
	}
	normalized := strings.ToUpper(strings.TrimSpace(level))
	if normalized == "" {
		normalized = "INFO"
	}
	formatted := formatLogFields(fields)
	if s.stdlog != nil {
		if formatted != "" {
			s.stdlog.Printf("[%s] %s | %s", normalized, message, formatted)
		} else {
			s.stdlog.Printf("[%s] %s", normalized, message)
		}
	} else {
		if formatted != "" {
			log.Printf("[%s] %s | %s", normalized, message, formatted)
		} else {
			log.Printf("[%s] %s", normalized, message)
		}
	}
	if s.logger != nil {
		var payload elastic.Fields
		if len(fields) > 0 {
			payload = make(elastic.Fields, len(fields))
			for k, v := range fields {
				if k == "" || v == nil {
					continue
				}
				payload[k] = v
			}
		}
		s.logger.Log(normalized, message, payload)
	}
}

func (s *Server) logf(level, format string, args ...interface{}) {
	s.logWithFields(level, fmt.Sprintf(format, args...), nil)
}

func (s *Server) logInfof(format string, args ...interface{}) {
	s.logf("info", format, args...)
}

func (s *Server) logWarnf(format string, args ...interface{}) {
	s.logf("warn", format, args...)
}

func (s *Server) logErrorf(format string, args ...interface{}) {
	s.logf("error", format, args...)
}

func formatLogFields(fields logFields) string {
	if len(fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(fields))
	for key, value := range fields {
		if value == nil {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, fields[key]))
	}
	return strings.Join(parts, " ")
}

func ensureTurnstileRequestID(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	ctx := c.Request.Context()
	if existing := ctx.Value(turnstileRequestIDContextKey); existing != nil {
		if id, ok := existing.(string); ok && id != "" {
			return id
		}
	}
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	id := hex.EncodeToString(buf)
	c.Request = c.Request.WithContext(context.WithValue(ctx, turnstileRequestIDContextKey, id))
	return id
}

func (s *Server) logTurnstileEvent(c *gin.Context, level, stage, message string, fields logFields) {
	data := make(logFields, len(fields)+6)
	for k, v := range fields {
		if k == "" || v == nil {
			continue
		}
		data[k] = v
	}
	if stage != "" {
		data["turnstile_stage"] = stage
	}
	if _, ok := data["turnstile_source"]; !ok {
		data["turnstile_source"] = "backend"
	}
	if c != nil && c.Request != nil {
		if ip := extractRemoteIP(c.Request); ip != "" {
			data["remote_ip"] = ip
		}
		if c.Request.URL != nil {
			data["path"] = c.Request.URL.Path
		}
		if ua := strings.TrimSpace(c.Request.UserAgent()); ua != "" {
			data["user_agent"] = ua
		}
	}
	if id := ensureTurnstileRequestID(c); id != "" {
		data["turnstile_request_id"] = id
	}
	s.logWithFields(level, message, data)
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
	Token    string `json:"turnstile_token"`
}

type passwordResetRequest struct {
	Email string `json:"email"`
	Token string `json:"turnstile_token"`
}

type passwordResetConfirmRequest struct {
	Token           string `json:"token"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	TurnstileToken  string `json:"turnstile_token"`
}

type activationCodeRequest struct {
	Code  string `json:"code"`
	Token string `json:"turnstile_token"`
}

type turnstileDebugRequest struct {
	Stage  string         `json:"stage"`
	Status string         `json:"status"`
	Detail any            `json:"detail"`
	Error  string         `json:"error"`
	Meta   map[string]any `json:"meta"`
}

type userResponse struct {
	ID         int64      `json:"id"`
	Email      string     `json:"email"`
	IsVerified bool       `json:"is_verified"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	Points     int64      `json:"points"`
}

const (
	defaultDBPath              = "data/pixels_new.db"
	sessionCookieName          = "kup_pixel_session"
	sessionCookieMaxAge        = 7 * 24 * 60 * 60
	defaultVerificationBaseURL = "http://localhost:3000"
	defaultVerificationTTL     = 24 * time.Hour
	defaultConfigPath          = "config.json"
	turnstileVerifyURL         = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
)

var activationCodePattern = regexp.MustCompile(`^[A-Z0-9]{4}(?:-[A-Z0-9]{4}){3}$`)

var turnstileHTTPClient = &http.Client{Timeout: 10 * time.Second}

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

func defaultTurnstileVerifier(ctx context.Context, secret, token, remoteIP string) (turnstileResponse, error) {
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return turnstileResponse{}, fmt.Errorf("create turnstile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := turnstileHTTPClient.Do(req)
	if err != nil {
		return turnstileResponse{}, fmt.Errorf("execute turnstile request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return turnstileResponse{}, fmt.Errorf("turnstile verification status %d", resp.StatusCode)
	}

	var result turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return turnstileResponse{}, fmt.Errorf("decode turnstile response: %w", err)
	}

	return result, nil
}

func sanitizeUser(user storage.User) userResponse {
	return userResponse{
		ID:         user.ID,
		Email:      user.Email,
		IsVerified: user.IsVerified,
		VerifiedAt: user.VerifiedAt,
		Points:     user.Points,
	}
}

func resolveSQLitePath(cfg *config.Config) string {
	if cfg != nil && cfg.Database != nil {
		if path := strings.TrimSpace(cfg.Database.SQLitePath); path != "" {
			return path
		}
	}
	return ""
}

func selectMySQLDSN(cfg *config.DatabaseConfig) string {
	if cfg == nil || cfg.MySQL == nil {
		return ""
	}
	if env := os.Getenv("PIXEL_MYSQL_DSN"); env != "" {
		return strings.TrimSpace(env)
	}
	if cfg.MySQL.UseExternal && cfg.MySQL.ExternalDSN != "" {
		return cfg.MySQL.ExternalDSN
	}
	return cfg.MySQL.DSN
}

func openConfiguredStore(cfg *config.Config) (storage.Store, string, error) {
	if cfg == nil || cfg.Database == nil {
		cfg = config.Default()
	}

	switch cfg.Database.Driver {
	case "sqlite":
		dbPath := os.Getenv("PIXEL_DB_PATH")
		if strings.TrimSpace(dbPath) == "" {
			dbPath = resolveSQLitePath(cfg)
		}
		if strings.TrimSpace(dbPath) == "" {
			dbPath = defaultDBPath
		}

		if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, "", fmt.Errorf("create database directory: %w", err)
			}
		}

		store, err := sqlite.Open(dbPath)
		if err != nil {
			return nil, "", fmt.Errorf("open sqlite store: %w", err)
		}
		return store, fmt.Sprintf("sqlite(path=%s)", dbPath), nil
	case "mysql":
		dsn := selectMySQLDSN(cfg.Database)
		if strings.TrimSpace(dsn) == "" {
			return nil, "", errors.New("mysql dsn must not be empty")
		}

		store, err := mysql.Open(dsn)
		if err != nil {
			return nil, "", fmt.Errorf("open mysql store: %w", err)
		}
		mode := "internal"
		if cfg.Database.MySQL != nil && cfg.Database.MySQL.UseExternal {
			mode = "external"
		}
		return store, fmt.Sprintf("mysql(mode=%s)", mode), nil
	default:
		return nil, "", fmt.Errorf("unsupported database driver %q", cfg.Database.Driver)
	}
}

func generateVerificationToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate verification token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func buildVerificationLink(base string, token string) (string, error) {
	trimmed := strings.TrimRight(base, "/")
	if trimmed == "" {
		trimmed = defaultVerificationBaseURL
	}
	if _, err := url.Parse(trimmed); err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	escapedToken := url.QueryEscape(token)
	return fmt.Sprintf("%s/verify?token=%s", trimmed, escapedToken), nil
}

func buildPasswordResetLink(base string, token string) (string, error) {
	trimmed := strings.TrimRight(base, "/")
	if trimmed == "" {
		trimmed = defaultVerificationBaseURL
	}
	if _, err := url.Parse(trimmed); err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	escapedToken := url.QueryEscape(token)
	return fmt.Sprintf("%s/reset-password?token=%s", trimmed, escapedToken), nil
}

func (s *Server) getSessionUser(c *gin.Context) (storage.User, string, bool) {
	sessionID, ok, err := readSessionCookie(c.Request)
	if err != nil {
		s.logWarnf("read session cookie: %v", err)
		return storage.User{}, "", false
	}
	if !ok {
		return storage.User{}, "", false
	}

	userID, exists := s.sessions.Get(sessionID)
	if !exists {
		return storage.User{}, sessionID, false
	}

	user, err := s.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storage.User{}, sessionID, false
		}
		s.logErrorf("load user %d: %v", userID, err)
		return storage.User{}, sessionID, false
	}

	return user, sessionID, true
}

func (s *Server) requireUser(c *gin.Context) (storage.User, bool) {
	user, sessionID, ok := s.getSessionUser(c)
	if !ok {
		if sessionID != "" {
			s.sessions.Delete(sessionID)
			clearSessionCookie(c)
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return storage.User{}, false
	}
	return user, true
}

func extractRemoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	proxyHeaders := []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"}
	for _, header := range proxyHeaders {
		for _, value := range r.Header.Values(header) {
			parts := strings.Split(value, ",")
			for _, part := range parts {
				candidate := strings.TrimSpace(part)
				if candidate == "" {
					continue
				}
				if ip := normalizeIP(candidate); ip != "" {
					return ip
				}
			}
		}
	}

	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if remoteAddr == "" {
		return ""
	}

	host := remoteAddr
	if splitHost, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = splitHost
	}

	return normalizeIP(host)
}

func normalizeIP(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	if ip := net.ParseIP(trimmed); isPublicIP(ip) {
		return trimmed
	}

	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		host = strings.TrimSpace(host)
		if ip := net.ParseIP(host); isPublicIP(ip) {
			return host
		}
	}

	return ""
}

func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	if ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}

	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1]&0xC0 == 0x40 { // 100.64.0.0/10 (CGNAT)
			return false
		}
	}

	return true
}

func (s *Server) requireTurnstile(c *gin.Context, token string) bool {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		s.logTurnstileEvent(c, "warn", "token.missing", "Turnstile token missing", logFields{"reason": "empty_token"})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Potwierdź, że nie jesteś robotem."})
		return false
	}

	if strings.TrimSpace(s.turnstileSecret) == "" {
		s.logTurnstileEvent(c, "error", "configuration", "Turnstile secret key missing in configuration", logFields{"reason": "missing_secret"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Weryfikacja bezpieczeństwa jest chwilowo niedostępna."})
		return false
	}

	verifier := s.turnstileVerify
	if verifier == nil {
		verifier = defaultTurnstileVerifier
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	remoteIP := extractRemoteIP(c.Request)
	s.logTurnstileEvent(c, "info", "verify.request", "Turnstile verification request", logFields{"token_length": len(trimmed)})
	result, err := verifier(ctx, s.turnstileSecret, trimmed, remoteIP)
	if err != nil {
		s.logTurnstileEvent(c, "error", "verify.transport", "Turnstile verification error", logFields{"error": err.Error(), "token_length": len(trimmed)})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Nie udało się zweryfikować zabezpieczenia. Spróbuj ponownie."})
		return false
	}
	if !result.Success {
		if len(result.ErrorCodes) > 0 {
			s.logTurnstileEvent(c, "warn", "verify.failure", "Turnstile verification failed", logFields{"error_codes": result.ErrorCodes, "token_length": len(trimmed)})
		} else {
			s.logTurnstileEvent(c, "warn", "verify.failure", "Turnstile verification failed", logFields{"token_length": len(trimmed)})
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nieprawidłowa weryfikacja CAPTCHA."})
		return false
	}
	s.logTurnstileEvent(c, "info", "verify.success", "Turnstile verification succeeded", logFields{"token_length": len(trimmed)})

	return true
}

func (s *Server) handleTurnstileDebug(c *gin.Context) {
	var req turnstileDebugRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	stage := strings.TrimSpace(req.Stage)
	if stage == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stage is required"})
		return
	}
	status := strings.TrimSpace(req.Status)
	errMsg := strings.TrimSpace(req.Error)
	fields := logFields{"turnstile_source": "frontend"}
	if status != "" {
		fields["turnstile_status"] = status
	}
	if req.Detail != nil {
		fields["turnstile_detail"] = req.Detail
	}
	if errMsg != "" {
		fields["turnstile_error"] = errMsg
	}
	if len(req.Meta) > 0 {
		fields["turnstile_meta"] = req.Meta
	}
	level := "info"
	switch strings.ToLower(status) {
	case "error", "failed", "failure":
		level = "warn"
	}
	if errMsg != "" && level == "info" {
		level = "warn"
	}
	message := fmt.Sprintf("Turnstile frontend stage: %s", stage)
	s.logTurnstileEvent(c, level, stage, message, fields)
	c.Status(http.StatusNoContent)
}

func main() {
	elasticCfg, elasticEnabled, err := elastic.FromEnv()
	if err != nil {
		log.Fatalf("elastic config: %v", err)
	}
	var elasticLogger *elastic.Logger
	if elasticEnabled {
		elasticLogger, err = elastic.NewLogger(elasticCfg)
		if err != nil {
			log.Fatalf("elastic logger: %v", err)
		}
		log.SetOutput(io.MultiWriter(os.Stderr, elasticLogger))
	}
	if elasticLogger != nil {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if closeErr := elasticLogger.Close(ctx); closeErr != nil {
				fmt.Fprintf(os.Stderr, "[ElasticLogger] close error: %v\n", closeErr)
			}
		}()
	}

	configPath := os.Getenv("PIXEL_CONFIG_PATH")
	if configPath == "" {
		configPath = defaultConfigPath
	}

	if _, err := os.Stat(configPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if dir := filepath.Dir(configPath); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					log.Fatalf("create config directory: %v", err)
				}
			}
			if err := config.WriteFile(configPath, config.Default()); err != nil {
				log.Fatalf("write default config: %v", err)
			}
		} else {
			log.Fatalf("stat config: %v", err)
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	log.Printf("loaded config from %s", configPath)

	pixelCost := cfg.PixelCostPoints
	if pixelCost <= 0 {
		pixelCost = config.Default().PixelCostPoints
	}
	log.Printf("pixel cost configured at %d points", pixelCost)

	store, storeDescription, err := openConfiguredStore(cfg)
	if err != nil {
		log.Fatalf("configure storage: %v", err)
	}
	defer func() {
		if cerr := store.Close(); cerr != nil {
			log.Printf("close store: %v", cerr)
		}
	}()
	log.Printf("storage backend: %s", storeDescription)

	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}
	seedDemoPixels(ctx, store)

	router := gin.Default()
	verificationBaseURL := strings.TrimSpace(os.Getenv("VERIFICATION_LINK_BASE_URL"))
	if verificationBaseURL == "" {
		verificationBaseURL = strings.TrimSpace(cfg.Verification.BaseURL)
	}
	if verificationBaseURL == "" {
		verificationBaseURL = defaultVerificationBaseURL
	}

	passwordResetBaseURL := strings.TrimSpace(os.Getenv("PASSWORD_RESET_LINK_BASE_URL"))
	if passwordResetBaseURL == "" {
		passwordResetBaseURL = strings.TrimSpace(cfg.PasswordReset.BaseURL)
	}
	if passwordResetBaseURL == "" {
		passwordResetBaseURL = verificationBaseURL
	}

	verificationTTL := time.Duration(cfg.Verification.TokenTTLHours) * time.Hour
	if verificationTTL <= 0 {
		verificationTTL = defaultVerificationTTL
	}

	passwordResetTTL := time.Duration(cfg.PasswordReset.TokenTTLHours) * time.Hour
	if passwordResetTTL <= 0 {
		passwordResetTTL = time.Duration(config.Default().PasswordReset.TokenTTLHours) * time.Hour
	}

	smtpConfigured := false
	var mailer email.Mailer = email.NewConsoleMailer("Kup Piksel", cfg.Email.Language)
	if cfg.SMTP != nil {
		log.Printf(
			"smtp config detected: host=%s port=%d username=%s from_email=%s from_name=%s language=%s",
			cfg.SMTP.Host,
			cfg.SMTP.Port,
			cfg.SMTP.Username,
			cfg.SMTP.FromEmail,
			cfg.SMTP.FromName,
			cfg.Email.Language,
		)
		smtpMailer, err := email.NewSMTPMailer(*cfg.SMTP, cfg.Email.Language)
		if err != nil {
			log.Printf("failed to initialise smtp mailer: %v", err)
			log.Printf("falling back to console mailer")
		} else {
			mailer = smtpMailer
			smtpConfigured = true
			log.Printf("smtp mailer enabled for %s", cfg.SMTP.Address())
		}
	} else {
		log.Printf("smtp config missing; using console mailer")
	}

	turnstileSecret := strings.TrimSpace(cfg.TurnstileSecretKey)

	server := &Server{
		store:                    store,
		sessions:                 NewSessionManager(),
		mailer:                   mailer,
		verificationBaseURL:      verificationBaseURL,
		verificationTokenTTL:     verificationTTL,
		passwordResetBaseURL:     passwordResetBaseURL,
		passwordResetTokenTTL:    passwordResetTTL,
		disableVerificationEmail: cfg.DisableVerificationEmail,
		pixelCostPoints:          int64(pixelCost),
		turnstileSecret:          turnstileSecret,
		turnstileVerify:          defaultTurnstileVerifier,
		logger:                   elasticLogger,
		stdlog:                   log.New(os.Stderr, "", log.LstdFlags),
		pixelURLBlacklist:        cfg.PixelURLBlacklist,
	}

	server.logWithFields("info", "startup config", logFields{
		"config_path":                configPath,
		"storage_backend":            storeDescription,
		"verification_base_url":      verificationBaseURL,
		"verification_ttl":           verificationTTL.String(),
		"password_reset_base_url":    passwordResetBaseURL,
		"password_reset_ttl":         passwordResetTTL.String(),
		"smtp_configured":            smtpConfigured,
		"disable_verification_email": cfg.DisableVerificationEmail,
		"pixel_cost_points":          pixelCost,
		"email_language":             cfg.Email.Language,
		"turnstile_configured":       turnstileSecret != "",
		"elastic_logging":            elasticLogger != nil,
		"pixel_url_blacklist_count":  len(server.pixelURLBlacklist),
	})

	log.Printf(
		"startup config: config_path=%s storage_backend=%s verification_base_url=%s verification_ttl=%s password_reset_base_url=%s reset_ttl=%s smtp_configured=%t disable_verification_email=%t pixel_cost_points=%d email_language=%s turnstile_configured=%t pixel_url_blacklist_count=%d",
		configPath,
		storeDescription,
		verificationBaseURL,
		verificationTTL,
		passwordResetBaseURL,
		passwordResetTTL,
		smtpConfigured,
		cfg.DisableVerificationEmail,
		pixelCost,
		cfg.Email.Language,
		turnstileSecret != "",
		len(server.pixelURLBlacklist),
	)

	router.POST("/api/register", server.handleRegister)
	router.POST("/api/login", server.handleLogin)
	router.POST("/api/logout", server.handleLogout)
	router.POST("/api/debug/turnstile", server.handleTurnstileDebug)
	router.GET("/api/session", server.handleSession)
	router.GET("/api/account", server.handleAccount)
	router.POST("/api/activation-codes/redeem", server.handleRedeemActivationCode)
	router.GET("/api/verify", server.handleVerifyAccount)
	router.POST("/api/resend-verification", server.handleResendVerification)
	router.POST("/api/password-reset/request", server.handlePasswordResetRequest)
	router.POST("/api/password-reset/confirm", server.handlePasswordResetConfirm)

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
		s.logErrorf("get pixels: %v", err)
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

	if !s.requireTurnstile(c, req.Token) {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logErrorf("hash password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	user, err := s.store.CreateUser(c.Request.Context(), email, string(hash))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "email already exists") {
			existing, getErr := s.store.GetUserByEmail(c.Request.Context(), email)
			if getErr != nil {
				if errors.Is(getErr, sql.ErrNoRows) {
					c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
					return
				}
				s.logErrorf("get user after duplicate registration: %v", getErr)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
				return
			}

			if existing.IsVerified || s.disableVerificationEmail {
				c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
				return
			}

			s.logWithFields("info", "register: existing unverified user found", logFields{"user_id": existing.ID, "email": existing.Email})

			token, issueErr := s.issueVerificationToken(c.Request.Context(), existing)
			if issueErr != nil {
				s.logErrorf("issue verification token (duplicate register): %v", issueErr)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare verification"})
				return
			}

			s.logWithFields("info", "register: issued verification token for existing user", logFields{"user_id": existing.ID})

			link, linkErr := buildVerificationLink(s.verificationBaseURL, token)
			if linkErr != nil {
				s.logErrorf("build verification link (duplicate register): %v", linkErr)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare verification"})
				return
			}

			s.logWithFields("info", "register: sending verification email (duplicate)", logFields{"email": existing.Email})

			if sendErr := s.mailer.SendVerificationEmail(c.Request.Context(), existing.Email, link); sendErr != nil {
				s.logErrorf("send verification email (duplicate register): %v", sendErr)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send verification email"})
				return
			}

			c.JSON(http.StatusAccepted, gin.H{
				"message": "Konto już istnieje. Wysłaliśmy nowy link aktywacyjny na Twój adres e-mail.",
			})
			return
		}
		s.logErrorf("create user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	s.logWithFields("info", "register: created new user", logFields{"user_id": user.ID, "email": user.Email, "disable_verification_email": s.disableVerificationEmail})

	if s.disableVerificationEmail {
		if err := s.store.MarkUserVerified(c.Request.Context(), user.ID); err != nil {
			s.logErrorf("auto-verify user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify user"})
			return
		}
		if err := s.store.DeleteVerificationTokensForUser(c.Request.Context(), user.ID); err != nil {
			s.logWarnf("cleanup verification tokens after auto verify: %v", err)
		}
		c.JSON(http.StatusCreated, gin.H{
			"message": "Konto zostało utworzone i jest już potwierdzone. Możesz się zalogować.",
		})
		return
	}

	s.logWithFields("info", "register: issuing verification token", logFields{"user_id": user.ID})

	token, err := s.issueVerificationToken(c.Request.Context(), user)
	if err != nil {
		s.logErrorf("issue verification token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare verification"})
		return
	}

	s.logWithFields("info", "register: verification token issued", logFields{"user_id": user.ID})

	link, err := buildVerificationLink(s.verificationBaseURL, token)
	if err != nil {
		s.logErrorf("build verification link: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare verification"})
		return
	}

	s.logWithFields("info", "register: sending verification email", logFields{"email": user.Email, "user_id": user.ID})

	if err := s.mailer.SendVerificationEmail(c.Request.Context(), user.Email, link); err != nil {
		s.logErrorf("send verification email: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send verification email"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Konto zostało utworzone. Sprawdź skrzynkę e-mail i potwierdź adres, aby się zalogować.",
	})
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

	if !s.requireTurnstile(c, req.Token) {
		return
	}

	user, err := s.store.GetUserByEmail(c.Request.Context(), email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		s.logErrorf("get user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if !user.IsVerified {
		if s.disableVerificationEmail {
			c.JSON(http.StatusForbidden, gin.H{"error": "konto nie zostało jeszcze potwierdzone. Sprawdź skrzynkę e-mail."})
			return
		}

		s.logWithFields("info", "login: unverified user attempting login", logFields{"user_id": user.ID, "email": user.Email})

		token, err := s.issueVerificationToken(c.Request.Context(), user)
		if err != nil {
			s.logErrorf("issue verification token (login): %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare verification"})
			return
		}

		s.logWithFields("info", "login: verification token issued", logFields{"user_id": user.ID})

		link, err := buildVerificationLink(s.verificationBaseURL, token)
		if err != nil {
			s.logErrorf("build verification link (login): %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare verification"})
			return
		}

		s.logWithFields("info", "login: sending verification email", logFields{"email": user.Email, "user_id": user.ID})

		if err := s.mailer.SendVerificationEmail(c.Request.Context(), user.Email, link); err != nil {
			s.logErrorf("send verification email (login): %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send verification email"})
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "konto nie zostało jeszcze potwierdzone. Nowy link weryfikacyjny został wysłany na adres e-mail."})
		return
	}

	sessionID, err := s.sessions.Create(user.ID)
	if err != nil {
		s.logErrorf("create session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	setSessionCookie(c, sessionID)

	c.JSON(http.StatusOK, gin.H{"user": sanitizeUser(user), "pixel_cost_points": s.pixelCostPoints})
}

func (s *Server) issueVerificationToken(ctx context.Context, user storage.User) (string, error) {
	if s.disableVerificationEmail {
		return "", errors.New("email verification disabled")
	}
	if user.ID <= 0 {
		return "", errors.New("invalid user id")
	}

	s.logWithFields("info", "issueVerificationToken: start", logFields{"user_id": user.ID})

	if err := s.store.DeleteVerificationTokensForUser(ctx, user.ID); err != nil {
		return "", err
	}
	s.logWithFields("info", "issueVerificationToken: cleared previous tokens", logFields{"user_id": user.ID})

	var token string
	var err error
	for i := 0; i < 5; i++ {
		token, err = generateVerificationToken()
		if err != nil {
			return "", err
		}
		expires := time.Now().Add(s.verificationTokenTTL)
		_, storeErr := s.store.CreateVerificationToken(ctx, token, user.ID, expires)
		if storeErr == nil {
			s.logWithFields("info", "issueVerificationToken: stored token", logFields{
				"user_id":    user.ID,
				"expires_at": expires.Format(time.RFC3339),
				"attempt":    i + 1,
			})
			return token, nil
		}
		if !strings.Contains(strings.ToLower(storeErr.Error()), "token already exists") {
			return "", storeErr
		}
	}
	s.logWithFields("error", "issueVerificationToken: failed to create unique token", logFields{"user_id": user.ID})
	return "", errors.New("unable to create unique verification token")
}

func (s *Server) issuePasswordResetToken(ctx context.Context, user storage.User) (string, error) {
	if user.ID <= 0 {
		return "", errors.New("invalid user id")
	}

	if err := s.store.DeletePasswordResetTokensForUser(ctx, user.ID); err != nil {
		s.logWarnf("cleanup password reset tokens for user_id=%d: %v", user.ID, err)
	}

	ttl := s.passwordResetTokenTTL
	if ttl <= 0 {
		ttl = time.Duration(config.Default().PasswordReset.TokenTTLHours) * time.Hour
	}

	for i := 0; i < 5; i++ {
		token, err := generateVerificationToken()
		if err != nil {
			return "", fmt.Errorf("generate reset token: %w", err)
		}
		expires := time.Now().Add(ttl)

		_, storeErr := s.store.CreatePasswordResetToken(ctx, token, user.ID, expires)
		if storeErr == nil {
			s.logWithFields("info", "issuePasswordResetToken: stored token", logFields{
				"user_id":    user.ID,
				"expires_at": expires.Format(time.RFC3339),
				"attempt":    i + 1,
			})
			return token, nil
		}
		if !strings.Contains(strings.ToLower(storeErr.Error()), "token already exists") {
			return "", storeErr
		}
	}

	s.logWithFields("error", "issuePasswordResetToken: failed to create unique token", logFields{"user_id": user.ID})
	return "", errors.New("unable to create unique password reset token")
}

func (s *Server) handleVerifyAccount(c *gin.Context) {
	if s.disableVerificationEmail {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Weryfikacja adresów e-mail jest wyłączona."})
		return
	}
	token := strings.TrimSpace(c.Request.URL.Query().Get("token"))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token"})
		return
	}

	record, err := s.store.GetVerificationToken(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "nieprawidłowy lub wykorzystany token"})
			return
		}
		s.logErrorf("get verification token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify account"})
		return
	}

	if time.Now().After(record.ExpiresAt) {
		_ = s.store.DeleteVerificationToken(c.Request.Context(), token)
		c.JSON(http.StatusBadRequest, gin.H{"error": "token wygasł. Poproś o nowy link weryfikacyjny."})
		return
	}

	if err := s.store.MarkUserVerified(c.Request.Context(), record.UserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "konto nie istnieje"})
			return
		}
		s.logErrorf("mark user verified: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify account"})
		return
	}

	if err := s.store.DeleteVerificationTokensForUser(c.Request.Context(), record.UserID); err != nil {
		s.logWarnf("cleanup verification tokens: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Adres e-mail został potwierdzony. Możesz się teraz zalogować."})
}

func (s *Server) handleResendVerification(c *gin.Context) {
	if s.disableVerificationEmail {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Konto jest już potwierdzone."})
		return
	}
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}

	if !s.requireTurnstile(c, req.Token) {
		return
	}

	user, err := s.store.GetUserByEmail(c.Request.Context(), email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "konto z tym adresem e-mail nie istnieje"})
			return
		}
		s.logErrorf("get user for resend: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process request"})
		return
	}

	if user.IsVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "konto jest już potwierdzone"})
		return
	}

	s.logWithFields("info", "resend: issuing verification token", logFields{"user_id": user.ID})

	token, err := s.issueVerificationToken(c.Request.Context(), user)
	if err != nil {
		s.logErrorf("issue verification token (resend): %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare verification"})
		return
	}

	s.logWithFields("info", "resend: verification token issued", logFields{"user_id": user.ID})

	link, err := buildVerificationLink(s.verificationBaseURL, token)
	if err != nil {
		s.logErrorf("build verification link (resend): %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare verification"})
		return
	}

	s.logWithFields("info", "resend: sending verification email", logFields{"email": user.Email, "user_id": user.ID})

	if err := s.mailer.SendVerificationEmail(c.Request.Context(), user.Email, link); err != nil {
		s.logErrorf("send verification email (resend): %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send verification email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Nowy link weryfikacyjny został wysłany."})
}

func (s *Server) handlePasswordResetRequest(c *gin.Context) {
	var req passwordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}

	if !s.requireTurnstile(c, req.Token) {
		return
	}

	const responseMessage = "Jeśli konto istnieje, wysłaliśmy instrukcje resetu hasła."

	user, err := s.store.GetUserByEmail(c.Request.Context(), email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusAccepted, gin.H{"message": responseMessage})
			return
		}
		s.logErrorf("get user for password reset: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process request"})
		return
	}

	token, err := s.issuePasswordResetToken(c.Request.Context(), user)
	if err != nil {
		s.logErrorf("issue password reset token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare reset"})
		return
	}

	link, err := buildPasswordResetLink(s.passwordResetBaseURL, token)
	if err != nil {
		s.logErrorf("build password reset link: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare reset"})
		return
	}

	if err := s.mailer.SendPasswordResetEmail(c.Request.Context(), user.Email, link); err != nil {
		s.logErrorf("send password reset email: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send reset email"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": responseMessage})
}

func (s *Server) handlePasswordResetConfirm(c *gin.Context) {
	var req passwordResetConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	token := strings.TrimSpace(req.Token)
	password := strings.TrimSpace(req.Password)
	confirm := strings.TrimSpace(req.ConfirmPassword)
	if token == "" || password == "" || confirm == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token and password are required"})
		return
	}

	if password != confirm {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hasła muszą być takie same"})
		return
	}

	if !s.requireTurnstile(c, req.TurnstileToken) {
		return
	}

	record, err := s.store.GetPasswordResetToken(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "nieprawidłowy lub wykorzystany token"})
			return
		}
		s.logErrorf("get password reset token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset password"})
		return
	}

	if time.Now().After(record.ExpiresAt) {
		if delErr := s.store.DeletePasswordResetToken(c.Request.Context(), token); delErr != nil {
			s.logWarnf("cleanup expired password reset token: %v", delErr)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "token wygasł. Poproś o nowy link resetu hasła."})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logErrorf("hash password reset: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset password"})
		return
	}

	if err := s.store.UpdateUserPassword(c.Request.Context(), record.UserID, string(hash)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "konto nie istnieje"})
			return
		}
		s.logErrorf("update user password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset password"})
		return
	}

	if err := s.store.DeletePasswordResetTokensForUser(c.Request.Context(), record.UserID); err != nil {
		s.logWarnf("cleanup password reset tokens for user_id=%d: %v", record.UserID, err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Hasło zostało zaktualizowane."})
}

func (s *Server) handleLogout(c *gin.Context) {
	sessionID, ok, err := readSessionCookie(c.Request)
	if err != nil {
		s.logWarnf("read session cookie: %v", err)
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
		c.JSON(http.StatusOK, gin.H{"user": nil, "pixel_cost_points": s.pixelCostPoints})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": sanitizeUser(user), "pixel_cost_points": s.pixelCostPoints})
}

func (s *Server) handleAccount(c *gin.Context) {
	user, ok := s.requireUser(c)
	if !ok {
		return
	}

	pixels, err := s.store.GetPixelsByOwner(c.Request.Context(), user.ID)
	if err != nil {
		s.logErrorf("get pixels for user %d: %v", user.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load account"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":              sanitizeUser(user),
		"pixels":            pixels,
		"pixel_cost_points": s.pixelCostPoints,
	})
}

func (s *Server) handleRedeemActivationCode(c *gin.Context) {
	user, ok := s.requireUser(c)
	if !ok {
		return
	}

	var req activationCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if !activationCodePattern.MatchString(code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nieprawidłowy format kodu. Użyj xxxx-xxxx-xxxx-xxxx."})
		return
	}

	if !s.requireTurnstile(c, req.Token) {
		return
	}

	updatedUser, added, err := s.store.RedeemActivationCode(c.Request.Context(), user.ID, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "kod nie istnieje lub został już wykorzystany."})
			return
		}
		s.logWithFields("error", "redeem activation code failed", logFields{"code": code, "user_id": user.ID, "error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "nie udało się aktywować kodu"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":              sanitizeUser(updatedUser),
		"added_points":      added,
		"pixel_cost_points": s.pixelCostPoints,
	})
}

func (s *Server) isPixelURLBlocked(rawURL string) (string, bool) {
	if s == nil {
		return "", false
	}
	normalized := strings.ToLower(strings.TrimSpace(rawURL))
	if normalized == "" {
		return "", false
	}

	var host string
	if parsed, err := url.Parse(normalized); err == nil {
		host = strings.ToLower(parsed.Hostname())
	}

	for _, entry := range s.pixelURLBlacklist {
		keyword := strings.ToLower(strings.TrimSpace(entry))
		if keyword == "" {
			continue
		}
		if host != "" && (host == keyword || strings.Contains(host, keyword)) {
			return fmt.Sprintf("url contains blocked keyword or domain: %s", keyword), true
		}
		if strings.Contains(normalized, keyword) {
			return fmt.Sprintf("url contains blocked keyword or domain: %s", keyword), true
		}
	}

	return "", false
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

	if len(req.Pixels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no pixels provided"})
		return
	}

	results := make([]PixelUpdateResult, 0, len(req.Pixels))
	currentUser := user
	var anySuccess bool
	var firstErrStatus int
	var firstErrMessage string

	for _, item := range req.Pixels {
		result := PixelUpdateResult{ID: item.ID}
		if item.ID < 0 || item.ID >= storage.TotalPixels {
			result.Error = "invalid pixel id"
			if firstErrStatus == 0 {
				firstErrStatus = http.StatusBadRequest
				firstErrMessage = result.Error
			}
			results = append(results, result)
			continue
		}

		pixel := storage.Pixel{ID: item.ID}
		if strings.ToLower(item.Status) == "taken" {
			color := strings.TrimSpace(item.Color)
			url := strings.TrimSpace(item.URL)
			if color == "" || url == "" {
				result.Error = "taken pixels require color and url"
				if firstErrStatus == 0 {
					firstErrStatus = http.StatusBadRequest
					firstErrMessage = result.Error
				}
				results = append(results, result)
				continue
			}
			if message, blocked := s.isPixelURLBlocked(url); blocked {
				result.Error = message
				if firstErrStatus == 0 {
					firstErrStatus = http.StatusBadRequest
					firstErrMessage = result.Error
				}
				results = append(results, result)
				continue
			}
			pixel.Status = "taken"
			pixel.Color = color
			pixel.URL = url
		} else {
			pixel.Status = "free"
			pixel.Color = ""
			pixel.URL = ""
		}

		updatedPixel, updatedUser, err := s.store.UpdatePixelForUserWithCost(c.Request.Context(), user.ID, pixel, s.pixelCostPoints)
		if err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, sql.ErrNoRows):
				result.Error = "pixel not found"
				status = http.StatusNotFound
			case errors.Is(err, storage.ErrPixelOwnedByAnotherUser):
				result.Error = "pixel already owned"
				status = http.StatusForbidden
			case errors.Is(err, storage.ErrInsufficientPoints):
				result.Error = "brak wystarczającej liczby punktów. Aktywuj kod, aby zdobyć więcej."
				status = http.StatusForbidden
			default:
				result.Error = "failed to update pixel"
				s.logWithFields("error", "update pixel", logFields{"pixel_id": item.ID, "error": err.Error(), "user_id": user.ID})
			}

			if firstErrStatus == 0 {
				firstErrStatus = status
				firstErrMessage = result.Error
			}
			results = append(results, result)
			continue
		}

		anySuccess = true
		currentUser = updatedUser
		result.Pixel = &updatedPixel
		results = append(results, result)
	}

	if !anySuccess {
		status := firstErrStatus
		if status == 0 {
			status = http.StatusBadRequest
		}
		message := firstErrMessage
		if message == "" {
			message = "failed to update pixels"
		}
		c.JSON(status, gin.H{
			"error":             message,
			"results":           results,
			"user":              sanitizeUser(currentUser),
			"pixel_cost_points": s.pixelCostPoints,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results":           results,
		"user":              sanitizeUser(currentUser),
		"pixel_cost_points": s.pixelCostPoints,
	})
}

func seedDemoPixels(ctx context.Context, store storage.Store) {
	demo := []storage.Pixel{
		{ID: 500500, Status: "taken", Color: "#ff4d4f", URL: "https://example.com"},
		{ID: 250250, Status: "taken", Color: "#36cfc9", URL: "https://minecraft.net"},
		{ID: 750750, Status: "taken", Color: "#722ed1", URL: "https://github.com"},
	}

	for _, pixel := range demo {
		if _, err := store.UpdatePixel(ctx, pixel); err != nil {
			fmt.Fprintf(os.Stderr, "[seed] seed pixel %d: %v\n", pixel.ID, err)
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
