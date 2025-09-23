package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/example/kup-piksel/internal/storage/sqlite"
)

func TestPasswordResetFlow(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	store.SetSkipPixelSeed(true)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("initial"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user, err := store.CreateUser(context.Background(), "user@example.com", string(hash))
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := store.MarkUserVerified(context.Background(), user.ID); err != nil {
		t.Fatalf("mark user verified: %v", err)
	}

	mailer := &fakeMailer{}
	server := &Server{
		store:                 store,
		sessions:              NewSessionManager(),
		mailer:                mailer,
		verificationBaseURL:   "http://example.com",
		verificationTokenTTL:  time.Hour,
		passwordResetBaseURL:  "http://example.com",
		passwordResetTokenTTL: time.Hour,
		pixelCostPoints:       10,
	}
	enableTurnstileForTest(server)

	body := bytes.NewBufferString(`{"email":"user@example.com","turnstile_token":"` + testTurnstileToken + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/password-reset/request", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c := &gin.Context{Writer: w, Request: req}

	server.handlePasswordResetRequest(c)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", w.Code)
	}
	if mailer.resetSent != 1 {
		t.Fatalf("expected one reset email, got %d", mailer.resetSent)
	}
	if mailer.lastResetLink == "" {
		t.Fatalf("expected reset link to be set")
	}

	parsed, err := url.Parse(mailer.lastResetLink)
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}
	token := parsed.Query().Get("token")
	if token == "" {
		t.Fatalf("expected token in reset link")
	}

	record, err := store.GetPasswordResetToken(context.Background(), token)
	if err != nil {
		t.Fatalf("get password reset token: %v", err)
	}
	if record.UserID != user.ID {
		t.Fatalf("expected token to belong to user %d, got %d", user.ID, record.UserID)
	}

	confirmBody := bytes.NewBufferString(fmt.Sprintf(`{"token":"%s","password":"new-secret","confirm_password":"new-secret","turnstile_token":"%s"}`, token, testTurnstileToken))
	confirmReq := httptest.NewRequest(http.MethodPost, "/api/password-reset/confirm", confirmBody)
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmW := httptest.NewRecorder()
	confirmCtx := &gin.Context{Writer: confirmW, Request: confirmReq}

	server.handlePasswordResetConfirm(confirmCtx)

	if confirmW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", confirmW.Code)
	}

	loginBody := bytes.NewBufferString(`{"email":"user@example.com","password":"new-secret","turnstile_token":"` + testTurnstileToken + `"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	loginCtx := &gin.Context{Writer: loginW, Request: loginReq}

	server.handleLogin(loginCtx)

	if loginW.Code != http.StatusOK {
		t.Fatalf("expected successful login with new password, got %d", loginW.Code)
	}

	oldLoginBody := bytes.NewBufferString(`{"email":"user@example.com","password":"initial","turnstile_token":"` + testTurnstileToken + `"}`)
	oldLoginReq := httptest.NewRequest(http.MethodPost, "/api/login", oldLoginBody)
	oldLoginReq.Header.Set("Content-Type", "application/json")
	oldLoginW := httptest.NewRecorder()
	oldLoginCtx := &gin.Context{Writer: oldLoginW, Request: oldLoginReq}

	server.handleLogin(oldLoginCtx)

	if oldLoginW.Code != http.StatusUnauthorized {
		t.Fatalf("expected login with old password to be rejected, got %d", oldLoginW.Code)
	}

	updated, err := store.GetUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("get user after reset: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("new-secret")); err != nil {
		t.Fatalf("expected password to be updated: %v", err)
	}

	if _, err := store.GetPasswordResetToken(context.Background(), token); err == nil {
		t.Fatalf("expected reset token to be removed")
	}
}
