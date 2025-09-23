package email

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
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
	mailer, err := NewSMTPMailer(cfg, "pl")
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
		mailer, err := NewSMTPMailer(cfg, "pl")
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

func TestSMTPMailerSendPasswordResetEmail(t *testing.T) {
	cfg := SMTPConfig{
		Host:      "smtp.example.com",
		Port:      587,
		FromEmail: "noreply@example.com",
		FromName:  "Kup Piksel",
	}
	mailer, err := NewSMTPMailer(cfg, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var capturedMsg []byte
	mailer.sendMail = func(ctx context.Context, cfg SMTPConfig, a smtp.Auth, from string, to []string, msg []byte) error {
		capturedMsg = append([]byte(nil), msg...)
		return nil
	}

	if err := mailer.SendPasswordResetEmail(context.Background(), "user@example.com", "https://example.com/reset?token=abc"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := strings.ToLower(string(capturedMsg))
	if !strings.Contains(payload, "subject: reset your password") {
		t.Fatalf("expected english subject in payload, got %s", string(capturedMsg))
	}
	if !strings.Contains(string(capturedMsg), "https://example.com/reset?token=abc") {
		t.Fatalf("expected reset link in payload")
	}
}

func TestSendMailWithContextImplicitTLS(t *testing.T) {
	cfg := SMTPConfig{
		Host:      "smtp.example.com",
		Port:      465,
		FromEmail: "noreply@example.com",
	}

	ctx := context.Background()

	called := false
	originalTLSDial := tlsDialWithDialer
	tlsDialWithDialer = func(dialer *net.Dialer, network, address string, config *tls.Config) (net.Conn, error) {
		called = true
		if config == nil {
			t.Fatalf("expected tls config")
		}
		if config.ServerName != cfg.Host {
			t.Fatalf("expected server name %q, got %q", cfg.Host, config.ServerName)
		}
		return nil, errors.New("test dial failure")
	}
	t.Cleanup(func() { tlsDialWithDialer = originalTLSDial })

	err := sendMailWithContext(ctx, cfg, nil, cfg.FromEmail, []string{"user@example.com"}, []byte("message"))
	if err == nil {
		t.Fatalf("expected error")
	}
	if !called {
		t.Fatalf("expected implicit TLS dialer to be invoked")
	}
}
