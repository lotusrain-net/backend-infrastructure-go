package config

import (
	"encoding/hex"
	"errors"
	"net/mail"
	"strconv"
	"strings"
)

func (c Config) ValidateAuthentication() error {
	if c.RecoveryCodeTTL <= 0 {
		return errors.New("RECOVERY_CODE_TTL must be positive")
	}
	for name, value := range map[string]string{"AUTHENTICATION_KEY": c.AuthenticationKey, "AUTHENTICATION_PEPPER": c.AuthenticationPepper} {
		decoded, e := hex.DecodeString(value)
		if e != nil || len(decoded) != 32 {
			return errors.New(name + " must be 64 hexadecimal characters (32 random bytes)")
		}
	}
	if c.AuthenticationKey == c.AuthenticationPepper {
		return errors.New("authentication encryption key and pepper must be independent")
	}
	if !c.SMTPEnabled {
		if c.Environment == "production" {
			return errors.New("SMTP_ENABLED must be true in production")
		}
		return nil
	}
	port, e := strconv.Atoi(c.SMTPPort)
	if e != nil || port < 1 || port > 65535 || c.SMTPHost == "" || strings.ContainsAny(c.SMTPHost, "\r\n /@") {
		return errors.New("SMTP_HOST and SMTP_PORT must be valid")
	}
	if c.SMTPUsername == "" || c.SMTPPassword == "" || c.SMTPTimeout <= 0 {
		return errors.New("SMTP credentials and positive SMTP_TIMEOUT are required")
	}
	sender, e := mail.ParseAddress(c.SMTPFrom)
	if e != nil || sender.Address != c.SMTPFrom {
		return errors.New("SMTP_FROM must be a plain email address")
	}
	if c.SMTPTLSMode != "tls" && c.SMTPTLSMode != "starttls" && c.SMTPTLSMode != "plain" {
		return errors.New("SMTP_TLS_MODE must be plain, tls or starttls")
	}
	if c.Environment == "production" && isPlaceholderSecret(c.SMTPPassword) {
		return errors.New("SMTP_PASSWORD must not use a public placeholder")
	}
	return nil
}
