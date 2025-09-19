package email

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
)

// Mailer is responsible for delivering transactional emails to users.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, recipient, verificationLink string) error
}

// ConsoleMailer logs outgoing emails instead of delivering them.
// It is useful for development environments where SMTP access is not available.
type ConsoleMailer struct {
	FromName string
}

// NewConsoleMailer creates a ConsoleMailer with the provided sender label.
func NewConsoleMailer(fromName string) *ConsoleMailer {
	name := strings.TrimSpace(fromName)
	if name == "" {
		name = "Kup Piksel"
	}
	return &ConsoleMailer{FromName: name}
}

// SendVerificationEmail logs the verification link so developers can copy it.
func (m *ConsoleMailer) SendVerificationEmail(ctx context.Context, recipient, verificationLink string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	link := strings.TrimSpace(verificationLink)
	if link == "" {
		return fmt.Errorf("verification link must not be empty")
	}
	if _, err := url.Parse(link); err != nil {
		return fmt.Errorf("invalid verification link: %w", err)
	}

	log.Printf("[email] To: %s | Subject: Potwierdź swój adres e-mail | Link: %s", strings.TrimSpace(recipient), link)
	return nil
}

var _ Mailer = (*ConsoleMailer)(nil)
