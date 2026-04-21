package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	appLogger "vitamins-backend_2/internal/logger"
)

type SMTPMailer struct {
	host string
	port string
	user string
	pass string
	from string
}

func NewSMTPMailer(host, port, user, pass, from string) *SMTPMailer {
	return &SMTPMailer{
		host: host,
		port: port,
		user: user,
		pass: pass,
		from: from,
	}
}

func (m *SMTPMailer) SendPasswordResetCode(ctx context.Context, toEmail, code string) error {
	if m == nil || m.host == "" || m.port == "" || m.user == "" || m.pass == "" || m.from == "" {
		return fmt.Errorf("smtp not configured")
	}

	addr := net.JoinHostPort(m.host, m.port)

	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	log := appLogger.WithContext(slog.Default(), ctx).With(
		"channel", "app",
		"operation", "mailer.smtp.send_password_reset_code",
		"smtp.host", m.host,
		"to.email_masked", appLogger.MaskEmail(toEmail),
	)

	dialer := &net.Dialer{}

	var conn net.Conn
	var err error

	// Don't force IPv4; let the OS/container decide.
	network := "tcp"

	// Port 465 = implicit TLS, other ports = plain TCP + STARTTLS (if supported/required).
	if m.port == "465" {
		log.Debug("smtp dialing tls", "smtp.addr", addr)
		td := &tls.Dialer{
			NetDialer: dialer,
			Config:    &tls.Config{ServerName: m.host},
		}
		conn, err = td.DialContext(ctx, network, addr)
	} else {
		log.Debug("smtp dialing", "smtp.addr", addr)
		conn, err = dialer.DialContext(ctx, network, addr)
	}

	if err != nil {
		return err
	}

	// Ensure the whole connection respects the context deadline.
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	log.Debug("smtp connected", "smtp.addr", addr)

	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer c.Close()

	// Explicit EHLO/HELO (helps some servers behave more predictably).
	if err := c.Hello("vitamins"); err != nil {
		return err
	}

	// For STARTTLS ports, require STARTTLS and enable it.
	if m.port == "587" || m.port == "2525" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("smtp: server does not support STARTTLS")
		}
		log.Debug("smtp starttls", "smtp.addr", addr)
		if err := c.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
			return err
		}
	}

	auth := smtp.PlainAuth("", m.user, m.pass, m.host)
	log.Debug("smtp auth")
	if err := c.Auth(auth); err != nil {
		return err
	}

	fromAddr, err := mail.ParseAddress(m.from)
	if err != nil {
		return err
	}
	toAddr, err := mail.ParseAddress(toEmail)
	if err != nil {
		return err
	}

	log.Debug("smtp mail from", "from.email_masked", appLogger.MaskEmail(fromAddr.Address))
	if err := c.Mail(fromAddr.Address); err != nil {
		return err
	}
	log.Debug("smtp rcpt to", "to.email_masked", appLogger.MaskEmail(toAddr.Address))
	if err := c.Rcpt(toAddr.Address); err != nil {
		return err
	}

	log.Debug("smtp data")
	w, err := c.Data()
	if err != nil {
		return err
	}
	defer w.Close()

	subject := "Password reset code"
	body := fmt.Sprintf("Your password reset code: %s", code)

	msg := strings.Join([]string{
		fmt.Sprintf("From: %s", m.from),
		fmt.Sprintf("To: %s", toAddr.String()),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
		"",
	}, "\r\n")

	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}

	return c.Quit()
}
