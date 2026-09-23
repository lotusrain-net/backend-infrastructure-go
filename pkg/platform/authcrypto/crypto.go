// Package authcrypto implements authenticated secret encryption and RFC 6238.
package authcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // RFC 6238 interoperability with authenticator applications.
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
)

type Crypto struct {
	aead   cipher.AEAD
	pepper []byte
	issuer string
}

func New(key, pepper []byte, issuer string) (*Crypto, error) {
	if len(key) != 32 || len(pepper) < 32 {
		return nil, errors.New("authentication encryption key and pepper must be 32 bytes")
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	aead, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	return &Crypto{aead: aead, pepper: append([]byte(nil), pepper...), issuer: issuer}, nil
}
func (c *Crypto) Encrypt(secret, user string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, e := rand.Read(nonce); e != nil {
		return nil, e
	}
	return c.aead.Seal(nonce, nonce, []byte(secret), []byte(user)), nil
}
func (c *Crypto) Decrypt(raw []byte, user string) (string, error) {
	n := c.aead.NonceSize()
	if len(raw) < n {
		return "", iam.ErrInvalidCode
	}
	v, e := c.aead.Open(nil, raw[:n], raw[n:], []byte(user))
	if e != nil {
		return "", iam.ErrInvalidCode
	}
	return string(v), nil
}
func (c *Crypto) NewSecret(email string) (string, string, error) {
	b := make([]byte, 20)
	if _, e := rand.Read(b); e != nil {
		return "", "", e
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	u := url.URL{Scheme: "otpauth", Host: "totp", Path: "/" + c.issuer + ":" + email}
	q := url.Values{"secret": {secret}, "issuer": {c.issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	u.RawQuery = q.Encode()
	return secret, u.String(), nil
}
func (c *Crypto) Verify(secret, code string, now time.Time, last int64) (int64, error) {
	if len(code) != 6 {
		return 0, iam.ErrInvalidCode
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return 0, iam.ErrInvalidCode
		}
	}
	key, e := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if e != nil {
		return 0, iam.ErrInvalidCode
	}
	current := now.Unix() / 30
	for _, offset := range []int64{0, -1, 1} {
		step := current + offset
		if step <= last || step < 0 {
			continue
		}
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(step))
		m := hmac.New(sha1.New, key)
		m.Write(b[:])
		digest := m.Sum(nil)
		o := digest[len(digest)-1] & 15
		v := binary.BigEndian.Uint32(digest[o:o+4]) & 0x7fffffff
		expected := fmt.Sprintf("%06d", v%1000000)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return step, nil
		}
	}
	return 0, iam.ErrInvalidCode
}
func (c *Crypto) RecoveryHash(code string) string {
	h := hmac.New(sha256.New, c.pepper)
	h.Write([]byte("recovery:" + strings.ToLower(strings.TrimSpace(code))))
	return hex.EncodeToString(h.Sum(nil))
}
func (c *Crypto) RecoveryCodes() ([]string, []string, error) {
	codes := make([]string, 10)
	hashes := make([]string, 10)
	for i := range codes {
		b := make([]byte, 16)
		if _, e := rand.Read(b); e != nil {
			return nil, nil, e
		}
		codes[i] = hex.EncodeToString(b)
		hashes[i] = c.RecoveryHash(codes[i])
	}
	return codes, hashes, nil
}
