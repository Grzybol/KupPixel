package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/example/kup-piksel/internal/storage/sqlite"
)

const (
	testLoginPassword     = "a"
	testLoginPasswordHash = "dyC+Vz8yo5yYPkBDtxnhbGF5j4W5TLuHTsHrMbAbfxd5/iF7zgTn2gxnFGTHk0pe"
)

func TestHandleLogin_UnverifiedResendsVerificationEmail(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(testLoginPasswordHash), []byte(testLoginPassword)); err != nil {
		t.Fatalf("prepare hash: %v", err)
	}

	user, err := store.CreateUser(context.Background(), "user@example.com", testLoginPasswordHash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	stored, err := store.GetUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(testLoginPassword)); err != nil {
		t.Fatalf("password hash mismatch: %v (hash=%q)", err, stored.PasswordHash)
	}

	mailer := &fakeMailer{}
	server := &Server{
		store:                store,
		sessions:             NewSessionManager(),
		mailer:               mailer,
		verificationBaseURL:  "http://example.com",
		verificationTokenTTL: time.Hour,
	}

	body := bytes.NewBufferString(`{"email":"user@example.com","password":"` + testLoginPassword + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c := &gin.Context{Writer: w, Request: req}

	server.handleLogin(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", w.Code)
	}
	if mailer.sent != 1 {
		t.Fatalf("expected one verification email to be sent, got %d", mailer.sent)
	}
	if mailer.lastRecipient != "user@example.com" {
		t.Fatalf("expected recipient to be user@example.com, got %s", mailer.lastRecipient)
	}
	if !strings.Contains(w.Body.String(), "Nowy link weryfikacyjny") {
		t.Fatalf("expected message about new verification link, got %s", w.Body.String())
	}

	parsed, err := url.Parse(mailer.lastLink)
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}
	token := parsed.Query().Get("token")
	if token == "" {
		t.Fatalf("expected token in verification link")
	}

	record, err := store.GetVerificationToken(context.Background(), token)
	if err != nil {
		t.Fatalf("get verification token: %v", err)
	}
	if record.UserID != user.ID {
		t.Fatalf("expected token to belong to user %d, got %d", user.ID, record.UserID)
	}
}

func TestHandleLogin_UnverifiedWhenVerificationDisabled(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(testLoginPasswordHash), []byte(testLoginPassword)); err != nil {
		t.Fatalf("prepare hash: %v", err)
	}

	if _, err := store.CreateUser(context.Background(), "user@example.com", testLoginPasswordHash); err != nil {
		t.Fatalf("create user: %v", err)
	}

	stored, err := store.GetUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(testLoginPassword)); err != nil {
		t.Fatalf("password hash mismatch: %v (hash=%q)", err, stored.PasswordHash)
	}

	mailer := &fakeMailer{}
	server := &Server{
		store:                    store,
		sessions:                 NewSessionManager(),
		mailer:                   mailer,
		verificationBaseURL:      "http://example.com",
		verificationTokenTTL:     time.Hour,
		disableVerificationEmail: true,
	}

	body := bytes.NewBufferString(`{"email":"user@example.com","password":"` + testLoginPassword + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c := &gin.Context{Writer: w, Request: req}

	server.handleLogin(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", w.Code)
	}
	if mailer.sent != 0 {
		t.Fatalf("expected no verification email to be sent, got %d", mailer.sent)
	}
	expected := "konto nie zostało jeszcze potwierdzone. Sprawdź skrzynkę e-mail."
	if strings.TrimSpace(w.Body.String()) == "" || !strings.Contains(w.Body.String(), expected) {
		t.Fatalf("expected original message %q, got %s", expected, w.Body.String())
	}
}
