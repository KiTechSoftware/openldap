package lifecycle

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kitechsoftware/ldappy/internal/common/config"
	"github.com/kitechsoftware/ldappy/internal/common/security"
	"github.com/kitechsoftware/ldappy/internal/common/shell"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"
	"github.com/kitechsoftware/ldappy/internal/common/ui/text"
)

// ---------- LifecycleReport ----------

type LifecycleReport struct {
	Action   string `json:"action"`
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	ErrorMsg string `json:"error,omitempty"`
	Duration string `json:"duration,omitempty"`
}

// ---------- Purge / Install / Upgrade ----------

func Purge(ctx context.Context, jsonOutput bool) *LifecycleReport {
	start := time.Now()
	report := startReport("purge")
	log.WarnCtx(ctx, "Purging any existing OpenLDAP installation...")

	failed := false
	var lastErr error

	steps := [][]string{
		{"systemctl", "stop", "slapd"},
		{"bash", "-c", "DEBIAN_FRONTEND=noninteractive apt purge -y slapd ldap-utils"},
		{"bash", "-c", "apt autoremove -y"},
		{"bash", "-c", "rm -rf /etc/ldap /var/lib/ldap /var/backups/ldappy/*"},
	}

	for _, cmdArgs := range steps {
		if err := shell.RunContext(ctx, cmdArgs[0], cmdArgs[1:]...); err != nil {
			failed = true
			lastErr = err
			log.WarnCtx(ctx, "Step failed (continuing): %v", err)
		}
	}

	_ = shell.RunContext(ctx, "bash", "-c", `echo "PURGE" | debconf-communicate slapd || true`)

	if failed {
		report.SetDuration(start)
		return report.
			Fail("some purge steps failed; system may not be clean", lastErr)
	}

	report.SuccessMsg("system cleaned and ready for fresh installation")
	report.SetDuration(start)
	return report
}

// ---------- Install ----------
func Install(ctx context.Context, cfg *config.Config, isContainer, jsonOutput bool) *LifecycleReport {
	start := time.Now()
	report := startReport("install")
	log.InfoCtx(ctx, "Installing OpenLDAP (slapd)...")

	configDir := "/etc/ldap/slapd.d"
	existedBefore := dirExists(configDir)

	if err := runStep(ctx, "apt update", "bash", "-c", "DEBIAN_FRONTEND=noninteractive apt update -y"); err != nil {
		return report.Fail("apt update failed", err)
	}

	pkgs := "slapd ldap-utils ca-certificates openssl procps"
	if isContainer {
		pkgs += " --no-install-recommends"
	}
	if err := runStep(ctx, "apt install", "bash", "-c",
		fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt install -y %s", pkgs)); err != nil {
		return report.Fail("apt install failed", err)
	}

	if !isContainer {
		log.InfoCtx(ctx, "Running initial OpenLDAP configuration (non-container environment)...")
		// If the directory didn’t exist before install, Debian just created a default config.
		// Force a proper re-init to seed ldappy's configuration.
		if !existedBefore {
			log.WarnCtx(ctx, "Detected fresh slapd installation — forcing initialization.")
		} else {
			log.InfoCtx(ctx, "Existing OpenLDAP config detected — preserving configuration.")
		}

		confReport := Init(ctx, cfg, jsonOutput, !existedBefore)
		if !confReport.Success {
			return confReport
		}

		if err := runStep(ctx, "enable/start slapd", "systemctl", "enable", "--now", "slapd"); err != nil {
			report.SetDuration(start)
			return report.Fail("failed to enable/start slapd", err)
		}
	} else {
		log.WarnCtx(ctx, "Skipping configuration — running inside container.")
	}

	runStep(ctx, "enable/start slapd", "systemctl", "enable", "--now", "slapd")

	if err := configurePasswordHashing(ctx, report); err != nil {
		log.WarnCtx(ctx, "Password hashing configuration failed: %v", err)
	}

	report.SuccessMsg("OpenLDAP installed successfully")
	report.SetDuration(start)
	return report
}

