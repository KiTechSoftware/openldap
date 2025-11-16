package log

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

var (
	debugEnabled bool
	verbose      bool
	jsonOutput   bool

	once       sync.Once
	logDir     string
	logFile    string
	maxLogSize = 10 * 1024 * 1024 // 10 MB rotation threshold
	mu         sync.Mutex
)

// ---------- Context Keys ----------

type ctxKey string

const (
	requestIDKey ctxKey = "request-id"
)

// WithRequestID attaches a request ID to the context (for log correlation).
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFrom extracts a request ID from context (if any).
func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// ---------- Verbosity / Mode Control ----------

// EnableDebug enables debug mode (used for verbose internal logs).
func EnableDebug() { debugEnabled = true }

// DisableDebug disables debug mode.
func DisableDebug() { debugEnabled = false }

// EnableVerbose activates verbose logging (alias for EnableDebug).
func EnableVerbose() {
	debugEnabled = true
	verbose = true
}

// DisableVerbose deactivates verbose logging.
func DisableVerbose() {
	debugEnabled = false
	verbose = false
}

// IsDebug returns true if verbose or debug mode is enabled.
func IsDebug() bool {
	initVerbosity()
	return debugEnabled || verbose
}

// SetJSONOutput toggles JSON log output (used for CLI --json flag).
func SetJSONOutput(v bool) { jsonOutput = v }

// JSONOutputEnabled reports whether JSON logging is enabled.
func JSONOutputEnabled() bool { return jsonOutput }

// ---------- Initialization ----------

func initVerbosity() {
	once.Do(func() {
		if os.Getenv("LDAPPY_DEBUG") != "" {
			debugEnabled = true
		}

		if d := os.Getenv("LDAPPY_LOG_DIR"); d != "" {
			logDir = d
		} else if canUse("/var/log/ldappy") {
			logDir = "/var/log/ldappy"
		} else {
			home, _ := os.UserHomeDir()
			logDir = filepath.Join(home, ".ldappy", "logs")
		}

		logFile = filepath.Join(logDir, "ldappy.log")
	})
}

func canUse(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	f := filepath.Join(dir, ".perm-test")
	if err := os.WriteFile(f, []byte("ok"), 0o644); err != nil {
		return false
	}
	_ = os.Remove(f)
	return true
}

// ---------- Public Logging Functions ----------

// --- Standard (non-context) ---
func Info(format string, a ...any) {
	logOut("[INFO]", fmt.Sprintf(format, a...), color.BlueString("ℹ️  "+format, a...))
}
func Success(format string, a ...any) {
	logOut("[SUCCESS]", fmt.Sprintf(format, a...), color.GreenString("✅ "+format, a...))
}
func Warn(format string, a ...any) {
	logOut("[WARN]", fmt.Sprintf(format, a...), color.YellowString("⚠️  "+format, a...))
}
func Error(format string, a ...any) {
	logOut("[ERROR]", fmt.Sprintf(format, a...), color.RedString("❌ "+format, a...))
}
func Debug(format string, a ...any) {
	initVerbosity()
	if !IsDebug() {
		return
	}
	logOut("[DEBUG]", fmt.Sprintf(format, a...), color.HiBlackString("🪶 "+format, a...))
}
func Verbosef(format string, a ...any) { Debug(format, a...) }

// --- Context-aware variants ---
func InfoCtx(ctx context.Context, format string, a ...any) {
	if ctx.Err() == nil {
		logOutCtx(ctx, "[INFO]", fmt.Sprintf(format, a...), color.BlueString("ℹ️  "+format, a...))
	}
}
func SuccessCtx(ctx context.Context, format string, a ...any) {
	if ctx.Err() == nil {
		logOutCtx(ctx, "[SUCCESS]", fmt.Sprintf(format, a...), color.GreenString("✅ "+format, a...))
	}
}
func WarnCtx(ctx context.Context, format string, a ...any) {
	if ctx.Err() == nil {
		logOutCtx(ctx, "[WARN]", fmt.Sprintf(format, a...), color.YellowString("⚠️  "+format, a...))
	}
}
func ErrorCtx(ctx context.Context, format string, a ...any) {
	if ctx.Err() == nil {
		logOutCtx(ctx, "[ERROR]", fmt.Sprintf(format, a...), color.RedString("❌ "+format, a...))
	}
}
func DebugCtx(ctx context.Context, format string, a ...any) {
	if ctx.Err() != nil {
		return
	}
	initVerbosity()
	if !IsDebug() {
		return
	}
	logOutCtx(ctx, "[DEBUG]", fmt.Sprintf(format, a...), color.HiBlackString("🪶 "+format, a...))
}

