package status

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// ---------- Data Types ----------

type StatusReport struct {
	Service struct {
		Active bool   `json:"active"`
		Since  string `json:"since,omitempty"`
	} `json:"service"`
	Ports struct {
		LDAP  bool `json:"ldap"`
		LDAPS bool `json:"ldaps"`
	} `json:"ports"`
	BaseDN        string  `json:"base_dn,omitempty"`
	TLS           TLSInfo `json:"tls"`
	LastBackup    string  `json:"last_backup,omitempty"`
	ConfigVersion string  `json:"config_version"`
}

type TLSInfo struct {
	Exists        bool   `json:"exists"`
	ExpiresInDays int    `json:"expires_in_days,omitempty"`
	ExpiryDate    string `json:"expiry_date,omitempty"`
}

type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
	Timestamp string `json:"timestamp"`
}

// Populated at build time via -ldflags
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// ---------- Public API ----------

// Collect gathers all status information into a structured report.
// It respects context cancellation during subprocess execution.
func Collect(ctx context.Context) (StatusReport, error) {
	var r StatusReport
	r.ConfigVersion = Version

	if ctx.Err() != nil {
		return r, ctx.Err()
	}

	if err := checkService(ctx, &r); err != nil && ctx.Err() == nil {
		// non-fatal, continue
	}

	if err := checkPorts(ctx, &r); err != nil && ctx.Err() == nil {
	}

	if ctx.Err() != nil {
		return r, ctx.Err()
	}

	r.BaseDN = getBaseDN(ctx)
	r.TLS = getTLSInfo(ctx)
	r.LastBackup = getLastBackup("/var/backups/ldappy")

	return r, nil
}

// ---------- Individual Checkers ----------

func checkService(ctx context.Context, r *StatusReport) error {
	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "slapd")
		if err := cmd.Run(); err == nil {
			r.Service.Active = true
			out, _ := exec.CommandContext(ctx, "systemctl", "show", "-p", "ActiveEnterTimestamp", "slapd").CombinedOutput()
			r.Service.Since = strings.TrimSpace(strings.TrimPrefix(string(out), "ActiveEnterTimestamp="))
			return nil
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// fallback: use process lookup
	if err := exec.CommandContext(ctx, "pgrep", "slapd").Run(); err == nil {
		r.Service.Active = true
		r.Service.Since = "(running - unmanaged)"
	}

	return nil
}

func checkPorts(ctx context.Context, r *StatusReport) error {
	if _, err := exec.LookPath("ss"); err == nil {
		out, err := exec.CommandContext(ctx, "ss", "-tlnp").CombinedOutput()
		if err == nil {
			r.Ports.LDAP = regexp.MustCompile(`:389\b`).Match(out)
			r.Ports.LDAPS = regexp.MustCompile(`:636\b`).Match(out)
			return nil
		}
	}

	// fallback: try TCP dial
	r.Ports.LDAP = tcpOpen(ctx, "127.0.0.1:389")
	r.Ports.LDAPS = tcpOpen(ctx, "127.0.0.1:636")
	return nil
}

func tcpOpen(ctx context.Context, addr string) bool {
	d := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

func getBaseDN(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "ldapsearch", "-x", "-s", "base", "-b", "", "namingContexts").Output()
	if err != nil {
		return ""
	}
	var bases []string
	for _, line := range strings.Split(string(out), "\n") {
		if ctx.Err() != nil {
			return ""
		}
		if strings.HasPrefix(line, "namingContexts:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				bases = append(bases, strings.TrimSpace(parts[1]))
			}
		}
	}
	return strings.Join(bases, ", ")
}

func getTLSInfo(ctx context.Context) TLSInfo {
	info := TLSInfo{}
	certPath := "/etc/ssl/certs/ldap.crt"
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return info
	}
	info.Exists = true

	out, err := exec.CommandContext(ctx, "openssl", "x509", "-enddate", "-noout", "-in", certPath).Output()
	if err != nil {
		return info
	}

	select {
	case <-ctx.Done():
		return info
	default:
	}

	dateStr := strings.TrimSpace(strings.TrimPrefix(string(out), "notAfter="))
	layouts := []string{
		"Jan 2 15:04:05 2006 MST",
		"Jan  2 15:04:05 2006 MST",
		"Jan 2 15:04:05 2006 GMT",
	}
	for _, layout := range layouts {
		if exp, err := time.Parse(layout, dateStr); err == nil {
			info.ExpiresInDays = int(time.Until(exp).Hours() / 24)
			info.ExpiryDate = exp.Format(time.RFC3339)
			break
		}
	}

	return info
}

func getLastBackup(dir string) string {
	files, err := os.ReadDir(dir)
	if err != nil || len(files) == 0 {
		return ""
	}

	sort.Slice(files, func(i, j int) bool {
		fi, _ := files[i].Info()
		fj, _ := files[j].Info()
		return fi.ModTime().After(fj.ModTime())
	})
	return filepath.Join(dir, files[0].Name())
}

// ---------- Build Info ----------

func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		Timestamp: time.Now().Format(time.RFC3339),
	}
}
