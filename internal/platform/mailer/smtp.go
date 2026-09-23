// Package mailer owns SMTP transport. Errors never include recipients or codes.
package mailer

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

var ErrTLS = errors.New("SMTP certificate or TLS negotiation failed")
var ErrAuthentication = errors.New("SMTP authentication failed")
var ErrTimeout = errors.New("SMTP timeout")
var ErrDelivery = errors.New("SMTP delivery failed")

type Config struct {
	Enabled                                       bool
	Host, Port, Username, Password, From, TLSMode string
	Timeout                                       time.Duration
	TLSConfig                                     *tls.Config
}
type SMTP struct {
	cfg    Config
	logger *slog.Logger
	pepper []byte
}

func New(cfg Config, logger *slog.Logger, pepper []byte) *SMTP {
	return &SMTP{cfg: cfg, logger: logger, pepper: pepper}
}
func (s *SMTP) SendCode(ctx context.Context, email, purpose, code string) (result error) {
	if !s.cfg.Enabled {
		if s.logger != nil {
			h := hmac.New(sha256.New, s.pepper)
			h.Write([]byte(email))
			s.logger.InfoContext(ctx, "development email_code stored", "recipient_digest", hex.EncodeToString(h.Sum(nil)), "purpose", purpose)
		}
		return nil
	}
	recipient, e := mail.ParseAddress(email)
	if e != nil || recipient.Address != email || strings.ContainsAny(email, "\r\n") {
		return ErrDelivery
	}
	sender, e := mail.ParseAddress(s.cfg.From)
	if e != nil {
		return ErrDelivery
	}
	timeout := s.cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	defer func() {
		if result != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result = ErrTimeout
		}
	}()
	tlsConfig := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}
	if s.cfg.TLSConfig != nil {
		tlsConfig = s.cfg.TLSConfig.Clone()
		tlsConfig.ServerName = s.cfg.Host
		tlsConfig.MinVersion = tls.VersionTLS12
		tlsConfig.InsecureSkipVerify = false
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, e := dialer.DialContext(ctx, "tcp", net.JoinHostPort(s.cfg.Host, s.cfg.Port))
	if e != nil {
		return transportError(e, ErrDelivery)
	}
	defer conn.Close()
	rawConn := conn
	stop := context.AfterFunc(ctx, func() { _ = rawConn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	if s.cfg.TLSMode == "tls" {
		secure := tls.Client(conn, tlsConfig)
		if e = secure.HandshakeContext(ctx); e != nil {
			return transportError(e, ErrTLS)
		}
		conn = secure
	}
	client, e := smtp.NewClient(conn, s.cfg.Host)
	if e != nil {
		return transportError(e, ErrDelivery)
	}
	defer client.Close()
	if s.cfg.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return ErrTLS
		}
		if e = client.StartTLS(tlsConfig); e != nil {
			return transportError(e, ErrTLS)
		}
	} else if s.cfg.TLSMode != "tls" {
		return ErrTLS
	}
	if e = client.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); e != nil {
		return transportError(e, ErrAuthentication)
	}
	if e = client.Mail(sender.Address); e != nil {
		return transportError(e, ErrDelivery)
	}
	if e = client.Rcpt(recipient.Address); e != nil {
		return transportError(e, ErrDelivery)
	}
	writer, e := client.Data()
	if e != nil {
		return transportError(e, ErrDelivery)
	}
	_, e = fmt.Fprintf(writer, "From: %s\r\nTo: %s\r\nSubject: Your verification code\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nYour verification code is %s. It expires in 10 minutes.\r\n", sender.Address, recipient.Address, code)
	if e != nil {
		return transportError(e, ErrDelivery)
	}
	if e = writer.Close(); e != nil {
		return transportError(e, ErrDelivery)
	}
	return transportError(client.Quit(), ErrDelivery)
}
func transportError(err, fallback error) error {
	if err == nil {
		return nil
	}
	var n net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &n) && n.Timeout()) {
		return ErrTimeout
	}
	return fallback
}
