package elastic

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// Fields represent structured metadata attached to a log entry.
type Fields map[string]any

// Logger buffers log events and ships them to Elasticsearch using the bulk API.
type Logger struct {
	cfg     Config
	baseURL *url.URL
	client  *http.Client

	mu         sync.Mutex
	buffer     bytes.Buffer
	entries    int
	bufferSize int

	flushMu sync.Mutex

	ticker *time.Ticker
	stopCh chan struct{}
	doneCh chan struct{}
}

// NewLogger validates the configuration, creates an HTTP client and starts the
// background flusher.
func NewLogger(cfg Config) (*Logger, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	parsed, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse elastic url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("elastic url must use http or https scheme")
	}

	transport := &http.Transport{}
	if parsed.Scheme == "https" {
		tlsConfig := &tls.Config{}
		if !cfg.VerifyCert {
			tlsConfig.InsecureSkipVerify = true //nolint:gosec // required for self-signed clusters
		}
		if cfg.CAPath != "" {
			pemBytes, err := os.ReadFile(cfg.CAPath)
			if err != nil {
				return nil, fmt.Errorf("read elastic ca: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pemBytes) {
				return nil, errors.New("elastic ca: failed to append certificates")
			}
			tlsConfig.RootCAs = pool
		}
		transport.TLSClientConfig = tlsConfig
	}

	client := &http.Client{Timeout: 30 * time.Second, Transport: transport}

	logger := &Logger{
		cfg:     cfg,
		baseURL: parsed,
		client:  client,
		ticker:  time.NewTicker(cfg.FlushInterval),
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	go logger.run()

	return logger, nil
}

func (l *Logger) run() {
	defer close(l.doneCh)
	for {
		select {
		case <-l.ticker.C:
			_ = l.Flush(context.Background())
		case <-l.stopCh:
			return
		}
	}
}

// Write implements io.Writer so the logger can be used as log output sink.
func (l *Logger) Write(p []byte) (int, error) {
	message := strings.TrimSpace(string(p))
	if message == "" {
		return len(p), nil
	}
	l.Log("info", message, nil)
	return len(p), nil
}

// Log enqueues a structured log entry for asynchronous delivery.
func (l *Logger) Log(level string, message string, fields Fields) {
	entry := make(map[string]any, len(fields)+4)
	entry["@timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	entry["level"] = strings.ToUpper(strings.TrimSpace(level))
	entry["message"] = message
	entry["service"] = "KupPixelBackend"
	for k, v := range fields {
		if k == "" || v == nil {
			continue
		}
		entry[k] = v
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ElasticLogger] marshal entry failed: %v\n", err)
		return
	}

	meta := []byte("{\"index\":{}}\n")
	ndjson := append(meta, payload...)
	ndjson = append(ndjson, '\n')

	l.enqueue(ndjson)
}

func (l *Logger) enqueue(item []byte) {
	l.mu.Lock()
	l.buffer.Write(item)
	l.entries++
	l.bufferSize += len(item)
	shouldFlush := l.entries >= l.cfg.MaxBuffer || l.bufferSize >= l.cfg.MaxBytes
	l.mu.Unlock()

	if shouldFlush {
		go func() {
			_ = l.Flush(context.Background())
		}()
	}
}

// Flush immediately sends the buffered entries to Elasticsearch.
func (l *Logger) Flush(ctx context.Context) error {
	l.flushMu.Lock()
	defer l.flushMu.Unlock()

	l.mu.Lock()
	if l.entries == 0 {
		l.mu.Unlock()
		return nil
	}

	batch := make([]byte, l.buffer.Len())
	copy(batch, l.buffer.Bytes())
	l.buffer.Reset()
	l.entries = 0
	l.bufferSize = 0
	l.mu.Unlock()

	if err := l.sendWithRetry(ctx, batch); err != nil {
		l.requeue(batch)
		return err
	}
	return nil
}

func (l *Logger) requeue(batch []byte) {
	lines := bytes.Split(batch, []byte{'\n'})
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := 0; i+1 < len(lines); i += 2 {
		meta := lines[i]
		if len(bytes.TrimSpace(meta)) == 0 {
			continue
		}
		doc := lines[i+1]
		nd := append(append(append([]byte{}, meta...), '\n'), append(doc, '\n')...)
		l.buffer.Write(nd)
		l.entries++
		l.bufferSize += len(nd)
		if l.entries >= l.cfg.MaxBuffer*2 || l.bufferSize >= l.cfg.MaxBytes*2 {
			break
		}
	}
}

func (l *Logger) sendWithRetry(ctx context.Context, payload []byte) error {
	var lastErr error
	for attempt := 0; attempt <= l.cfg.MaxRetries; attempt++ {
		status, body, err := l.post(ctx, payload)
		if err != nil {
			lastErr = err
		} else if status >= 200 && status < 300 {
			l.reportPartialErrors(body)
			return nil
		} else {
			truncated := truncateBody(body)
			lastErr = fmt.Errorf("bulk request failed: status=%d body=%s", status, truncated)
			if status != http.StatusTooManyRequests && (status < 500 || status >= 600) {
				return lastErr
			}
		}

		if attempt < l.cfg.MaxRetries {
			backoff := time.Duration(min(15000, 500*(1<<attempt))) * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return lastErr
}

func (l *Logger) post(ctx context.Context, payload []byte) (int, []byte, error) {
	endpoint := *l.baseURL
	cleanPath := strings.TrimRight(endpoint.Path, "/")
	escapedIndex := url.PathEscape(l.cfg.Index)
	endpoint.Path = path.Join(cleanPath, escapedIndex, "_bulk")
	if !strings.HasPrefix(endpoint.Path, "/") {
		endpoint.Path = "/" + endpoint.Path
	}
	endpoint.RawQuery = ""
	endpoint.Fragment = ""

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("Authorization", "ApiKey "+l.cfg.APIKey)

	resp, err := l.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func (l *Logger) reportPartialErrors(body []byte) {
	if len(body) == 0 {
		return
	}
	var parsed bulkResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return
	}
	if !parsed.Errors {
		return
	}
	first := parsed.FirstError()
	if first == nil {
		return
	}
	data, err := json.Marshal(first)
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "[ElasticLogger] partial error: %s\n", string(data))
}

// Close stops the background flusher and drains the buffer.
func (l *Logger) Close(ctx context.Context) error {
	l.ticker.Stop()
	close(l.stopCh)
	<-l.doneCh
	return l.Flush(ctx)
}

func truncateBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	const limit = 512
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit]) + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type bulkResponse struct {
	Errors bool                          `json:"errors"`
	Items  []map[string]bulkResponseItem `json:"items"`
}

type bulkResponseItem struct {
	Status int            `json:"status"`
	Error  map[string]any `json:"error"`
}

func (r *bulkResponse) FirstError() map[string]any {
	if r == nil {
		return nil
	}
	for _, item := range r.Items {
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			entry := item[key]
			if entry.Error != nil {
				entryMap := map[string]any{
					"operation": key,
					"status":    entry.Status,
					"error":     entry.Error,
				}
				return entryMap
			}
		}
	}
	return nil
}
