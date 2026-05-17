package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

// EmailConfig はSMTP接続に必要な設定を保持します
type EmailConfig struct {
	Host     string
	Port     string
	From     string
	User     string
	Password string
	Timeout  time.Duration
	UseTLS   bool
}

// Mailer はメール送信のインターフェースです
type Mailer interface {
	Send(to string, subject string, text string, html string) error
}

type SMTPMailer struct {
	cfg *EmailConfig
}

func NewSMTPMailer(cfg *EmailConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

// Send は実際にSMTPサーバーを叩いてメールを送信します
func (m *SMTPMailer) Send(to, subject, text, html string) error {
	addr := fmt.Sprintf("%s:%s", m.cfg.Host, m.cfg.Port)
	conn, err := net.DialTimeout("tcp", addr, m.cfg.Timeout)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client creation failed: %w", err)
	}
	defer c.Quit()

	if m.cfg.UseTLS {
		tlsCfg := &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}
		if err = c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("starttls failed: %w", err)
		}
	}

	if m.cfg.Password != "" {
		auth := smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)
		if err = c.Auth(auth); err != nil {
			return fmt.Errorf("auth failed: %w", err)
		}
	}

	if err = c.Mail(m.cfg.From); err != nil {
		return err
	}
	if err = c.Rcpt(to); err != nil {
		return err
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	data := buildMessageData(EmailMessage{
		From:    m.cfg.From,
		To:      to,
		Subject: subject,
		Text:    text,
		HTML:    html,
	})

	if _, err = w.Write(data); err != nil {
		return err
	}
	return w.Close()
}

func buildMessageData(m EmailMessage) []byte {
	boundary := m.Boundary
	if boundary == "" {
		boundary = "taskly-boundary-42"
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", m.From)
	fmt.Fprintf(&buf, "To: %s\r\n", m.To)
	fmt.Fprintf(&buf, "Subject: %s\r\n", m.Subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary)
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=utf-8\r\n\r\n")
	fmt.Fprintf(&buf, "%s\r\n", m.Text)
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/html; charset=utf-8\r\n\r\n")
	fmt.Fprintf(&buf, "%s\r\n", m.HTML)
	fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	return buf.Bytes()
}
