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
}

// Default returns the configuration used when no config file exists on disk yet.
func Default() *Config {
	return &Config{DisableVerificationEmail: true}
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
