package mailer

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
)

type Mailer interface {
	SendVerificationEmail(ctx context.Context, recipientEmail, token string) error
}

type LoggerMailer struct {
	BaseURL string
}

func NewLoggerMailer(baseURL string) *LoggerMailer {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		trimmed = "http://localhost:5173"
	}
	return &LoggerMailer{BaseURL: trimmed}
}

func (m *LoggerMailer) SendVerificationEmail(ctx context.Context, recipientEmail, token string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	verificationURL := m.buildVerificationURL(token)
	log.Printf("[mailer] send verification email to %s: %s", recipientEmail, verificationURL)
	return nil
}

func (m *LoggerMailer) buildVerificationURL(token string) string {
	base := strings.TrimRight(m.BaseURL, "/")
	if base == "" {
		base = "http://localhost:5173"
	}
	u, err := url.Parse(base)
	if err != nil {
		return fmt.Sprintf("%s/verify-email?token=%s", base, token)
	}
	q := u.Query()
	if !strings.HasSuffix(u.Path, "/verify-email") {
		u.Path = strings.TrimRight(u.Path, "/") + "/verify-email"
	}
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String()
}
