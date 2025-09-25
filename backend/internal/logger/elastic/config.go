package elastic

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config describes connection and buffering settings for the Elastic logger.
type Config struct {
	URL           string
	Index         string
	APIKey        string
	VerifyCert    bool
	CAPath        string
	FlushInterval time.Duration
	MaxBuffer     int
	MaxBytes      int
	MaxRetries    int
}

// FromEnv builds a configuration from environment variables. It returns the
// parsed configuration, a boolean indicating whether logging should be enabled
// and an error if any value is malformed.
func FromEnv() (Config, bool, error) {
	rawURL := strings.TrimSpace(os.Getenv("ELASTIC_URL"))
	if rawURL == "" {
		rawURL = "https://127.0.0.1:9200"
	}

	index := strings.TrimSpace(os.Getenv("ELASTIC_INDEX"))
	if index == "" {
		index = "website-backend"
	}

	apiKey := strings.TrimSpace(os.Getenv("ELASTIC_API_KEY"))
	if apiKey == "" {
		return Config{}, false, nil
	}

	verifyCert, err := parseBoolEnv("ELASTIC_VERIFY_CERT", false)
	if err != nil {
		return Config{}, false, err
	}

	caPath := strings.TrimSpace(os.Getenv("ELASTIC_CA_PATH"))

	flushMS, err := parseIntEnv("ELASTIC_FLUSH_MS", 60000)
	if err != nil {
		return Config{}, false, err
	}
	if flushMS <= 0 {
		flushMS = 60000
	}

	maxBuffer, err := parseIntEnv("ELASTIC_MAX_BUFFER", 2000)
	if err != nil {
		return Config{}, false, err
	}
	if maxBuffer <= 0 {
		maxBuffer = 2000
	}

	maxBytes, err := parseIntEnv("ELASTIC_MAX_BYTES", 5*1024*1024)
	if err != nil {
		return Config{}, false, err
	}
	if maxBytes <= 0 {
		maxBytes = 5 * 1024 * 1024
	}

	maxRetries, err := parseIntEnv("ELASTIC_MAX_RETRIES", 3)
	if err != nil {
		return Config{}, false, err
	}
	if maxRetries < 0 {
		maxRetries = 3
	}

	cfg := Config{
		URL:           rawURL,
		Index:         index,
		APIKey:        apiKey,
		VerifyCert:    verifyCert,
		CAPath:        caPath,
		FlushInterval: time.Duration(flushMS) * time.Millisecond,
		MaxBuffer:     maxBuffer,
		MaxBytes:      maxBytes,
		MaxRetries:    maxRetries,
	}

	return cfg, true, nil
}

func parseBoolEnv(name string, def bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def, nil
	}
	lower := strings.ToLower(raw)
	switch lower {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return def, fmt.Errorf("%s: invalid boolean value %q", name, raw)
	}
}

func parseIntEnv(name string, def int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return def, fmt.Errorf("%s: invalid integer value %q", name, raw)
	}
	return value, nil
}

// Validate ensures the configuration is internally consistent.
func (c Config) Validate() error {
	if strings.TrimSpace(c.URL) == "" {
		return fmt.Errorf("elastic url must not be empty")
	}
	if strings.TrimSpace(c.Index) == "" {
		return fmt.Errorf("elastic index must not be empty")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("elastic api key must not be empty")
	}
	if c.FlushInterval <= 0 {
		return fmt.Errorf("flush interval must be positive")
	}
	if c.MaxBuffer <= 0 {
		return fmt.Errorf("max buffer must be positive")
	}
	if c.MaxBytes <= 0 {
		return fmt.Errorf("max bytes must be positive")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries must be zero or greater")
	}
	return nil
}
