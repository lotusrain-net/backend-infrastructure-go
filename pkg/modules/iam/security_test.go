package iam

import (
	"strings"
	"testing"
	"time"
)

func TestArgon2PasswordRoundTripAndWrongPassword(t *testing.T) {
	hasher := NewPasswordHasher(DefaultArgon2Params())
	hash, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("unexpected hash: %s", hash)
	}
	ok, err := hasher.Verify("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Fatalf("verify=%v err=%v", ok, err)
	}
	ok, err = hasher.Verify("wrong", hash)
	if err != nil || ok {
		t.Fatalf("wrong password verify=%v err=%v", ok, err)
	}
}

func TestArgon2RejectsMalformedHash(t *testing.T) {
	_, err := NewPasswordHasher(DefaultArgon2Params()).Verify("password", "$argon2id$broken")
	if err == nil {
		t.Fatal("expected malformed hash error")
	}
}

func TestArgon2RejectsUnsafeEncodedParameters(t *testing.T) {
	encoded := "$argon2id$v=19$m=1048576,t=3,p=2$c2FsdHNhbHRzYWx0c2FsdA$ZmFrZWtleQ"
	if _, err := NewPasswordHasher(DefaultArgon2Params()).Verify("password", encoded); err == nil {
		t.Fatal("expected unsafe memory parameter rejection")
	}
}

func TestJWTIssueParseTamperAndExpiry(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tokens, err := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "backend", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	tokens.now = func() time.Time { return now }
	raw, err := tokens.Issue("2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := tokens.Parse(raw)
	if err != nil || claims.Subject != "2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a" {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	if _, err := tokens.Parse(raw[:len(raw)-1] + "x"); err == nil {
		t.Fatal("tampered token accepted")
	}
	tokens.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := tokens.Parse(raw); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestJWTRejectsWeakSecret(t *testing.T) {
	if _, err := NewJWTManager([]byte("short"), "backend", time.Minute); err == nil {
		t.Fatal("expected weak secret rejection")
	}
}

func TestPermissionMatching(t *testing.T) {
	tests := []struct {
		granted  []string
		required string
		want     bool
	}{
		{[]string{"users:read"}, "users:read", true},
		{[]string{"users:*"}, "users:write", true},
		{[]string{"*"}, "audit:read", true},
		{[]string{"users:read"}, "users:write", false},
		{[]string{"user*"}, "users:read", false},
	}
	for _, tt := range tests {
		if got := HasPermission(tt.granted, tt.required); got != tt.want {
			t.Fatalf("%v requires %q: got %v", tt.granted, tt.required, got)
		}
	}
}
