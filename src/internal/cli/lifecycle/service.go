package lifecycle

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kitechsoftware/ldappy/internal/cli/verify"
	"github.com/kitechsoftware/ldappy/internal/common/shell"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"
)

// ---------- Public Entrypoint ----------

func Service(ctx context.Context, action string, jsonOutput bool, isContainer bool, daemon bool) *LifecycleReport {
	start := time.Now()
	report := startReport("service:" + action)
	log.InfoCtx(ctx, "Handling service action: %s (isContainer=%v)", action, isContainer)

	if daemon {
		handleDaemonMode(ctx, action, report)
		report.SetDuration(start)
		return report
	}

	if isContainer {
		handleContainerMode(ctx, action, report)
		report.SetDuration(start)
		return report
	}

	handleSystemMode(ctx, action, report)

	report.SetDuration(start)
	return report
}

// ---------- Daemon Mode ----------

func handleDaemonMode(ctx context.Context, action string, report *LifecycleReport) {
	switch action {
	case "start":
		daemonStart(ctx, report)
	case "stop":
		containerStop(ctx, report)
	case "restart":
		daemonRestart(ctx, report)
	default:
		report.Fail(fmt.Sprintf("unsupported action: %s", action), nil)
	}
}

func daemonStart(ctx context.Context, report *LifecycleReport) {
	manualStart(ctx, report, true)
}

func daemonRestart(ctx context.Context, report *LifecycleReport) {
	containerStop(ctx, report)
	time.Sleep(1 * time.Second)
	containerStart(ctx, report)
	report.SuccessMsg("slapd restarted successfully (container mode)")
}

// ---------- Container Mode ----------

func handleContainerMode(ctx context.Context, action string, report *LifecycleReport) {
	switch action {
	case "status":
		containerStatus(ctx, report)
	case "start":
		containerStart(ctx, report)
	case "stop":
		containerStop(ctx, report)
	case "restart":
		containerRestart(ctx, report)
	default:
		report.Fail(fmt.Sprintf("unsupported action: %s", action), nil)
	}
}

func containerStatus(ctx context.Context, report *LifecycleReport) {
	if err := verify.LdapIsActive(ctx); err != nil {
		report.Fail("slapd not running", err)
	} else {
		report.SuccessMsg("slapd is active")
	}
}

func containerStart(ctx context.Context, report *LifecycleReport) {
	manualStart(ctx, report, false)
}

func manualStart(ctx context.Context, report *LifecycleReport, daemon bool) {
	if verify.LdapIsActive(ctx) == nil {
		log.InfoCtx(ctx, "slapd already running")
		report.SuccessMsg("slapd already running")
		return
	}
	env := loadLdapEnv()

	if err := shell.RunContext(ctx, "mkdir", "-p", "/run/slapd"); err != nil {
		report.Fail("failed to create /run/slapd directory", err)
		return
	}

	if err := shell.RunContext(ctx, "chown", fmt.Sprintf("%s:%s", env.User, env.Group), "/run/slapd"); err != nil {
		report.Fail("failed to change ownership of /run/slapd", err)
		return
	}

	log.InfoCtx(ctx, "Starting slapd manually (container mode)...")

	// Validate TLS setup if enabled
	if env.EnableTLS {
		if !fileExists(env.CertFile) || !fileExists(env.KeyFile) {
			msg := fmt.Sprintf("TLS is enabled but certificate or key file not found (cert=%s, key=%s)",
				env.CertFile, env.KeyFile)
			log.ErrorCtx(ctx, "%s", msg)
			report.Fail(msg, nil)
			return
		}
		log.InfoCtx(ctx, "TLS enabled: cert=%s key=%s", env.CertFile, env.KeyFile)
	}

	// Build listener URLs dynamically
	listeners := fmt.Sprintf("ldap://0.0.0.0:%s ldapi:///", env.Port)
	if env.EnableTLS {
		listeners = fmt.Sprintf("%s ldaps://0.0.0.0:%s", listeners, env.LdapsPort)
	}

	var args []string
	args = append(args, "-h", listeners)
	if env.User != "" {
		args = append(args, "-u", env.User)
	}
	if env.Group != "" {
		args = append(args, "-g", env.Group)
	}
	// if env.DataDir != "" {
	// 	args = append(args, "-F", env.ConfigDir)
	// }
	// if env.LogDir != "" {
	// 	args = append(args, "-l", env.LogDir)
	// }
	if daemon {
		args = append(args, "-d", "stats")
	}

	if err := shell.RunContext(ctx, "/usr/sbin/slapd", args...); err != nil {
		report.Fail("failed to start slapd manually", err)
	} else {
		report.SuccessMsg("slapd started successfully (container mode)")
	}
}

