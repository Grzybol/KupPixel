package main

import (
	"context"
	"strings"
)

const testTurnstileToken = "test-turnstile-token"

func enableTurnstileForTest(server *Server) {
	server.turnstileSecret = "test-secret"
	server.turnstileVerify = func(ctx context.Context, secret, token, remoteIP string) (turnstileResponse, error) {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			return turnstileResponse{Success: false}, nil
		}
		return turnstileResponse{Success: true}, nil
	}
}
