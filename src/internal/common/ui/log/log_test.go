package log

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// captureOutput intercepts stdout during a function call.
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestDebugToggles(t *testing.T) {
	debugEnabled = false
	verbose = false

	assert.False(t, IsDebug(), "expected IsDebug() false initially")

	EnableDebug()
	assert.True(t, IsDebug(), "EnableDebug() should enable debug")

	DisableDebug()
	assert.False(t, IsDebug(), "DisableDebug() should disable debug")

	EnableVerbose()
	assert.True(t, IsDebug(), "EnableVerbose() should enable debug")
	assert.True(t, verbose, "verbose flag should be true")

	DisableVerbose()
	assert.False(t, IsDebug(), "DisableVerbose() should disable debug and verbose")
	assert.False(t, verbose, "verbose flag should be false")
}

func TestJSONOutputToggle(t *testing.T) {
	jsonOutput = false
	assert.False(t, JSONOutputEnabled(), "should start disabled")

	SetJSONOutput(true)
	assert.True(t, JSONOutputEnabled(), "should enable JSON output")

	SetJSONOutput(false)
	assert.False(t, JSONOutputEnabled(), "should disable JSON output")
}

func TestSanitize(t *testing.T) {
	input := "hello\nworld"
	expected := "hello world"
	assert.Equal(t, expected, sanitize(input))

	long := strings.Repeat("x", 5000)
	out := sanitize(long)
	assert.True(t, len(out) < len(long), "should truncate long input")
	assert.True(t, strings.HasSuffix(out, "…[truncated]"), "should end with truncation marker")
}

func TestTimedFunction_Success(t *testing.T) {
	start := time.Now()
	assert.NotPanics(t, func() {
		Timed("unit-test", start, true, nil)
	})
}

func TestSectionHelpers(t *testing.T) {
	assert.NotPanics(t, func() {
		Section("Setup")
		Subsection("Database")
		SectionEnd()
	})
}

// ---------- Context-Aware Tests ----------

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")

	id := RequestIDFrom(ctx)
	assert.Equal(t, "req-123", id, "should extract same request ID")

	ctx2 := context.Background()
	assert.Empty(t, RequestIDFrom(ctx2), "empty context should return empty string")
}

func TestInfoCtx_RespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	output := captureOutput(func() {
		InfoCtx(ctx, "should not print after cancel")
	})

	assert.Empty(t, strings.TrimSpace(output), "no log output expected after cancellation")
}

func TestInfoCtx_PrintsBeforeCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	output := captureOutput(func() {
		InfoCtx(ctx, "hello %s", "world")
	})
	cancel()

	assert.Contains(t, output, "hello world", "should print before cancellation")
}

func TestRequestID_InOutput(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "RID-777")

	output := captureOutput(func() {
		InfoCtx(ctx, "testing request id inclusion")
	})

	assert.Contains(t, output, "testing request id inclusion", "message should appear")
	assert.Contains(t, output, "RID-777", "request ID should be visible in output")
}

func TestDebugCtx_OnlyPrintsWhenEnabled(t *testing.T) {
	DisableDebug()
	output1 := captureOutput(func() {
		DebugCtx(context.Background(), "invisible debug")
	})
	assert.Empty(t, strings.TrimSpace(output1), "should not print in non-debug mode")

	EnableDebug()
	output2 := captureOutput(func() {
		DebugCtx(context.Background(), "visible debug message")
	})
	assert.Contains(t, output2, "visible debug message", "should print in debug mode")
}

// ---------- JSON Output Tests (Structured Validation) ----------

func parseJSONOutput(t *testing.T, output string) map[string]any {
	t.Helper()
	output = strings.TrimSpace(output)
	if output == "" {
		t.Fatalf("no output captured for JSON parse")
	}
	var data map[string]any
	err := json.Unmarshal([]byte(output), &data)
	assert.NoError(t, err, "output should be valid JSON")
	return data
}

func TestJSONOutput_Mode_EmitsJSON(t *testing.T) {
	SetJSONOutput(true)
	defer SetJSONOutput(false)

	output := captureOutput(func() {
		Info("json-mode log message")
	})

	data := parseJSONOutput(t, output)
	assert.Equal(t, "INFO", data["level"])
	assert.Equal(t, "json-mode log message", data["msg"])
	assert.NotEmpty(t, data["time"], "timestamp should be present")
}

func TestJSONOutput_Mode_WarnCtx(t *testing.T) {
	SetJSONOutput(true)
	defer SetJSONOutput(false)

	ctx := WithRequestID(context.Background(), "REQ-22")

	output := captureOutput(func() {
		WarnCtx(ctx, "disk nearly full: %d%% used", 95)
	})

	data := parseJSONOutput(t, output)
	assert.Equal(t, "WARN", data["level"])
	assert.Contains(t, data["msg"].(string), "95%", "should include formatted message")
	assert.NotEmpty(t, data["time"])
	assert.Equal(t, "REQ-22", data["request_id"])
}

func TestJSONOutput_Mode_ErrorCtx(t *testing.T) {
	SetJSONOutput(true)
	defer SetJSONOutput(false)

	ctx := WithRequestID(context.Background(), "REQ-ERR")

	output := captureOutput(func() {
		ErrorCtx(ctx, "failed to connect: %s", "timeout")
	})

	data := parseJSONOutput(t, output)
	assert.Equal(t, "ERROR", data["level"])
	assert.Contains(t, data["msg"].(string), "timeout")
	assert.Equal(t, "REQ-ERR", data["request_id"])
	assert.NotEmpty(t, data["time"])
}

func TestJSONOutput_Mode_SuccessCtx(t *testing.T) {
	SetJSONOutput(true)
	defer SetJSONOutput(false)

	ctx := context.Background()

	output := captureOutput(func() {
		SuccessCtx(ctx, "operation complete in %dms", 123)
	})

	data := parseJSONOutput(t, output)
	assert.Equal(t, "SUCCESS", data["level"])
	assert.Contains(t, data["msg"].(string), "123ms")
	assert.NotEmpty(t, data["time"])
}