func containerStop(ctx context.Context, report *LifecycleReport) {
	log.InfoCtx(ctx, "Stopping slapd manually (container mode)...")
	pidFile := "/run/slapd/slapd.pid"
	if fileExists(pidFile) {
		if err := shell.RunContext(ctx, "bash", "-c", "kill -TERM $(cat "+pidFile+")"); err == nil {
			report.SuccessMsg("slapd stopped successfully (pidfile)")
			return
		}
	}
	if err := shell.RunContext(ctx, "pkill", "slapd"); err != nil {
		report.Fail("failed to stop slapd manually", err)
	} else {
		report.SuccessMsg("slapd stopped successfully (container mode)")
	}
}

func containerRestart(ctx context.Context, report *LifecycleReport) {
	containerStop(ctx, report)
	time.Sleep(1 * time.Second)
	containerStart(ctx, report)
	report.SuccessMsg("slapd restarted successfully (container mode)")
}

// ---------- System Mode ----------

func handleSystemMode(ctx context.Context, action string, report *LifecycleReport) {
	systemctlAvailable := shell.CommandExists("systemctl") && fileExists("/run/systemd/system")

	if !systemctlAvailable {
		log.WarnCtx(ctx, "Systemd not available; falling back to manual container mode")
		handleContainerMode(ctx, action, report)
		return
	}

	switch action {
	case "status":
		if err := verify.LdapIsActive(ctx); err != nil {
			report.Fail("slapd not running", err)
		} else {
			report.SuccessMsg("slapd is active")
		}
	default:
		runSystemctl(ctx, action, report)
	}
}

func runSystemctl(ctx context.Context, action string, report *LifecycleReport) {
	log.DebugCtx(ctx, "Using systemctl for service action")
	if err := shell.RunContext(ctx, "systemctl", action, "slapd"); err != nil {
		report.Fail(fmt.Sprintf("Failed to run systemctl %s slapd", action), err)
	} else {
		report.SuccessMsg(fmt.Sprintf("slapd %sed successfully (systemd)", action))
	}
}

// ---------- Environment Configuration ----------

type ldapEnv struct {
	DataDir    string
	ConfigDir  string
	LogDir     string
	LdappyDir  string
	User       string
	Group      string
	Port       string
	LdapsPort  string
	DebugLevel string
	CertDir    string
	CertFile   string
	KeyFile    string
	EnableTLS  bool
}

func loadLdapEnv() ldapEnv {
	env := ldapEnv{
		DataDir:    getEnv("LDAP_DATA_DIR", "/var/lib/ldap"),
		ConfigDir:  getEnv("LDAP_CONFIG_DIR", "/etc/ldap/slapd.d"),
		LogDir:     getEnv("LDAP_LOG_DIR", "/var/log/ldap"),
		LdappyDir:  getEnv("LDAPPY_DIR", "/etc/ldappy"),
		User:       getEnv("LDAP_USER", "openldap"),
		Group:      getEnv("LDAP_GROUP", "openldap"),
		Port:       getEnv("LDAP_PORT", "389"),
		LdapsPort:  getEnv("LDAPS_PORT", "636"),
		DebugLevel: getEnv("LDAP_DEBUG_LEVEL", "0"),
		CertDir:    getEnv("LDAP_CERT_DIR", "/etc/ssl/ldap"),
	}
	env.CertFile = getEnv("LDAP_CERT_FILE", env.CertDir+"/server.crt")
	env.KeyFile = getEnv("LDAP_KEY_FILE", env.CertDir+"/server.key")
	env.EnableTLS = isEnvTrue("LDAP_ENABLE_TLS")
	return env
}

// ---------- Utility Functions ----------

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func isEnvTrue(key string) bool {
	val := os.Getenv(key)
	return val == "1" || val == "true" || val == "TRUE" || val == "yes" || val == "on"
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
