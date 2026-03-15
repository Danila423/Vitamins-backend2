package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
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
	dialer := &net.Dialer{}

	var conn net.Conn
	var err error
	network := "tcp4"
	if m.port == "465" {
		fmt.Printf("smtp: dialing tls %s\n", addr)
		td := &tls.Dialer{
			NetDialer: dialer,
			Config:    &tls.Config{ServerName: m.host},
		}
		conn, err = td.DialContext(ctx, network, addr)
	} else {
		fmt.Printf("smtp: dialing %s\n", addr)
		conn, err = dialer.DialContext(ctx, network, addr)
	}
	if err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	fmt.Printf("smtp: connected %s\n", addr)

	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	if m.port != "465" {
		if ok, _ := c.Extension("STARTTLS"); ok {
			fmt.Printf("smtp: starttls %s\n", addr)
			if err := c.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
				_ = c.Close()
				return err
			}
		}
	}
	defer c.Close()

	auth := smtp.PlainAuth("", m.user, m.pass, m.host)
	fmt.Printf("smtp: auth %s\n", m.host)
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

	fmt.Printf("smtp: mail from %s\n", fromAddr.Address)
	if err := c.Mail(fromAddr.Address); err != nil {
		return err
	}
	fmt.Printf("smtp: rcpt to %s\n", toAddr.Address)
	if err := c.Rcpt(toAddr.Address); err != nil {
		return err
	}
	fmt.Printf("smtp: data\n")
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

	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	if err := c.Quit(); err != nil {
		// Некоторые SMTP-серверы отвечают 250 OK на QUIT, net/smtp трактует это как ошибку.
		fmt.Printf("smtp: quit warning: %v\n", err)
	}
	return nil
}
