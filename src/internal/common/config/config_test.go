package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var osUserHomeDir = os.UserHomeDir

func TestDefaultsAndEnvOverride(t *testing.T) {
	os.Setenv("LDAP_DOMAIN", "acme.org")
	defer os.Unsetenv("LDAP_DOMAIN")

	cfg := Default()
	cfg.envOverride()

	assert.Equal(t, "acme.org", cfg.LDAP.Domain)
	assert.Equal(t, "dc=acme,dc=org", cfg.LDAP.BaseDN)
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := Default()
	err := Save(path, &cfg)
	assert.NoError(t, err)

	loaded, err := Load(path)
	assert.NoError(t, err)
	assert.Equal(t, cfg.LDAP.Domain, loaded.LDAP.Domain)
	assert.Equal(t, cfg.LDAP.Organization, loaded.LDAP.Organization)
	assert.Equal(t, cfg.LDAP.BaseDN, loaded.LDAP.BaseDN)
	assert.NoError(t, loaded.Validate())
}

func TestSaveCtxAndLoadCtx(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ctx.toml")
	cfg := Default()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	assert.NoError(t, SaveCtx(ctx, path, &cfg))

	loaded, err := LoadCtx(ctx, path)
	assert.NoError(t, err)
	assert.Equal(t, cfg.LDAP.Domain, loaded.LDAP.Domain)
}

func TestLoadCtxCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := LoadCtx(ctx, "does-not-matter.toml")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestValidateTLS(t *testing.T) {
	cfg := Default()
	cfg.TLS.Method = "invalid"
	err := cfg.Validate()
	assert.ErrorContains(t, err, "invalid tls.method")
}

func TestConfigDirFallbacks(t *testing.T) {
	// 1. Default behavior (HOME-based)
	os.Unsetenv("LDAP_DATA_DIR")
	os.Unsetenv("XDG_CONFIG_HOME")
	dir := ConfigDir()
	assert.Contains(t, dir, ".config/ldappy")

	// 2. XDG_CONFIG_HOME
	os.Setenv("XDG_CONFIG_HOME", "/tmp/testcfg")
	assert.Equal(t, "/tmp/testcfg/ldappy", ConfigDir())
	os.Unsetenv("XDG_CONFIG_HOME")

	// 3. LDAP_DATA_DIR overrides everything
	os.Setenv("LDAP_DATA_DIR", "/opt/ldappy/conf")
	assert.Equal(t, "/opt/ldappy/conf", ConfigDir())
	os.Unsetenv("LDAP_DATA_DIR")

	// 4. No HOME fallback (simulate container)
	oldHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	// Temporarily override os.UserHomeDir
	oldUserHomeDir := osUserHomeDir
	osUserHomeDir = func() (string, error) { return "", os.ErrNotExist }
	defer func() { osUserHomeDir = oldUserHomeDir; os.Setenv("HOME", oldHome) }()

	fallback := ConfigDir()
	assert.Equal(t, "/tmp/ldappy", fallback)
}

func TestDefaultSummary(t *testing.T) {
	cfg := Default()
	human := cfg.Summary()
	jsonOut := cfg.SummaryJSON()

	assert.Contains(t, human, "example.com")
	assert.Contains(t, jsonOut, `"domain": "example.com"`)
}
