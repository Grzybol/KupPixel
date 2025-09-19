package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"

	"github.com/example/kup-piksel/internal/email"
	"github.com/example/kup-piksel/internal/storage/sqlite"
)

type fakeMailer struct {
	sent int
}

func (f *fakeMailer) SendVerificationEmail(ctx context.Context, recipient, verificationLink string) error {
	f.sent++
	return nil
}

var _ email.Mailer = (*fakeMailer)(nil)

func TestHandleRegister_DisableVerificationEmail(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

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
