package authcrypto

import (
	"bytes"
	"testing"
	"time"
)

func TestEncryptionBindsUserAndRejectsTampering(t *testing.T) {
	c, e := New(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32), "Console")
	if e != nil {
		t.Fatal(e)
	}
	encrypted, e := c.Encrypt("secret", "user-1")
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(encrypted, []byte("secret")) {
		t.Fatal("plaintext")
	}
	if _, e = c.Decrypt(encrypted, "user-2"); e == nil {
		t.Fatal("cross-user decryption")
	}
	plain, e := c.Decrypt(encrypted, "user-1")
	if e != nil || plain != "secret" {
		t.Fatal(plain, e)
	}
	encrypted[len(encrypted)-1] ^= 1
	if _, e = c.Decrypt(encrypted, "user-1"); e == nil {
		t.Fatal("tampering accepted")
	}
}
func TestRFC6238AndReplay(t *testing.T) {
	c, _ := New(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32), "Console")
	step, e := c.Verify("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "287082", time.Unix(59, 0), -1)
	if e != nil || step != 1 {
		t.Fatal(step, e)
	}
	if _, e = c.Verify("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "287082", time.Unix(59, 0), step); e == nil {
		t.Fatal("replay accepted")
	}
	if _, e = c.Verify("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "287082", time.Unix(180, 0), -1); e == nil {
		t.Fatal("expired code accepted")
	}
	codes, hashes, e := c.RecoveryCodes()
	if e != nil || len(codes) != 10 {
		t.Fatal(e)
	}
	for i, code := range codes {
		if code == hashes[i] || c.RecoveryHash(code) != hashes[i] {
			t.Fatal("invalid recovery storage")
		}
	}
}
