package email

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
)

// SMTPConfig contains configuration required to send transactional emails via SMTP.
type SMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

// Sanitize trims whitespace from configuration fields and applies defaults.
func (c *SMTPConfig) Sanitize() {
	c.Host = strings.TrimSpace(c.Host)
	c.Username = strings.TrimSpace(c.Username)
	c.Password = strings.TrimSpace(c.Password)
	c.FromEmail = strings.TrimSpace(c.FromEmail)
	c.FromName = strings.TrimSpace(c.FromName)
	if c.FromName == "" {
		c.FromName = "Kup Piksel"
	}
}

// Validate checks that the configuration contains mandatory fields.
func (c *SMTPConfig) Validate() error {
	if c == nil {
		return errors.New("smtp config is nil")
	}
	if c.Host == "" {
		return errors.New("SMTP host is required")
	}
	if c.Port <= 0 {
		return errors.New("SMTP port must be a positive integer")
	}
	if c.FromEmail == "" {
		return errors.New("SMTP sender email is required")
	}
	if (c.Username == "") != (c.Password == "") {
		return errors.New("SMTP username and password must be provided together")
	}
	return nil
}

// Address returns the host:port pair for smtp.SendMail.
func (c SMTPConfig) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// LoadSMTPConfigFromEnv builds SMTP configuration using environment variables.
// If no SMTP-related variables are present it returns (nil, nil).
func LoadSMTPConfigFromEnv(getenv func(string) string) (*SMTPConfig, error) {
	trim := func(key string) string { return strings.TrimSpace(getenv(key)) }

	host := trim("SMTP_HOST")
	portValue := trim("SMTP_PORT")
	username := trim("SMTP_USERNAME")
	password := trim("SMTP_PASSWORD")
	fromEmail := trim("SMTP_SENDER_EMAIL")
	fromName := trim("SMTP_SENDER_NAME")

	if host == "" && portValue == "" && username == "" && password == "" && fromEmail == "" && fromName == "" {
		return nil, nil
	}
	if host == "" {
		return nil, errors.New("SMTP_HOST must be set when enabling SMTP mailer")
	}
	if portValue == "" {
		return nil, errors.New("SMTP_PORT must be set when enabling SMTP mailer")
	}

	port, err := strconv.Atoi(portValue)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT value %q: %w", portValue, err)
	}

	cfg := &SMTPConfig{
		Host:      host,
		Port:      port,
		Username:  username,
		Password:  password,
		FromEmail: fromEmail,
		FromName:  fromName,
	}
	cfg.Sanitize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

var smtpSendMailFunc = smtp.SendMail

// SMTPMailer delivers emails through a configured SMTP transport.
type SMTPMailer struct {
	config   SMTPConfig
	auth     smtp.Auth
	sendMail func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// NewSMTPMailer constructs a Mailer using real SMTP transport.
func NewSMTPMailer(cfg SMTPConfig) (*SMTPMailer, error) {
	cfg.Sanitize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	return &SMTPMailer{
		config:   cfg,
		auth:     auth,
		sendMail: smtpSendMailFunc,
	}, nil
}

// SendVerificationEmail sends an activation email with a verification link.
func (m *SMTPMailer) SendVerificationEmail(ctx context.Context, recipient, verificationLink string) error {
	if m == nil {
		return errors.New("smtp mailer is nil")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return errors.New("recipient must not be empty")
	}
	if _, err := mail.ParseAddress(recipient); err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}

	verificationLink = strings.TrimSpace(verificationLink)
	if verificationLink == "" {
		return errors.New("verification link must not be empty")
	}

	from := mail.Address{Name: m.config.FromName, Address: m.config.FromEmail}
	to := mail.Address{Address: recipient}

	subject := "Potwierdź swój adres e-mail"
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)
	body := fmt.Sprintf("Cześć!\n\nKliknij poniższy link, aby potwierdzić swoje konto w Kup Piksel:\n%s\n\nJeżeli to nie Ty zakładałeś konto, zignoruj tę wiadomość.\n", verificationLink)

	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from.String()))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to.String()))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", encodedSubject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	if err := m.sendMail(m.config.Address(), m.auth, m.config.FromEmail, []string{recipient}, msg.Bytes()); err != nil {
		return fmt.Errorf("send smtp email: %w", err)
	}
	return nil
}

var _ Mailer = (*SMTPMailer)(nil)
