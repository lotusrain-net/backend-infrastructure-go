package iam

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AccessClaims struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type JWTManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

func NewJWTManager(secret []byte, issuer string, ttl time.Duration) (*JWTManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT secret must contain at least 32 bytes")
	}
	if strings.TrimSpace(issuer) == "" || ttl <= 0 {
		return nil, errors.New("JWT issuer and positive TTL are required")
	}
	return &JWTManager{secret: append([]byte(nil), secret...), issuer: issuer, ttl: ttl, now: time.Now}, nil
}

func (m *JWTManager) Issue(subject string) (string, error) {
	if strings.TrimSpace(subject) == "" {
		return "", errors.New("token subject is required")
	}
	now := m.now()
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(AccessClaims{Issuer: m.issuer, Subject: subject, IssuedAt: now.Unix(), ExpiresAt: now.Add(m.ttl).Unix()})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return unsigned + "." + m.sign(unsigned), nil
}

func (m *JWTManager) Parse(raw string) (AccessClaims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return AccessClaims{}, errors.New("invalid access token")
	}
	if !hmac.Equal([]byte(parts[2]), []byte(m.sign(parts[0]+"."+parts[1]))) {
		return AccessClaims{}, errors.New("invalid access token signature")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AccessClaims{}, errors.New("invalid access token header")
	}
	var header map[string]string
	if json.Unmarshal(headerBytes, &header) != nil || header["alg"] != "HS256" || header["typ"] != "JWT" {
		return AccessClaims{}, errors.New("unsupported access token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AccessClaims{}, errors.New("invalid access token payload")
	}
	var claims AccessClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return AccessClaims{}, fmt.Errorf("decode access token: %w", err)
	}
	if claims.Issuer != m.issuer || claims.Subject == "" || claims.ExpiresAt <= m.now().Unix() {
		return AccessClaims{}, errors.New("access token expired or invalid")
	}
	return claims, nil
}

func (m *JWTManager) sign(value string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
