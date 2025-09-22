package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/example/kup-piksel/internal/email"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoad_DisableVerificationWithoutSMTP(t *testing.T) {
	path := writeTempConfig(t, `{
                // JSON5 style comment should be accepted
                "disableVerificationEmail": true,
        }`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config instance, got nil")
	}
	if !cfg.DisableVerificationEmail {
		t.Fatalf("expected DisableVerificationEmail to be true, got %v", cfg.DisableVerificationEmail)
	}
	if cfg.SMTP != nil {
		t.Fatalf("expected SMTP to be nil, got %#v", cfg.SMTP)
	}
	if cfg.Database == nil {
		t.Fatal("expected database configuration")
	}
	if cfg.Database.Driver != "sqlite" {
		t.Fatalf("expected sqlite driver, got %q", cfg.Database.Driver)
	}
	if cfg.Email.Language != "pl" {
		t.Fatalf("expected default email language pl, got %q", cfg.Email.Language)
	}
	if cfg.PasswordReset.TokenTTLHours != Default().PasswordReset.TokenTTLHours {
		t.Fatalf("expected default reset ttl, got %d", cfg.PasswordReset.TokenTTLHours)
	}
}

func TestLoad_WithValidSMTP(t *testing.T) {
	path := writeTempConfig(t, `{
                "smtp": {
                        "host": "smtp.example.com",
                        "port": 587,
                        "username": " user ",
                        "password": " secret ",
                        "fromEmail": "noreply@example.com",
                        "fromName": "  Team  "
                }
        }`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.SMTP == nil {
		t.Fatal("expected SMTP configuration")
	}
	expected := &email.SMTPConfig{
		Host:      "smtp.example.com",
		Port:      587,
		Username:  "user",
		Password:  "secret",
		FromEmail: "noreply@example.com",
		FromName:  "Team",
	}
	if *cfg.SMTP != *expected {
		t.Fatalf("unexpected SMTP configuration: %#v", cfg.SMTP)
	}
	if cfg.Database == nil {
		t.Fatal("expected database configuration")
	}
	if cfg.Database.Driver != "sqlite" {
		t.Fatalf("expected sqlite driver by default, got %q", cfg.Database.Driver)
	}
	if cfg.Email.Language != "pl" {
		t.Fatalf("expected default email language pl, got %q", cfg.Email.Language)
	}
	if cfg.PasswordReset.TokenTTLHours != Default().PasswordReset.TokenTTLHours {
		t.Fatalf("expected default reset ttl, got %d", cfg.PasswordReset.TokenTTLHours)
	}
}

func TestLoad_InvalidSMTP(t *testing.T) {
	path := writeTempConfig(t, `{
                "smtp": {
                        "host": "",
                        "port": 587,
                        "fromEmail": "noreply@example.com"
                }
        }`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error when SMTP is invalid")
	}
}

func TestLoad_MissingPath(t *testing.T) {
	if _, err := Load(" "); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestLoad_InvalidDatabaseDriver(t *testing.T) {
	path := writeTempConfig(t, `{ "database": { "driver": "oracle" } }`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for unsupported driver")
	}
}

func TestLoad_MySQLRequiresDSN(t *testing.T) {
	path := writeTempConfig(t, `{ "database": { "driver": "mysql" } }`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error when mysql dsn missing")
	}
}

func TestWriteFile_DefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file to not exist, got: %v", err)
	}

	if err := WriteFile(path, Default()); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist, got: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.DisableVerificationEmail {
		t.Fatalf("expected DisableVerificationEmail to be false, got %v", cfg.DisableVerificationEmail)
	}

	if cfg.SMTP != nil {
		t.Fatalf("expected SMTP to be nil, got %#v", cfg.SMTP)
	}
	if cfg.Database == nil {
		t.Fatal("expected database configuration")
	}
	if cfg.Database.Driver != "sqlite" {
		t.Fatalf("expected sqlite driver, got %q", cfg.Database.Driver)
	}
	if cfg.Email.Language != "pl" {
		t.Fatalf("expected default email language pl, got %q", cfg.Email.Language)
	}
	if cfg.PasswordReset.TokenTTLHours != Default().PasswordReset.TokenTTLHours {
		t.Fatalf("expected default reset ttl, got %d", cfg.PasswordReset.TokenTTLHours)
	}
}
