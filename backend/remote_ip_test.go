package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractRemoteIPDirectConnection(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	r.RemoteAddr = "93.184.216.34:54321"

	if got := extractRemoteIP(r); got != "93.184.216.34" {
		t.Fatalf("expected remote IP 93.184.216.34, got %q", got)
	}
}

func TestExtractRemoteIPCloudflareHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	r.RemoteAddr = "10.0.0.5:12345"
	r.Header.Set("CF-Connecting-IP", "198.51.100.10")

	if got := extractRemoteIP(r); got != "198.51.100.10" {
		t.Fatalf("expected CF-Connecting-IP to be used, got %q", got)
	}
}

func TestExtractRemoteIPForwardedForChain(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	r.RemoteAddr = "10.0.0.5:12345"
	r.Header.Set("X-Forwarded-For", " 10.0.0.1, 198.51.100.25 , 203.0.113.9")

	if got := extractRemoteIP(r); got != "198.51.100.25" {
		t.Fatalf("expected first public IP from X-Forwarded-For, got %q", got)
	}
}

func TestExtractRemoteIPIgnoresPrivateHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	r.RemoteAddr = "203.0.113.30:4242"
	r.Header.Set("X-Forwarded-For", " 10.0.0.1, 192.168.1.5")

	if got := extractRemoteIP(r); got != "203.0.113.30" {
		t.Fatalf("expected fallback to remote address, got %q", got)
	}
}

func TestExtractRemoteIPReturnsEmptyWhenNoPublicIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", " 10.0.0.1")

	if got := extractRemoteIP(r); got != "" {
		t.Fatalf("expected empty result when no public IP is available, got %q", got)
	}
}