func Upgrade(ctx context.Context, jsonOutput bool) *LifecycleReport {
	start := time.Now()
	report := startReport("upgrade")
	log.InfoCtx(ctx, "Upgrading OpenLDAP packages...")

	if err := runStep(ctx, "apt update", "apt", "update"); err != nil {
		report.SetDuration(start)
		return report.Fail("apt update failed", err)
	}
	if err := runStep(ctx, "upgrade slapd/ldap-utils", "apt", "-y", "--only-upgrade", "slapd", "ldap-utils"); err != nil {
		report.SetDuration(start)
		return report.Fail("upgrade failed", err)
	}

	_ = shell.RunContext(ctx, "systemctl", "restart", "slapd")
	report.SuccessMsg("Upgrade complete and slapd restarted")
	report.SetDuration(start)
	return report
}

// ---------- Initialization ----------

func Init(ctx context.Context, cfg *config.Config, jsonOutput bool, force bool) *LifecycleReport {
	start := time.Now()
	report := startReport("init")
	configDir := "/etc/ldap/slapd.d"

	if !force {
		if _, err := os.Stat(configDir); err == nil {
			log.SuccessCtx(ctx, "OpenLDAP already initialized — skipping setup (%s exists)", configDir)
			report.SuccessMsg("OpenLDAP already configured, skipping initialization")
			report.SetDuration(start)
			return report
		}
	}

	log.InfoCtx(ctx, "Initializing OpenLDAP configuration...")

	// 1. Generate admin password if not provided
	if cfg.LDAP.AdminPassword == "" {
		p, err := security.GeneratePassword(16)
		if err != nil {
			return report.Fail("failed to generate admin password", err)
		}
		cfg.LDAP.AdminPassword = p
		_ = cfg.Save()
		log.WarnCtx(ctx, "Generated random LDAP admin password: %s", cfg.LDAP.AdminPassword)
	}

	// 2. Hash admin password (SHA512 → SSHA fallback)
	hashedPassword := cfg.LDAP.AdminPassword
	if cfg.Modules.PasswordHashing {
		h, err := security.HashPasswordContext(ctx, cfg.LDAP.AdminPassword)
		if err != nil {
			return report.Fail("failed to hash admin password", err)
		}
		hashedPassword = h
		log.SuccessCtx(ctx, "Admin password hashed with OpenLDAP-compatible scheme")
	}

	// 3. Run non-interactive dpkg reconfigure
	seed := fmt.Sprintf(`slapd slapd/internal/adminpw password %[1]s
slapd slapd/internal/generated_adminpw password %[1]s
slapd slapd/password1 password %[1]s
slapd slapd/password2 password %[1]s
slapd slapd/domain string %[2]s
slapd shared/organization string %[3]s
slapd slapd/backend select MDB
slapd slapd/no_configuration boolean false
`, cfg.LDAP.AdminPassword, cfg.LDAP.Domain, cfg.LDAP.Organization)

	_ = shell.EchoToContext(ctx, "bash", seed, "-c", "debconf-set-selections")
	if err := runStep(ctx, "dpkg-reconfigure slapd",
		"bash", "-c", "DEBIAN_FRONTEND=noninteractive dpkg-reconfigure slapd"); err != nil {
		return report.Fail("slapd reconfiguration failed", err)
	}

	// 4. Inject admin hash into both config & data DB
	setRootLDIF := fmt.Sprintf(`
dn: olcDatabase={1}mdb,cn=config
changetype: modify
replace: olcRootPW
olcRootPW: %s
`, hashedPassword)

	if err := shell.EchoToContext(ctx, "bash", setRootLDIF, "-c",
		"ldapmodify -Y EXTERNAL -H ldapi:/// >/dev/null 2>&1"); err != nil {
		log.WarnCtx(ctx, "Failed to set olcRootPW in MDB DB: %v", err)
	}

	// 5. Add base DN if missing
	baseLDIF := fmt.Sprintf(`
dn: %s
objectClass: top
objectClass: dcObject
objectClass: organization
o: %s
dc: %s
`, cfg.LDAP.BaseDN, cfg.LDAP.Organization, strings.Split(cfg.LDAP.Domain, ".")[0])

	shell.EchoToContext(ctx, "bash", baseLDIF, "-c",
		"ldapadd -Y EXTERNAL -H ldapi:/// >/dev/null 2>&1 || true")

	// 6. Add admin entry under base DN
	adminLDIF := fmt.Sprintf(`
dn: cn=%s,%s
objectClass: simpleSecurityObject
objectClass: organizationalRole
cn: %s
description: Directory Administrator
userPassword: %s
`, cfg.LDAP.AdminUser, cfg.LDAP.BaseDN, cfg.LDAP.AdminUser, hashedPassword)

	shell.EchoToContext(ctx, "bash", adminLDIF, "-c",
		"ldapadd -Y EXTERNAL -H ldapi:/// >/dev/null 2>&1 || true")

	// restart slapd to apply changes
	serviceReport := Service(ctx, "restart", false, false, false)
	if !serviceReport.Success {
		log.WarnCtx(ctx, "Failed to restart slapd using init system - trying again manually")
		serviceReport = Service(ctx, "restart", false, true, false)
	}
	if !serviceReport.Success {
		return serviceReport
	}

	// 7. Verify admin credentials actually work
	verifyCmd := fmt.Sprintf(
		"ldapwhoami -x -D 'cn=%s,%s' -w '%s' >/dev/null 2>&1",
		cfg.LDAP.AdminUser, cfg.LDAP.BaseDN, cfg.LDAP.AdminPassword,
	)
	if err := shell.RunContext(ctx, "bash", "-c", verifyCmd); err != nil {
		return report.Fail("admin bind verification failed", fmt.Errorf("ldapwhoami failed with provided credentials"))
	}

	log.SuccessCtx(ctx, "Verified admin bind for cn=%s,%s", cfg.LDAP.AdminUser, cfg.LDAP.BaseDN)
	cfg.Save()
	report.SuccessMsg("OpenLDAP initialized successfully")
	report.SetDuration(start)
	return report
}

