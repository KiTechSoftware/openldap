package security

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestGeneratePassword ensures we generate a valid random string of the correct length.
func TestGeneratePassword(t *testing.T) {
	pass, err := GeneratePassword(32)
	if err != nil {
		t.Fatalf("GeneratePassword failed: %v", err)
	}

	if len(pass) != 32 {
		t.Errorf("expected password length 32, got %d (%q)", len(pass), pass)
	}

	pass2, _ := GeneratePassword(32)
	if pass == pass2 {
		t.Errorf("expected unique passwords, got identical values: %q", pass)
	}
}

// TestHashPasswordContext_Default ensures {SSHA} fallback and slappasswd work.
func TestHashPasswordContext_Default(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hash, err := HashPasswordContext(ctx, "supersecret")
	if err != nil {
		t.Fatalf("HashPasswordContext failed: %v", err)
	}

	if hash == "" {
		t.Fatalf("expected non-empty hash output")
	}
	if !strings.HasPrefix(hash, "{") {
		t.Errorf("expected LDAP-style hash (e.g. {SSHA}), got: %s", hash)
	}
}

// TestHashPasswordContext_ExplicitSchemes tries {SHA512} and {ARGON2} if slappasswd is present.
func TestHashPasswordContext_SHA512Fallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hash, err := HashPasswordContext(ctx, "secretpass")
	if err != nil {
		t.Fatalf("HashPasswordContext failed: %v", err)
	}

	if !strings.HasPrefix(hash, "{SHA512}") && !strings.HasPrefix(hash, "{SSHA}") {
		t.Errorf("expected {SHA512} or {SSHA} prefix, got: %s", hash)
	}
}

// TestHashPasswordSSHAFallback validates internal {SSHA} fallback.
func TestHashPasswordSSHAFallback(t *testing.T) {
	hash := hashPasswordSSHA("localpass")
	if !strings.HasPrefix(hash, "{SSHA}") {
		t.Errorf("expected {SSHA} prefix, got: %s", hash)
	}
	if len(hash) < 20 {
		t.Errorf("hash too short: %s", hash)
	}
}
