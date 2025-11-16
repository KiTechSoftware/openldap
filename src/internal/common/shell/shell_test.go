package shell

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCommandAndRun(t *testing.T) {
	out, err := Command("echo", "hello")
	assert.NoError(t, err)
	assert.Contains(t, out, "hello")

	// Run should stream output successfully
	assert.NoError(t, Run("echo", "world"))
}

func TestCommandContextAndCapture(t *testing.T) {
	ctx := context.Background()
	out, err := CommandContext(ctx, "echo", "ctx")
	assert.NoError(t, err)
	assert.Contains(t, out, "ctx")

	out2, err := RunCapture(ctx, "echo", "capture")
	assert.NoError(t, err)
	assert.Contains(t, out2, "capture")
}

func TestRunWithTimeout(t *testing.T) {
	// Should succeed quickly
	assert.NoError(t, RunWithTimeout(2*time.Second, "echo", "fast"))

	// Should timeout
	err := RunWithTimeout(50*time.Millisecond, "sleep", "1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestQuiet(t *testing.T) {
	err := Quiet("true")
	assert.NoError(t, err)

	err = Quiet("false")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exited with code")
}

func TestEchoTo(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "echo-test-*")
	assert.NoError(t, err)
	defer f.Close()

	err = EchoTo("cat", "abc123", ">", f.Name()) // this won't actually redirect in subshell
	assert.Error(t, err)                         // cat > file isn't parsed by exec.Command
}

func TestAppendAndReplaceLine(t *testing.T) {
	tmp := t.TempDir() + "/file.txt"

	// Append new line
	assert.NoError(t, AppendIfMissing(tmp, "line1"))
	data, _ := os.ReadFile(tmp)
	assert.Contains(t, string(data), "line1")

	// Append again (no duplicates)
	assert.NoError(t, AppendIfMissing(tmp, "line1"))
	data2, _ := os.ReadFile(tmp)
	assert.Equal(t, string(data), string(data2))

	// Replace existing line
	assert.NoError(t, ReplaceLineContains(tmp, "line1", "replaced"))
	data3, _ := os.ReadFile(tmp)
	assert.Contains(t, string(data3), "replaced")

	// Replace nonexistent substring
	err := ReplaceLineContains(tmp, "nope", "x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "needle")
}

func TestWrapCmdError(t *testing.T) {
	// Should wrap with exit code
	err := Run("false")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exited with code")
}

func TestIsVerboseEnv(t *testing.T) {
	_ = os.Setenv("LDAPPY_DEBUG", "1")
	assert.True(t, isVerbose())
	_ = os.Unsetenv("LDAPPY_DEBUG")
	assert.False(t, isVerbose())
}
