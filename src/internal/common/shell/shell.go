package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
)

// isVerbose checks whether LDAPPY_DEBUG is enabled dynamically.
func isVerbose() bool {
	return os.Getenv("LDAPPY_DEBUG") != ""
}

// Run executes a command and streams output live to stdout/stderr.
func Run(name string, args ...string) error {
	return RunContext(context.Background(), name, args...)
}

// RunContext executes a command with a provided context.
func RunContext(ctx context.Context, name string, args ...string) error {
	if isVerbose() {
		color.Cyan("> %s %s", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = color.Output
	cmd.Stderr = color.Output

	if err := cmd.Run(); err != nil {
		return wrapCmdError(name, args, err)
	}
	return nil
}

// RunWithTimeout executes a command with a maximum duration.
func RunWithTimeout(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if isVerbose() {
		color.Cyan("> %s %s (timeout: %v)", name, strings.Join(args, " "), timeout)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = color.Output
	cmd.Stderr = color.Output

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("command timed out after %v: %s %s", timeout, name, strings.Join(args, " "))
		}
		return wrapCmdError(name, args, err)
	}
	return nil
}

// Command runs a command and returns its combined stdout/stderr output.
func Command(name string, args ...string) (string, error) {
	return CommandContext(context.Background(), name, args...)
}

// CommandContext runs a command with context and returns combined output.
func CommandContext(ctx context.Context, name string, args ...string) (string, error) {
	if isVerbose() {
		color.Cyan("> %s %s", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), wrapCmdError(name, args, err)
	}
	return string(out), nil
}

// CommandExists returns true if a command exists in PATH.
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// RunCapture runs a command, streams output live, and also returns the captured output.
func RunCapture(ctx context.Context, name string, args ...string) (string, error) {
	if isVerbose() {
		color.Cyan("> %s %s", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(ctx, name, args...)
	var buf strings.Builder
	mw := io.MultiWriter(color.Output, &buf)
	cmd.Stdout = mw
	cmd.Stderr = mw

	err := cmd.Run()
	return buf.String(), wrapCmdError(name, args, err)
}

// Quiet executes a command silently without streaming output.
func Quiet(name string, args ...string) error {
	if isVerbose() {
		color.Cyan("> %s %s (quiet)", name, strings.Join(args, " "))
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return wrapCmdError(name, args, cmd.Run())
}

// ---------- File Utilities & Echo Helpers ----------

// EchoTo runs a command and writes content to its STDIN (used for ldapmodify, etc.)
func EchoTo(name string, content string, args ...string) error {
	if isVerbose() {
		color.Cyan("> echo | %s %s", name, strings.Join(args, " "))
	}

	cmd := exec.Command(name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = color.Output
	cmd.Stderr = color.Output

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return err
	}

	if _, err := io.WriteString(stdin, content); err != nil {
		_ = stdin.Close()
		return err
	}
	_ = stdin.Close()

	return wrapCmdError(name, args, cmd.Wait())
}

// EchoToContext runs a command with context and writes content to STDIN.
func EchoToContext(ctx context.Context, name string, content string, args ...string) error {
	if isVerbose() {
		color.Cyan("> echo | %s %s", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(ctx, name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = color.Output
	cmd.Stderr = color.Output

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return err
	}

	if _, err := io.WriteString(stdin, content); err != nil {
		_ = stdin.Close()
		return err
	}
	_ = stdin.Close()

	return wrapCmdError(name, args, cmd.Wait())
}

// AppendIfMissing appends a line to a file only if it does not already exist.
func AppendIfMissing(path, line string) error {
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), line) {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

// ReplaceLineContains replaces the first line containing a substring with a new line.
func ReplaceLineContains(path, needle, replacement string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for i, l := range lines {
		if strings.Contains(l, needle) {
			lines[i] = replacement
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("needle %q not found in %s", needle, path)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// ---------- Internal Helpers ----------

// wrapCmdError adds context and exit code info if available.
func wrapCmdError(name string, args []string, err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("%s %v exited with code %d: %w", name, args, exitErr.ExitCode(), err)
	}
	return fmt.Errorf("%s %v failed: %w", name, args, err)
}