// ---------- Reporting Helpers ----------

func startReport(action string) *LifecycleReport {
	return &LifecycleReport{
		Action: action,
	}
}

func (r *LifecycleReport) SuccessMsg(msg string) *LifecycleReport {
	r.Success = true
	r.Message = msg
	return r
}

func (r *LifecycleReport) Fail(msg string, err error) *LifecycleReport {
	r.Success = false
	r.Message = msg
	if err != nil {
		r.ErrorMsg = err.Error()
	}
	return r
}

func (r *LifecycleReport) Finish(jsonOutput bool) *LifecycleReport {
	if r.Duration == "" {
		r.Duration = "unknown"
	}
	if jsonOutput {
		log.JSON(r)
		return r
	}
	d := fmt.Sprintf(" (%s)", r.Duration)
	switch {
	case r.Success:
		log.Success("%s — %s%s", text.Title(r.Action), r.Message, d)
	case r.ErrorMsg != "":
		log.Error("%s — %s%s", text.Title(r.Action), r.ErrorMsg, d)
	default:
		log.Warn("%s — %s%s", text.Title(r.Action), r.Message, d)
	}
	return r
}

func (r *LifecycleReport) SetDuration(start time.Time) {
	r.Duration = time.Since(start).Round(time.Millisecond).String()
}

// ---------- Utilities ----------

func runStep(ctx context.Context, step string, cmd string, args ...string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	log.DebugCtx(ctx, "Running step: %s", step)
	return shell.RunContext(ctx, cmd, args...)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func configurePasswordHashing(ctx context.Context, report *LifecycleReport) error {
	log.InfoCtx(ctx, "Configuring OpenLDAP password hashing (SHA512 preferred)...")

	// Try to load the pw-sha2 module (safe if already loaded)
	loadLDIF := `
dn: cn=module{0},cn=config
changetype: modify
add: olcModuleLoad
olcModuleLoad: pw-sha2.la
`
	_ = shell.EchoToContext(ctx, "bash", loadLDIF, "-c",
		"ldapmodify -Q -Y EXTERNAL -H ldapi:/// >/dev/null 2>&1 || true")

	// Decide on the hashing scheme
	scheme := "{SHA512}"
	if _, err := os.Stat("/usr/lib/ldap/pw-sha2.so"); err != nil {
		log.WarnCtx(ctx, "pw-sha2 not found — falling back to {SSHA}")
		scheme = "{SSHA}"
	}

	setLDIF := fmt.Sprintf(`
dn: cn=config
changetype: modify
replace: olcPasswordHash
olcPasswordHash: %s
`, scheme)

	if err := shell.EchoToContext(ctx, "bash", setLDIF, "-c",
		"ldapmodify -Q -Y EXTERNAL -H ldapi:///"); err != nil {
		return fmt.Errorf("failed to set olcPasswordHash=%s: %w", scheme, err)
	}

	log.SuccessCtx(ctx, "Configured olcPasswordHash=%s", scheme)
	report.SuccessMsg(fmt.Sprintf("Configured password hashing: %s", scheme))
	return nil
}