// ---------- Structured Sections ----------

func Section(title string) {
	if jsonOutput {
		return
	}
	color.Cyan("\n────────── %s ──────────", title)
}

func Subsection(title string) {
	if jsonOutput {
		return
	}
	color.HiCyan("\n[%s]", title)
}

func SectionEnd() {
	if jsonOutput {
		return
	}
	color.Cyan("──────────────────────────────\n")
}

// ---------- JSON + Timed Helpers ----------

func JSON(v any) {
	data, _ := json.Marshal(v)
	fmt.Println(string(data))
	writeLog("JSON", string(data))
}

func Timed(action string, start time.Time, success bool, err error) {
	dur := time.Since(start).Round(time.Millisecond)
	switch {
	case err != nil:
		Error("%s failed after %v: %v", action, dur, err)
	case success:
		Success("%s completed successfully in %v", action, dur)
	default:
		Warn("%s finished in %v with no explicit result", action, dur)
	}
}

// ---------- Core Logging ----------

func logOut(level, message, colored string) {
	initVerbosity()
	if jsonOutput {
		entry := map[string]any{
			"time":  time.Now().Format(time.RFC3339),
			"level": level,
			"msg":   sanitize(message),
		}
		_ = json.NewEncoder(os.Stdout).Encode(entry)
	} else {
		fmt.Fprintln(os.Stdout, colored)
	}
	writeLog(level, message)
}

func logOutCtx(ctx context.Context, level, message, colored string) {
	initVerbosity()
	reqID := RequestIDFrom(ctx)

	if jsonOutput {
		entry := map[string]any{
			"time":  time.Now().Format(time.RFC3339),
			"level": level,
			"msg":   sanitize(message),
		}
		if reqID != "" {
			entry["request_id"] = reqID
		}
		_ = json.NewEncoder(os.Stdout).Encode(entry)
	} else {
		if reqID != "" {
			colored = fmt.Sprintf("[%s] %s", reqID, colored)
		}
		fmt.Fprintln(os.Stdout, colored)
	}

	writeLogWithReq(level, message, reqID)
}

func writeLog(level, message string) { writeLogWithReq(level, message, "") }

func writeLogWithReq(level, message, reqID string) {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		if IsDebug() {
			color.Yellow("⚠️  Cannot create log dir %s: %v", logDir, err)
		}
		return
	}

	if err := rotateIfNeeded(); err != nil && IsDebug() {
		color.Yellow("⚠️  Log rotation failed: %v", err)
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		if IsDebug() {
			color.Yellow("⚠️  Cannot open log file %s: %v", logFile, err)
		}
		return
	}
	defer f.Close()

	ts := time.Now().Format(time.RFC3339)

	if jsonOutput {
		entry := map[string]any{
			"time":  ts,
			"level": level,
			"msg":   sanitize(message),
		}
		if reqID != "" {
			entry["request_id"] = reqID
		}
		data, _ := json.Marshal(entry)
		_, _ = f.Write(append(data, '\n'))
		return
	}

	if reqID != "" {
		_, _ = f.WriteString(fmt.Sprintf("%s | %-7s | %s | %s\n", ts, level, reqID, sanitize(message)))
	} else {
		_, _ = f.WriteString(fmt.Sprintf("%s | %-7s | %s\n", ts, level, sanitize(message)))
	}
}

func rotateIfNeeded() error {
	info, err := os.Stat(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() < int64(maxLogSize) {
		return nil
	}

	timestamp := time.Now().Format("20060102-150405")
	backup := filepath.Join(logDir, fmt.Sprintf("ldappy-%s.log", timestamp))
	if err := os.Rename(logFile, backup); err != nil {
		return fmt.Errorf("failed to rotate log file: %w", err)
	}
	if IsDebug() {
		color.HiBlack("🌀 Rotated log to %s", backup)
	}
	return nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if len(s) > 4096 {
		s = s[:4096] + "…[truncated]"
	}
	return s
}
