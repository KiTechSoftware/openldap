package status

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mockCommand allows overriding exec.CommandContext during test.
var (
	origLookPath  = exec.LookPath
	origCommand   = exec.CommandContext
	fakeCmdOutput = map[string]string{}
)

func fakeLookPath(name string) (string, error) {
	// Pretend every command exists
	return "/usr/bin/" + name, nil
}

func fakeCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", name}
	cs = append(cs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess is a helper subprocess that mimics command outputs.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i := 0; i < len(args); i++ {
		if args[i] == "--" && i+1 < len(args) {
			cmd := args[i+1]
			out := fakeCmdOutput[cmd]
			os.Stdout.WriteString(out)
			os.Exit(0)
		}
	}
	os.Exit(1)
}

// --- Tests ---

func TestCollectProducesValidJSON(t *testing.T) {
	// Patch exec
	origLookPath = fakeLookPath
	origCommand = fakeCommandContext
	defer func() {
		origLookPath = origLookPath
		origCommand = origCommand
	}()

	fakeCmdOutput = map[string]string{
		"systemctl": "ActiveEnterTimestamp=Wed 2025-10-29 10:00:00 UTC\n",
		"ss":        "LISTEN 0 128 *:389 *:* users:((",
		"ldapsearch": `namingContexts: dc=example,dc=org
namingContexts: dc=example,dc=com`,
		"openssl": "notAfter=Jan 2 15:04:05 2026 GMT\n",
	}

	ctx := context.Background()
	r, err := Collect(ctx)
	require.NoError(t, err)

	// Marshal and ensure it's valid JSON
	data, err := json.Marshal(r)
	require.NoError(t, err)
	require.True(t, json.Valid(data), "status report must marshal to valid JSON")

	// Assert expected fields
	require.NotEmpty(t, r.ConfigVersion)
	require.True(t, r.Ports.LDAP)
	require.True(t, r.TLS.Exists)
	require.True(t, strings.HasPrefix(r.BaseDN, "dc=example"), "BaseDN parsed correctly")
	require.NotEmpty(t, r.TLS.ExpiryDate)
	require.NotZero(t, r.TLS.ExpiresInDays)
}

func TestGetBuildInfoFields(t *testing.T) {
	b := GetBuildInfo()
	require.NotEmpty(t, b.Version)
	require.NotEmpty(t, b.Platform)
	require.True(t, strings.Contains(b.Platform, runtime.GOOS))
	require.True(t, strings.Contains(b.Platform, runtime.GOARCH))
	_, err := time.Parse(time.RFC3339, b.Timestamp)
	require.NoError(t, err, "timestamp must be RFC3339")
}

func TestGetLastBackup(t *testing.T) {
	tmp := t.TempDir()

	// create fake backups
	file1 := filepath.Join(tmp, "b1.ldif")
	file2 := filepath.Join(tmp, "b2.ldif")

	os.WriteFile(file1, []byte("a"), 0o644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(file2, []byte("b"), 0o644)

	last := getLastBackup(tmp)
	require.Equal(t, file2, last)
}
