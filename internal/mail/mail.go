// Package mail sends outbound email behind a small interface.
package mail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"
)

// Mailer sends email messages.
type Mailer interface {
	Send(ctx context.Context, to, subject, plainBody string) error
	SendHTML(ctx context.Context, to, subject, plainBody, htmlBody string) error
	// SendBulk sends a message with extra headers (e.g. List-Unsubscribe),
	// for mail sent to many recipients rather than in direct response to a
	// user action.
	SendBulk(ctx context.Context, to, subject, plainBody, htmlBody string, headers map[string]string) error
}

// SMTPConfig configures an SMTPMailer.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// SMTPMailer sends mail via net/smtp with PLAIN auth.
type SMTPMailer struct {
	cfg SMTPConfig
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

func (m *SMTPMailer) Send(ctx context.Context, to, subject, plainBody string) error {
	return m.SendBulk(ctx, to, subject, plainBody, "", nil)
}

func (m *SMTPMailer) SendHTML(ctx context.Context, to, subject, plainBody, htmlBody string) error {
	return m.SendBulk(ctx, to, subject, plainBody, htmlBody, nil)
}

func (m *SMTPMailer) SendBulk(ctx context.Context, to, subject, plainBody, htmlBody string, headers map[string]string) error {
	addr := m.cfg.Host + ":" + m.cfg.Port
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}

	msg := buildMessage(m.cfg.From, to, subject, plainBody, htmlBody, headers)
	return sendMailWithTimeout(ctx, addr, auth, m.cfg.From, []string{to}, msg, 15*time.Second)
}

func buildMessage(from, to, subject, plainBody, htmlBody string, headers map[string]string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-Id: %s\r\n", newMessageID(from))
	for k, v := range headers {
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}

	if htmlBody != "" {
		boundary := "waldi-mail-boundary"
		fmt.Fprintf(&b, "MIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", boundary, plainBody)
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s\r\n", boundary, htmlBody)
		fmt.Fprintf(&b, "--%s--\r\n", boundary)
	} else {
		fmt.Fprintf(&b, "Content-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", plainBody)
	}

	return []byte(b.String())
}

func newMessageID(from string) string {
	domain := from
	if i := strings.LastIndex(from, "@"); i != -1 {
		domain = from[i+1:]
	}
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(buf), domain)
}

func sendMailWithTimeout(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		err error
	}
	done := make(chan result, 1)
	go func() {
		done <- result{err: smtp.SendMail(addr, auth, from, to, msg)}
	}()

	select {
	case res := <-done:
		if res.err != nil {
			return fmt.Errorf("sending mail: %w", res.err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("sending mail: %w", ctx.Err())
	}
}

// Configured reports whether outbound mail will be sent over SMTP.
func Configured(m Mailer) bool {
	_, ok := m.(NoopMailer)
	return m != nil && !ok
}

// NoopMailer logs the email instead of sending it.
type NoopMailer struct {
	Logger *slog.Logger
}

func (m NoopMailer) Send(_ context.Context, to, subject, plainBody string) error {
	return m.SendBulk(context.Background(), to, subject, plainBody, "", nil)
}

func (m NoopMailer) SendHTML(_ context.Context, to, subject, plainBody, htmlBody string) error {
	return m.SendBulk(context.Background(), to, subject, plainBody, htmlBody, nil)
}

func (m NoopMailer) SendBulk(_ context.Context, to, subject, plainBody, htmlBody string, headers map[string]string) error {
	logger := m.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("mail not sent (no SMTP configured)", "to", to, "subject", subject, "plain", plainBody, "html_len", len(htmlBody), "headers", headers)
	return nil
}
