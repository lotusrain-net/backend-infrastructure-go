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
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"log/slog"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

var ErrTLS = fmt.Errorf("%w: SMTP certificate or TLS negotiation failed", iam.ErrAuthenticationUnavailable)
var ErrAuthentication = fmt.Errorf("%w: SMTP authentication failed", iam.ErrAuthenticationUnavailable)
var ErrTimeout = fmt.Errorf("%w: SMTP timeout", iam.ErrAuthenticationUnavailable)
var ErrDelivery = iam.ErrMailRejected

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
func (s *SMTP) SendCode(ctx context.Context, email string, purpose iam.EmailCodePurpose, code string) (result error) {
	defer func() {
		if result != nil && s.logger != nil {
			s.logger.WarnContext(ctx, "email_code delivery failed", "recipient_digest", s.recipientDigest(email), "purpose", purpose, "error", result.Error())
		}
	}()
	if !s.cfg.Enabled {
		if s.logger != nil {
			s.logger.InfoContext(ctx, "development email_code stored", "recipient_digest", s.recipientDigest(email), "purpose", purpose)
		}
		return nil
	}
	recipient, e := mail.ParseAddress(email)
	if e != nil || recipient.Address != email || strings.ContainsAny(email, "\r\n") {
		return ErrDelivery
	}
	sender, e := mail.ParseAddress(s.cfg.From)
	if e != nil {
		return iam.ErrAuthenticationUnavailable
	}
	if s.cfg.TLSMode != "plain" && s.cfg.TLSMode != "tls" && s.cfg.TLSMode != "starttls" {
		return ErrTLS
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
		} else if result != nil && errors.Is(ctx.Err(), context.Canceled) {
			result = context.Canceled
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
		return transportError(e, iam.ErrAuthenticationUnavailable)
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
		return transportError(e, iam.ErrAuthenticationUnavailable)
	}
	defer client.Close()
	if s.cfg.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return ErrTLS
		}
		if e = client.StartTLS(tlsConfig); e != nil {
			return transportError(e, ErrTLS)
		}
	}
	var auth smtp.Auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	if s.cfg.TLSMode == "plain" {
		auth = explicitPlainAuth{host: s.cfg.Host, username: s.cfg.Username, password: s.cfg.Password}
	}
	if e = client.Auth(auth); e != nil {
		return transportError(e, ErrAuthentication)
	}
	if e = client.Mail(sender.Address); e != nil {
		return transportError(e, iam.ErrAuthenticationUnavailable)
	}
	if e = client.Rcpt(recipient.Address); e != nil {
		return deliveryError(e)
	}
	writer, e := client.Data()
	if e != nil {
		return deliveryError(e)
	}
	_, e = fmt.Fprintf(writer, "From: %s\r\nTo: %s\r\nSubject: Your verification code\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nYour verification code is %s. It expires in 10 minutes.\r\n", sender.Address, recipient.Address, code)
	if e != nil {
		return transportError(e, iam.ErrAuthenticationUnavailable)
	}
	if e = writer.Close(); e != nil {
		return deliveryError(e)
	}
	return transportError(client.Quit(), iam.ErrAuthenticationUnavailable)
}

func (s *SMTP) recipientDigest(email string) string {
	h := hmac.New(sha256.New, s.pepper)
	h.Write([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(h.Sum(nil))
}

// Only explicit plain transport permits credentials on a non-TLS connection.
// Host binding is retained; TLS transports use the standard library auth policy.
type explicitPlainAuth struct{ host, username, password string }

func (a explicitPlainAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if server.Name != a.host {
		return "", nil, ErrAuthentication
	}
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}
func (a explicitPlainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, ErrAuthentication
	}
	return nil, nil
}

func deliveryError(err error) error {
	var response *textproto.Error
	if errors.As(err, &response) {
		switch response.Code {
		case 450, 452, 550, 551, 552, 553, 554:
			return ErrDelivery
		}
	}
	return transportError(err, iam.ErrAuthenticationUnavailable)
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
