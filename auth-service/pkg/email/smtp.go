package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Config holds SMTP connection settings.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	// UseTLS: port 465 implicit TLS. Port 587 uses STARTTLS automatically.
	UseTLS bool
}

// Client sends transactional emails over SMTP.
type Client struct {
	cfg Config
}

// NewClient creates an SMTP email client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("smtp host is required")
	}
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if cfg.From == "" {
		return nil, fmt.Errorf("smtp from address is required")
	}
	// Gmail app passwords are sometimes copied with spaces.
	cfg.Password = strings.ReplaceAll(cfg.Password, " ", "")
	return &Client{cfg: cfg}, nil
}

// SendVerificationEmail sends an email verification message with an HTML body.
func (c *Client) SendVerificationEmail(ctx context.Context, to, verifyURL string) error {
	data := verificationTemplateData{
		VerifyURL: verifyURL,
		Year:      time.Now().Year(),
	}
	body, err := renderTemplate(verificationHTMLTemplate, data)
	if err != nil {
		return fmt.Errorf("render verification template: %w", err)
	}
	return c.send(ctx, to, "Verify your email address", body)
}

// SendPasswordResetEmail sends a password reset message with an HTML body.
func (c *Client) SendPasswordResetEmail(ctx context.Context, to, resetURL string) error {
	data := resetTemplateData{
		ResetURL: resetURL,
		Year:     time.Now().Year(),
	}
	body, err := renderTemplate(resetHTMLTemplate, data)
	if err != nil {
		return fmt.Errorf("render reset template: %w", err)
	}
	return c.send(ctx, to, "Reset your password", body)
}

func (c *Client) send(ctx context.Context, to, subject, htmlBody string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("recipient email is required")
	}

	from := c.cfg.From
	fromHeader := from
	if c.cfg.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", c.cfg.FromName, from)
	}

	var msg bytes.Buffer
	msg.WriteString("From: " + fromHeader + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	addr := net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port))
	auth := smtpAuth(c.cfg)

	// Port 465: implicit TLS (SMTPS). Port 587/25: STARTTLS (Gmail, Mailhog, etc.).
	if c.cfg.Port == 465 || (c.cfg.UseTLS && c.cfg.Port != 587) {
		return c.sendImplicitTLS(addr, auth, from, []string{to}, msg.Bytes())
	}
	return c.sendSTARTTLS(addr, auth, from, []string{to}, msg.Bytes())
}

func (c *Client) sendImplicitTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsCfg := &tls.Config{ServerName: c.cfg.Host, MinVersion: tls.VersionTLS12}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	return c.deliver(client, auth, from, to, msg)
}

func (c *Client) sendSTARTTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: c.cfg.Host, MinVersion: tls.VersionTLS12}
		if err := client.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	return c.deliver(client, auth, from, to, msg)
}

func (c *Client) deliver(client *smtp.Client, auth smtp.Auth, from string, to []string, msg []byte) error {
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp rcpt %s: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return client.Quit()
}

func smtpAuth(cfg Config) smtp.Auth {
	if cfg.Username == "" && cfg.Password == "" {
		return nil
	}
	return smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
}

type verificationTemplateData struct {
	VerifyURL string
	Year      int
}

type resetTemplateData struct {
	ResetURL string
	Year     int
}

const verificationHTMLTemplate = `<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>Verify your email</title></head>
<body style="font-family:Arial,sans-serif;line-height:1.6;color:#222;">
  <h2>Verify your email</h2>
  <p>Thanks for signing up. Please confirm your email address by clicking the button below:</p>
  <p><a href="{{.VerifyURL}}" style="display:inline-block;padding:12px 20px;background:#2563eb;color:#fff;text-decoration:none;border-radius:6px;">Verify email</a></p>
  <p>Or copy this link into your browser:<br><a href="{{.VerifyURL}}">{{.VerifyURL}}</a></p>
  <p style="color:#666;font-size:12px;">If you did not create an account, you can ignore this email.</p>
  <p style="color:#666;font-size:12px;">&copy; {{.Year}} Auth Service</p>
</body>
</html>`

const resetHTMLTemplate = `<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>Reset your password</title></head>
<body style="font-family:Arial,sans-serif;line-height:1.6;color:#222;">
  <h2>Reset your password</h2>
  <p>We received a request to reset your password. Click the button below to choose a new password:</p>
  <p><a href="{{.ResetURL}}" style="display:inline-block;padding:12px 20px;background:#2563eb;color:#fff;text-decoration:none;border-radius:6px;">Reset password</a></p>
  <p>Or copy this link into your browser:<br><a href="{{.ResetURL}}">{{.ResetURL}}</a></p>
  <p style="color:#666;font-size:12px;">This link expires soon. If you did not request a reset, ignore this email.</p>
  <p style="color:#666;font-size:12px;">&copy; {{.Year}} Auth Service</p>
</body>
</html>`

func renderTemplate(tmpl string, data any) (string, error) {
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
