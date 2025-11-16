package security

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/kitechsoftware/ldappy/internal/common/shell"
)

// GeneratePassword creates a cryptographically secure random password.
func GeneratePassword(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random password: %w", err)
	}
	pass := base64.RawURLEncoding.EncodeToString(bytes)
	if len(pass) > n {
		pass = pass[:n]
	}
	return pass, nil
}

// HashPassword hashes a password with default context and SHA512/SSHA logic.
func HashPassword(pass string) (string, error) {
	return HashPasswordContext(context.Background(), pass)
}

// HashPasswordContext hashes the password using OpenLDAP’s slappasswd command.
// Preferred scheme: {SHA512}, fallback: {SSHA}.
// This function only produces hashes that OpenLDAP can natively verify.
func HashPasswordContext(ctx context.Context, pass string) (string, error) {
	if pass == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	// Check if slappasswd exists
	if _, err := shell.CommandContext(ctx, "which", "slappasswd"); err != nil {
		// No slappasswd — fallback to {SSHA}
		return hashPasswordSSHA(pass), nil
	}

	// Try SHA512 first
	out, err := shell.CommandContext(ctx, "slappasswd", "-h", "{SHA512}", "-s", pass)
	if err == nil && strings.HasPrefix(strings.TrimSpace(out), "{SHA512}") {
		return strings.TrimSpace(out), nil
	}

	// Fallback to SSHA if SHA512 unsupported or failed
	out, err = shell.CommandContext(ctx, "slappasswd", "-h", "{SSHA}", "-s", pass)
	if err == nil && strings.HasPrefix(strings.TrimSpace(out), "{SSHA}") {
		return strings.TrimSpace(out), nil
	}

	// Final fallback — local {SSHA} implementation
	return hashPasswordSSHA(pass), nil
}

// hashPasswordSSHA provides a pure-Go {SSHA} fallback.
func hashPasswordSSHA(pass string) string {
	salt := make([]byte, 4)
	_, _ = rand.Read(salt)
	h := sha1.Sum(append([]byte(pass), salt...))
	return fmt.Sprintf("{SSHA}%s", base64.StdEncoding.EncodeToString(append(h[:], salt...)))
}
