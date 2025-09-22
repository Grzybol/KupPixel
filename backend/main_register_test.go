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

	"github.com/example/kup-piksel/internal/email"
	"github.com/example/kup-piksel/internal/storage/sqlite"
)

type fakeMailer struct {
	sent          int
	lastRecipient string
	lastLink      string
	resetSent     int
	lastResetLink string
}

func (f *fakeMailer) SendVerificationEmail(ctx context.Context, recipient, verificationLink string) error {
	f.sent++
	f.lastRecipient = recipient
	f.lastLink = verificationLink
	return nil
}

func (f *fakeMailer) SendPasswordResetEmail(ctx context.Context, recipient, resetLink string) error {
	f.resetSent++
	f.lastRecipient = recipient
	f.lastResetLink = resetLink
	return nil
}

var _ email.Mailer = (*fakeMailer)(nil)

func TestHandleRegister_DisableVerificationEmail(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	store.SetSkipPixelSeed(true)

	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	mailer := &fakeMailer{}
	server := &Server{
		store:                    store,
		sessions:                 NewSessionManager(),
		mailer:                   mailer,
		verificationBaseURL:      "http://example.com",
		verificationTokenTTL:     time.Hour,
		disableVerificationEmail: true,
		pixelCostPoints:          10,
	}

	body := bytes.NewBufferString(`{"email":"user@example.com","password":"strong"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/register", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c := &gin.Context{Writer: w, Request: req}

	server.handleRegister(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}
	if mailer.sent != 0 {
		t.Fatalf("expected no verification emails to be sent, got %d", mailer.sent)
	}

	user, err := store.GetUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if !user.IsVerified {
		t.Fatalf("expected user to be verified")
	}
}

func TestHandleRegister_ExistingUnverifiedResendsEmail(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	store.SetSkipPixelSeed(true)

	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	existing, err := store.CreateUser(context.Background(), "user@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	mailer := &fakeMailer{}
	server := &Server{
		store:                store,
		sessions:             NewSessionManager(),
		mailer:               mailer,
		verificationBaseURL:  "http://example.com",
		verificationTokenTTL: time.Hour,
		pixelCostPoints:      10,
	}

	body := bytes.NewBufferString(`{"email":"user@example.com","password":"strong"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/register", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c := &gin.Context{Writer: w, Request: req}

	server.handleRegister(c)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", w.Code)
	}
	if mailer.sent != 1 {
		t.Fatalf("expected one verification email to be sent, got %d", mailer.sent)
	}
	if mailer.lastRecipient != "user@example.com" {
		t.Fatalf("expected recipient to be user@example.com, got %s", mailer.lastRecipient)
	}
	if strings.TrimSpace(mailer.lastLink) == "" {
		t.Fatalf("expected verification link to be set")
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
	if record.UserID != existing.ID {
		t.Fatalf("expected token to belong to user %d, got %d", existing.ID, record.UserID)
	}

	if !strings.Contains(w.Body.String(), "link aktywacyjny") {
		t.Fatalf("expected success message about activation link, got %s", w.Body.String())
	}
}

func TestHandleRegister_ExistingVerifiedConflict(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	store.SetSkipPixelSeed(true)

	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	user, err := store.CreateUser(context.Background(), "user@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := store.MarkUserVerified(context.Background(), user.ID); err != nil {
		t.Fatalf("mark user verified: %v", err)
	}

	mailer := &fakeMailer{}
	server := &Server{
		store:                store,
		sessions:             NewSessionManager(),
		mailer:               mailer,
		verificationBaseURL:  "http://example.com",
		verificationTokenTTL: time.Hour,
		pixelCostPoints:      10,
	}

	body := bytes.NewBufferString(`{"email":"user@example.com","password":"strong"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/register", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c := &gin.Context{Writer: w, Request: req}

	server.handleRegister(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", w.Code)
	}
	if mailer.sent != 0 {
		t.Fatalf("expected no verification email to be sent, got %d", mailer.sent)
	}
	if !strings.Contains(w.Body.String(), "user already exists") {
		t.Fatalf("expected conflict message, got %s", w.Body.String())
	}
}
