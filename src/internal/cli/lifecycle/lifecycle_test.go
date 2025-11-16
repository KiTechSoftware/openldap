package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kitechsoftware/ldappy/internal/common/config"
	"github.com/kitechsoftware/ldappy/internal/common/core"
	"github.com/stretchr/testify/assert"
)

// ---- Test scaffolding ----

// mock shell.RunContext
var originalRunContext = shellRun
var shellRun = func(ctx context.Context, cmd string, args ...string) error { return nil }

// mock verify.LdapIsActive
var originalVerifyLdapIsActive = verifyLdapIsActive
var verifyLdapIsActive = func() error { return nil }

func setupMocks() {
	shellRun = func(ctx context.Context, cmd string, args ...string) error { return nil }
	verifyLdapIsActive = func() error { return nil }
	// ensureConfig = func(path string) (*config.Config, error) {
	// 	c := config.Default()
	// 	c.LDAP.Domain = "example.org"
	// 	return &c, nil
	// }
}

func teardownMocks() {
	shellRun = originalRunContext
	verifyLdapIsActive = originalVerifyLdapIsActive
	// ensureConfig = originalEnsureConfig
}

// ---- Actual tests ----

func TestReportHelpers(t *testing.T) {
	r := startReport("install")
	assert.Equal(t, "install", r.Action)

	r.SuccessMsg("ok")
	assert.True(t, r.Success)
	assert.Equal(t, "ok", r.Message)

	r.Fail("bad", errors.New("boom"))
	assert.False(t, r.Success)
	assert.Contains(t, r.ErrorMsg, "boom")
}

func TestServiceStatusSuccess(t *testing.T) {
	setupMocks()
	defer teardownMocks()

	ctx := context.Background()
	report := Service(ctx, "status", false, false, false)
	assert.True(t, report.Success)
	assert.Contains(t, report.Message, "active")
}

func TestServiceStatusFail(t *testing.T) {
	setupMocks()
	defer teardownMocks()

	verifyLdapIsActive = func() error { return errors.New("not running") }

	ctx := context.Background()
	report := Service(ctx, "status", false, false, false)
	assert.False(t, report.Success)
	assert.Contains(t, report.ErrorMsg, "not running")
}

func TestServiceActionSuccess(t *testing.T) {
	setupMocks()
	defer teardownMocks()

	ctx := context.Background()
	report := Service(ctx, "restart", false, false, false)
	assert.True(t, report.Success)
	assert.Contains(t, report.Message, "successfully")
}

func TestServiceActionFail(t *testing.T) {
	setupMocks()
	defer teardownMocks()

	shellRun = func(ctx context.Context, cmd string, args ...string) error {
		return errors.New("systemctl failed")
	}
	ctx := context.Background()
	report := Service(ctx, "restart", false, false, false)
	assert.False(t, report.Success)
	assert.Contains(t, report.ErrorMsg, "systemctl failed")
}

func TestLoadContextCanceled(t *testing.T) {
	setupMocks()
	defer teardownMocks()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg, err := core.Load(ctx, "dummy")
	assert.Nil(t, cfg)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPurgeSuccess(t *testing.T) {
	setupMocks()
	defer teardownMocks()

	ctx := context.Background()
	report := Purge(ctx, false)
	assert.True(t, report.Success)
	assert.Contains(t, report.Message, "ready")
}

func TestInstallSuccess(t *testing.T) {
	setupMocks()
	defer teardownMocks()

	ctx := context.Background()
	cfg := config.Default()
	report := Install(ctx, &cfg, false, false)
	assert.True(t, report.Success)
}

func TestUpgradeSuccess(t *testing.T) {
	setupMocks()
	defer teardownMocks()

	ctx := context.Background()
	report := Upgrade(ctx, false)
	assert.True(t, report.Success)
	assert.Contains(t, report.Message, "Upgrade complete")
}

func TestInitSuccess(t *testing.T) {
	setupMocks()
	defer teardownMocks()

	ctx := context.Background()
	cfg := config.Default()
	report := Init(ctx, &cfg, false, true) // force=true to skip /etc/ldap check
	assert.True(t, report.Success)
}

// func TestInitFailOnPasswordHash(t *testing.T) {
// 	setupMocks()
// 	defer teardownMocks()

// 	ctx := context.Background()
// 	cfg := config.Default()
// 	cfg.Modules.PasswordHashing = true

// 	// Simulate hashing failure
// 	securityHashPasswordContext = func(ctx context.Context, pwd string) (string, error) {
// 		return "", errors.New("hash error")
// 	}
// 	defer func() { securityHashPasswordContext = security.HashPasswordContext }()

// 	report := Init(ctx, &cfg, false, true)
// 	assert.False(t, report.Success)
// 	assert.Contains(t, report.ErrorMsg, "hash error")
// }

func TestContextCancellation(t *testing.T) {
	setupMocks()
	defer teardownMocks()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	cancel()

	report := Purge(ctx, false)
	assert.NotNil(t, report)
}
