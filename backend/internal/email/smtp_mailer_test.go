package email

import (
	"context"
	"errors"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestLoadSMTPConfigFromEnv(t *testing.T) {
	t.Run("no config", func(t *testing.T) {
		cfg, err := LoadSMTPConfigFromEnv(func(string) string { return "" })
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cfg != nil {
			t.Fatalf("expected nil config, got %#v", cfg)
		}
	})

	t.Run("missing host", func(t *testing.T) {
		_, err := LoadSMTPConfigFromEnv(func(key string) string {
			switch key {
			case "SMTP_PORT":
				return "25"
			case "SMTP_SENDER_EMAIL":
				return "noreply@example.com"
			default:
				return ""
			}
		})
		if err == nil {
			t.Fatalf("expected error when host missing")
		}
	})

	t.Run("complete", func(t *testing.T) {
		cfg, err := LoadSMTPConfigFromEnv(func(key string) string {
			switch key {
			case "SMTP_HOST":
				return "smtp.example.com"
			case "SMTP_PORT":
				return "587"
			case "SMTP_USERNAME":
				return "user"
			case "SMTP_PASSWORD":
				return "secret"
			case "SMTP_SENDER_EMAIL":
				return "noreply@example.com"
			default:
				return ""
			}
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cfg == nil {
			t.Fatalf("expected config, got nil")
		}
		if cfg.FromName != "Kup Piksel" {
			t.Fatalf("expected default from name, got %q", cfg.FromName)
		}
	})
}

func TestSMTPMailerSendVerificationEmail(t *testing.T) {
	cfg := SMTPConfig{
		Host:      "smtp.example.com",
		Port:      587,
		FromEmail: "noreply@example.com",
		FromName:  "Kup Piksel",
	}
	mailer, err := NewSMTPMailer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var capturedAddr string
	var capturedFrom string
	var capturedTo []string
	var capturedMsg []byte
	mailer.sendMail = func(ctx context.Context, cfg SMTPConfig, a smtp.Auth, from string, to []string, msg []byte) error {
		capturedAddr = cfg.Address()
		capturedFrom = from
		capturedTo = append([]string(nil), to...)
		capturedMsg = append([]byte(nil), msg...)
		return nil
	}

	ctx := context.Background()
	err = mailer.SendVerificationEmail(ctx, "user@example.com", "https://kup-piksel.test/verify?token=abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedAddr == "" {
		t.Fatalf("expected sendMail to be called")
	}
	if capturedFrom != cfg.FromEmail {
		t.Fatalf("unexpected from address: %q", capturedFrom)
	}
	if len(capturedTo) != 1 || capturedTo[0] != "user@example.com" {
		t.Fatalf("unexpected recipients: %#v", capturedTo)
	}
	if !strings.Contains(string(capturedMsg), "https://kup-piksel.test/verify?token=abc") {
		t.Fatalf("message should contain verification link")
	}
	lowerMsg := strings.ToLower(string(capturedMsg))
	if !strings.Contains(lowerMsg, "subject: =?utf-8?q?potwierd=c5=ba_sw=c3=b3j_adres_e-mail?=") {
		t.Fatalf("subject should be encoded, got %s", string(capturedMsg))
	}

	t.Run("context cancellation", func(t *testing.T) {
		cfg := SMTPConfig{
			Host:      "smtp.example.com",
			Port:      587,
			FromEmail: "noreply@example.com",
			FromName:  "Kup Piksel",
		}
		mailer, err := NewSMTPMailer(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		started := make(chan struct{})
		mailer.sendMail = func(ctx context.Context, cfg SMTPConfig, a smtp.Auth, from string, to []string, msg []byte) error {
			close(started)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				t.Fatalf("sendMail was not cancelled")
			}
			return nil
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- mailer.SendVerificationEmail(ctx, "user@example.com", "https://kup-piksel.test/verify?token=cancel")
		}()

		<-started
		cancel()

		err = <-errCh
		if err == nil {
			t.Fatalf("expected error when context cancelled")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation error, got %v", err)
		}
	})
}
