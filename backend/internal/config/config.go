package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/example/kup-piksel/internal/email"
)

// Config represents backend configuration options loaded from disk.
type Config struct {
	SMTP                     *email.SMTPConfig `json:"smtp"`
	DisableVerificationEmail bool              `json:"disableVerificationEmail"`
	PixelCostPoints          int               `json:"pixelCostPoints"`
	Database                 *DatabaseConfig   `json:"database"`
	Email                    EmailConfig       `json:"email"`
	PasswordReset            PasswordReset     `json:"passwordReset"`
	Verification             Verification      `json:"verification"`
	TurnstileSecretKey       string            `json:"turnstileSecretKey"`
}

// EmailConfig controls localisation of transactional emails sent by the backend.
type EmailConfig struct {
	Language string `json:"language"`
}

// PasswordReset holds configuration for password reset tokens and links.
type PasswordReset struct {
	TokenTTLHours int    `json:"tokenTtlHours"`
	BaseURL       string `json:"baseUrl"`
}

// Verification holds configuration for verification tokens and links.
type Verification struct {
	TokenTTLHours int    `json:"tokenTtlHours"`
	BaseURL       string `json:"baseUrl"`
}

// DatabaseConfig encapsulates storage backend configuration.
type DatabaseConfig struct {
	Driver     string       `json:"driver"`
	SQLitePath string       `json:"sqlitePath"`
	MySQL      *MySQLConfig `json:"mysql"`
}

// MySQLConfig describes connection settings for MariaDB/MySQL engines.
type MySQLConfig struct {
	DSN         string `json:"dsn"`
	ExternalDSN string `json:"externalDsn"`
	UseExternal bool   `json:"useExternal"`
}

func defaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{Driver: "sqlite"}
}

func (c *DatabaseConfig) normalize() {
	if c == nil {
		return
	}
	c.Driver = strings.ToLower(strings.TrimSpace(c.Driver))
	if c.Driver == "" {
		c.Driver = "sqlite"
	}
	c.SQLitePath = strings.TrimSpace(c.SQLitePath)
	if c.MySQL != nil {
		c.MySQL.sanitize()
	}
}

func (c *MySQLConfig) sanitize() {
	if c == nil {
		return
	}
	c.DSN = strings.TrimSpace(c.DSN)
	c.ExternalDSN = strings.TrimSpace(c.ExternalDSN)
}

// Default returns the configuration used when no config file exists on disk yet.
func Default() *Config {
	return &Config{
		DisableVerificationEmail: false,
		PixelCostPoints:          10,
		Database:                 defaultDatabaseConfig(),
		Email:                    EmailConfig{Language: "pl"},
		PasswordReset:            PasswordReset{TokenTTLHours: 24},
		Verification:             Verification{TokenTTLHours: 24},
	}
}

// WriteFile writes the given configuration as prettified JSON to the provided path.
func WriteFile(path string, cfg *Config) error {
	if cfg == nil {
		return errors.New("config must not be nil")
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// Load reads the configuration from a JSON or JSON5 file located at the given path.
func Load(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("config path must not be empty")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config file: %w", err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	normalized := normalizeJSON5(data)

	var cfg Config
	if err := json.Unmarshal(normalized, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.SMTP != nil {
		cfg.SMTP.Sanitize()
		if err := cfg.SMTP.Validate(); err != nil {
			return nil, fmt.Errorf("smtp: %w", err)
		}
	}

	if cfg.PixelCostPoints <= 0 {
		cfg.PixelCostPoints = Default().PixelCostPoints
	}

	cfg.Email.Language = strings.ToLower(strings.TrimSpace(cfg.Email.Language))
	if cfg.Email.Language == "" {
		cfg.Email.Language = Default().Email.Language
	}

	cfg.TurnstileSecretKey = strings.TrimSpace(cfg.TurnstileSecretKey)

	if cfg.PasswordReset.TokenTTLHours <= 0 {
		cfg.PasswordReset.TokenTTLHours = Default().PasswordReset.TokenTTLHours
	}
	cfg.PasswordReset.BaseURL = strings.TrimSpace(cfg.PasswordReset.BaseURL)

	if cfg.Verification.TokenTTLHours <= 0 {
		cfg.Verification.TokenTTLHours = Default().Verification.TokenTTLHours
	}
	cfg.Verification.BaseURL = strings.TrimSpace(cfg.Verification.BaseURL)

	if cfg.Database == nil {
		cfg.Database = defaultDatabaseConfig()
	} else {
		cfg.Database.normalize()
	}

	switch cfg.Database.Driver {
	case "sqlite":
	case "mysql":
		if cfg.Database.MySQL == nil {
			return nil, errors.New("database: mysql configuration required")
		}
		if cfg.Database.MySQL.DSN == "" && cfg.Database.MySQL.ExternalDSN == "" {
			return nil, errors.New("database: mysql dsn must be provided")
		}
	default:
		return nil, fmt.Errorf("database: unsupported driver %q", cfg.Database.Driver)
	}

	return &cfg, nil
}

func normalizeJSON5(input []byte) []byte {
	withoutComments := stripJSONComments(input)
	return removeTrailingCommas(withoutComments)
}

func stripJSONComments(input []byte) []byte {
	var buf bytes.Buffer
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if inString {
			buf.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			buf.WriteByte(ch)
			continue
		}

		if ch == '/' && i+1 < len(input) {
			next := input[i+1]
			if next == '/' {
				i += 2
				for i < len(input) && input[i] != '\n' && input[i] != '\r' {
					i++
				}
				buf.WriteByte('\n')
				continue
			}
			if next == '*' {
				i += 2
				for i+1 < len(input) && !(input[i] == '*' && input[i+1] == '/') {
					i++
				}
				i++
				continue
			}
		}

		buf.WriteByte(ch)
	}
	return buf.Bytes()
}

func removeTrailingCommas(input []byte) []byte {
	var buf bytes.Buffer
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if inString {
			buf.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			buf.WriteByte(ch)
			continue
		}

		if ch == ',' {
			j := i + 1
			for j < len(input) {
				if input[j] == ' ' || input[j] == '\n' || input[j] == '\r' || input[j] == '\t' {
					j++
					continue
				}
				break
			}
			if j < len(input) && (input[j] == '}' || input[j] == ']') {
				continue
			}
		}

		buf.WriteByte(ch)
	}
	return buf.Bytes()
}
