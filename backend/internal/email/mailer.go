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
	SendPasswordResetEmail(ctx context.Context, recipient, resetLink string) error
}

// ConsoleMailer logs outgoing emails instead of delivering them.
// It is useful for development environments where SMTP access is not available.
type ConsoleMailer struct {
	FromName string
	locale   localeContent
}

// NewConsoleMailer creates a ConsoleMailer with the provided sender label.
func NewConsoleMailer(fromName, language string) *ConsoleMailer {
	name := strings.TrimSpace(fromName)
	if name == "" {
		name = "Kup Piksel"
	}
	return &ConsoleMailer{FromName: name, locale: resolveLocale(language)}
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

	log.Printf("[email] To: %s | Subject: %s | Link: %s", strings.TrimSpace(recipient), m.locale.verificationSubject, link)
	return nil
}

var _ Mailer = (*ConsoleMailer)(nil)

// SendPasswordResetEmail logs the reset link for developers.
func (m *ConsoleMailer) SendPasswordResetEmail(ctx context.Context, recipient, resetLink string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	link := strings.TrimSpace(resetLink)
	if link == "" {
		return fmt.Errorf("reset link must not be empty")
	}
	if _, err := url.Parse(link); err != nil {
		return fmt.Errorf("invalid reset link: %w", err)
	}

	log.Printf("[email] To: %s | Subject: %s | Link: %s", strings.TrimSpace(recipient), m.locale.resetSubject, link)
	return nil
}

type localeContent struct {
	verificationSubject string
	verificationBody    string
	resetSubject        string
	resetBody           string
}

var locales = map[string]localeContent{
	"pl": {
		verificationSubject: "Potwierdź swój adres e-mail",
		verificationBody:    "Cześć!\n\nKliknij poniższy link, aby potwierdzić swoje konto w Kup Piksel:\n%s\n\nJeżeli to nie Ty zakładałeś konto, zignoruj tę wiadomość.\n",
		resetSubject:        "Zresetuj swoje hasło",
		resetBody:           "Cześć!\n\nKliknij poniższy link, aby ustawić nowe hasło do konta w Kup Piksel:\n%s\n\nJeżeli to nie Ty prosiłeś o reset hasła, zignoruj tę wiadomość.\n",
	},
	"en": {
		verificationSubject: "Confirm your email address",
		verificationBody:    "Hello!\n\nClick the link below to confirm your Kup Piksel account:\n%s\n\nIf you didn't create an account, please ignore this message.\n",
		resetSubject:        "Reset your password",
		resetBody:           "Hello!\n\nClick the link below to set a new password for your Kup Piksel account:\n%s\n\nIf you didn't request a password reset, please ignore this message.\n",
	},
}

func resolveLocale(language string) localeContent {
	lang := strings.ToLower(strings.TrimSpace(language))
	if lang == "" {
		lang = "pl"
	}
	if content, ok := locales[lang]; ok {
		return content
	}
	return locales["pl"]
}
