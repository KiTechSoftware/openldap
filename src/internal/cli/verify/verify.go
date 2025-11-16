package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/kitechsoftware/ldappy/internal/common/config"
	"github.com/kitechsoftware/ldappy/internal/common/shell"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"
)

// VerifyReport holds the results of all verification checks.
type VerifyReport struct {
	SlapdActive   CheckResult  `json:"slapd_active"`
	BaseDN        CheckResult  `json:"base_dn"`
	TLS           *CheckResult `json:"tls,omitempty"`
	AllSuccessful bool         `json:"all_successful"`
}

// CheckResult represents the outcome of a single check.
type CheckResult struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// RunAll performs a full verification and returns a structured report.
// It never aborts early — all checks are performed even if one fails.
func RunAll(ctx context.Context, cfg *config.Config) *VerifyReport {
	report := &VerifyReport{}

	log.InfoCtx(ctx, "🔍 Running LDAP verification checks...")

	var wg sync.WaitGroup

	// Run slapd and BaseDN checks concurrently.
	wg.Add(2)
	go func() {
		defer wg.Done()
		report.SlapdActive = runCheck(ctx, "slapd service", func() error { return LdapIsActive(ctx) }, "slapd service is active")
	}()
	go func() {
		defer wg.Done()
		report.BaseDN = runCheck(ctx, fmt.Sprintf("BaseDN %s", cfg.LDAP.BaseDN),
			func() error { return BaseDNReachable(ctx, cfg.LDAP.BaseDN) },
			fmt.Sprintf("BaseDN reachable: %s", cfg.LDAP.BaseDN))
	}()

	wg.Wait()

	// TLS check if enabled
	if cfg.Modules.TLS {
		res := runCheck(ctx, "TLS on port 636", func() error { return TlsEnabled(ctx) }, "✔ TLS verified on port 636")
		report.TLS = &res
	}

	report.AllSuccessful = report.SlapdActive.Success &&
		report.BaseDN.Success &&
		(report.TLS == nil || report.TLS.Success)

	return report
}

// runCheck executes a check function, prints its outcome, and returns the result.
func runCheck(ctx context.Context, name string, fn func() error, successMsg string) CheckResult {
	log.DebugCtx(ctx, "> Verifying %s...", name)

	err := fn()
	if err != nil {
		log.ErrorCtx(ctx, "✖ %s failed: %v", name, err)
		return CheckResult{
			Name:    name,
			Success: false,
			Message: fmt.Sprintf("%s failed", name),
			Error:   err.Error(),
		}
	}

	log.DebugCtx(ctx, "✔ %s passed", name)
	return CheckResult{
		Name:    name,
		Success: true,
		Message: successMsg,
	}
}

// ---------- Individual Checks ----------

// LdapIsActive verifies that the slapd service is running.
// Works with both systemd and systemd-less environments.
func LdapIsActive(ctx context.Context) error {
	log.DebugCtx(ctx, "> Checking if slapd service is active...")

	if _, err := exec.LookPath("systemctl"); err == nil {
		if err := shell.Run("systemctl", "is-active", "--quiet", "slapd"); err == nil {
			return nil
		}
	}

	// fallback: check process existence
	if err := shell.Run("pgrep", "slapd"); err != nil {
		return fmt.Errorf("slapd process not found")
	}
	return nil
}

// BaseDNReachable verifies that ldapsearch can query the BaseDN.
func BaseDNReachable(ctx context.Context, baseDN string) error {
	log.DebugCtx(ctx, "> Checking BaseDN reachability: %s", baseDN)
	_, err := shell.Command("ldapsearch", "-x", "-LLL", "-b", baseDN, "dn")
	return err
}

// TlsEnabled verifies that LDAPS is accepting connections and presents a cert.
func TlsEnabled(ctx context.Context) error {
	log.DebugCtx(ctx, "> Verifying TLS handshake on localhost:636...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "openssl", "s_client",
		"-connect", "localhost:636",
		"-servername", "localhost",
	)
	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("TLS check timed out after 5s")
	}
	if err != nil {
		return fmt.Errorf("openssl handshake failed: %v", err)
	}
	if !strings.Contains(string(out), "subject=") {
		return fmt.Errorf("no valid certificate presented on port 636")
	}
	return nil
}

// ---------- Summary Printer ----------

// PrintSummary prints a human-readable or JSON summary of the verification results.
func PrintSummary(report *VerifyReport, jsonOutput bool) {
	if jsonOutput {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			color.Red("Failed to encode JSON report: %v", err)
			return
		}
		fmt.Println(string(data))
		return
	}

	fmt.Println()
	color.Yellow("🧾 Verification Summary:")
	printCheck(report.SlapdActive)
	printCheck(report.BaseDN)
	if report.TLS != nil {
		printCheck(*report.TLS)
	}

	fmt.Println()
	if report.AllSuccessful {
		color.Green("✅ All LDAP checks passed successfully!\n")
	} else {
		color.Red("❌ Some LDAP checks failed.\n")
	}
}

func printCheck(c CheckResult) {
	if c.Success {
		color.Green("  ✔ %s", c.Message)
	} else {
		color.Red("  ✖ %s — %s", c.Name, c.Error)
	}
}
