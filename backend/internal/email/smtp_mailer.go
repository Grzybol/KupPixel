package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
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

// SMTPMailer delivers emails through a configured SMTP transport.
type SMTPMailer struct {
	config   SMTPConfig
	auth     smtp.Auth
	locale   localeContent
	sendMail func(ctx context.Context, cfg SMTPConfig, auth smtp.Auth, from string, to []string, msg []byte) error
}

// NewSMTPMailer constructs a Mailer using real SMTP transport.
func NewSMTPMailer(cfg SMTPConfig, language string) (*SMTPMailer, error) {
	cfg.Sanitize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	return &SMTPMailer{
		config: cfg,
		auth:   auth,
		locale: resolveLocale(language),
		sendMail: func(ctx context.Context, cfg SMTPConfig, auth smtp.Auth, from string, to []string, msg []byte) error {
			return sendMailWithContext(ctx, cfg, auth, from, to, msg)
		},
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

	log.Printf(
		"[smtp] preparing verification email via %s from=%s to=%s",
		m.config.Address(),
		from.String(),
		to.Address,
	)

	subject := m.locale.verificationSubject
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)
	body := fmt.Sprintf(m.locale.verificationBody, verificationLink)

	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from.String()))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to.String()))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", encodedSubject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	payload := msg.Bytes()
	log.Printf("[smtp] sending email payload size=%d bytes", len(payload))

	if err := m.sendMail(ctx, m.config, m.auth, m.config.FromEmail, []string{recipient}, payload); err != nil {
		return fmt.Errorf("send smtp email: %w", err)
	}
	log.Printf("[smtp] verification email sent successfully to %s", recipient)
	return nil
}

// SendPasswordResetEmail sends a password reset link to the user.
func (m *SMTPMailer) SendPasswordResetEmail(ctx context.Context, recipient, resetLink string) error {
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

	resetLink = strings.TrimSpace(resetLink)
	if resetLink == "" {
		return errors.New("reset link must not be empty")
	}

	from := mail.Address{Name: m.config.FromName, Address: m.config.FromEmail}
	to := mail.Address{Address: recipient}

	log.Printf(
		"[smtp] preparing password reset email via %s from=%s to=%s",
		m.config.Address(),
		from.String(),
		to.Address,
	)

	subject := m.locale.resetSubject
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)
	body := fmt.Sprintf(m.locale.resetBody, resetLink)

	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from.String()))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to.String()))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", encodedSubject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	payload := msg.Bytes()
	log.Printf("[smtp] sending email payload size=%d bytes", len(payload))

	if err := m.sendMail(ctx, m.config, m.auth, m.config.FromEmail, []string{recipient}, payload); err != nil {
		return fmt.Errorf("send smtp email: %w", err)
	}
	log.Printf("[smtp] password reset email sent successfully to %s", recipient)
	return nil
}

var (
	newNetDialer      = func() *net.Dialer { return &net.Dialer{} }
	tlsDialWithDialer = func(dialer *net.Dialer, network, address string, config *tls.Config) (net.Conn, error) {
		return tls.DialWithDialer(dialer, network, address, config)
	}
)

func isImplicitTLSPort(port int) bool {
	return port == 465
}

func sendMailWithContext(ctx context.Context, cfg SMTPConfig, auth smtp.Auth, from string, to []string, msg []byte) (err error) {
	dialer := newNetDialer()

	address := cfg.Address()
	var conn net.Conn
	if isImplicitTLSPort(cfg.Port) {
		log.Printf("[smtp] dialing %s using implicit TLS", address)
		tlsConfig := &tls.Config{ServerName: cfg.Host}
		conn, err = tlsDialWithDialer(dialer, "tcp", address, tlsConfig)
	} else {
		log.Printf("[smtp] dialing %s", address)
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}
	log.Printf("[smtp] connected to %s", address)

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}
	log.Printf("[smtp] created smtp client for host=%s", cfg.Host)

	ctxDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = client.Close()
		case <-ctxDone:
		}
	}()

	defer func() {
		close(ctxDone)
		if err != nil {
			_ = client.Close()
			return
		}
		if quitErr := client.Quit(); quitErr != nil && !errors.Is(quitErr, io.EOF) {
			err = quitErr
		}
	}()

	if isImplicitTLSPort(cfg.Port) {
		log.Printf("[smtp] implicit TLS already negotiated; skipping STARTTLS")
	} else if ok, _ := client.Extension("STARTTLS"); ok {
		log.Printf("[smtp] attempting STARTTLS for host=%s", cfg.Host)
		tlsConfig := &tls.Config{ServerName: cfg.Host}
		if err = client.StartTLS(tlsConfig); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return err
		}
		log.Printf("[smtp] STARTTLS negotiation succeeded")
	} else {
		log.Printf("[smtp] server does not advertise STARTTLS; continuing without TLS upgrade")
	}

	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			log.Printf("[smtp] authenticating as %s", cfg.Username)
			if err = client.Auth(auth); err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return ctxErr
				}
				return err
			}
			log.Printf("[smtp] authentication successful")
		} else {
			log.Printf("[smtp] server does not advertise AUTH; skipping authentication")
		}
	} else {
		log.Printf("[smtp] no smtp auth configured; proceeding without authentication")
	}

	log.Printf("[smtp] MAIL FROM %s", from)
	if err = client.Mail(from); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}

	for _, addr := range to {
		log.Printf("[smtp] RCPT TO %s", addr)
		if err = client.Rcpt(addr); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return err
		}
	}

	log.Printf("[smtp] entering DATA phase")
	wc, err := client.Data()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}

	if _, err = wc.Write(msg); err != nil {
		_ = wc.Close()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}

	if err = wc.Close(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}
	log.Printf("[smtp] DATA phase completed")

	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	log.Printf("[smtp] smtp transaction finished successfully")
	return nil
}

var _ Mailer = (*SMTPMailer)(nil)
