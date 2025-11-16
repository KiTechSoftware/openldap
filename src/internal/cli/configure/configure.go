package configure

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/kitechsoftware/ldappy/internal/common/security"
	"github.com/kitechsoftware/ldappy/internal/common/shell"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"
	"github.com/kitechsoftware/ldappy/internal/common/ui/text"
)

var (
	BackupDir   = "/var/backups/ldappy"
	LdapDataDir = "/var/lib/ldap"
)

// ---------- Common Report Struct ----------

type ConfigReport struct {
	Action     string    `json:"action"`
	Success    bool      `json:"success"`
	Message    string    `json:"message,omitempty"`
	ErrorMsg   string    `json:"error,omitempty"`
	BackupFile string    `json:"backup_file,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Duration   string    `json:"duration,omitempty"`
}

// ---------- Report Printer ----------

func PrintReport(report *ConfigReport, jsonOutput bool) {
	if jsonOutput {
		log.JSON(report)
		return
	}

	if report.Duration == "" {
		report.Duration = "unknown"
	}

	switch {
	case report.Success:
		log.Success("%s — %s (%s)", text.Title(report.Action), report.Message, report.Duration)
	case report.ErrorMsg != "":
		log.Error("%s — %s (%s)", text.Title(report.Action), report.ErrorMsg, report.Duration)
	default:
		log.Warn("%s — %s (%s)", text.Title(report.Action), report.Message, report.Duration)
	}
}

// ---------- LDIF Apply ----------

func ApplyLDIF(ctx context.Context, ldifFile string, interactive, jsonOutput bool) *ConfigReport {
	start := time.Now()
	report := &ConfigReport{Action: "apply_ldif", Timestamp: start}

	if interactive {
		log.Info("🧩 Starting interactive configuration: dpkg-reconfigure slapd...")
		if err := shell.RunContext(ctx, "dpkg-reconfigure", "slapd"); err != nil {
			report.ErrorMsg = fmt.Sprintf("interactive configuration failed: %v", err)
		} else {
			report.Success = true
			report.Message = "Interactive configuration completed successfully"
		}
		report.Duration = time.Since(start).String()
		PrintReport(report, jsonOutput)
		return report
	}

	if ldifFile == "" {
		report.ErrorMsg = "provide an LDIF file path or use --interactive mode"
		PrintReport(report, jsonOutput)
		return report
	}

	log.Info("Applying LDIF configuration patch: %s", ldifFile)
	if err := shell.RunContext(ctx, "ldapmodify", "-Y", "EXTERNAL", "-H", "ldapi:///", "-f", ldifFile); err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to apply LDIF: %v", err)
	} else {
		report.Success = true
		report.Message = "LDIF applied successfully"
	}
	report.Duration = time.Since(start).String()
	PrintReport(report, jsonOutput)
	return report
}

// ---------- Password Hashing ----------

func HashPassword(ctx context.Context, plaintext string) (string, error) {
	if plaintext == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	hashed, err := security.HashPasswordContext(ctx, plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	log.Success("🔐 Generated LDAP password hash: %s", hashed)
	return hashed, nil
}

// ---------- Admin Password Reset ----------

func ResetAdminPassword(ctx context.Context, newPlainPassword string, jsonOutput bool) *ConfigReport {
	start := time.Now()
	report := &ConfigReport{Action: "reset_admin_password", Timestamp: start}

	if newPlainPassword == "" {
		report.ErrorMsg = "new password cannot be empty"
		PrintReport(report, jsonOutput)
		return report
	}

	hashed, err := HashPassword(ctx, newPlainPassword)
	if err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to generate password hash: %v", err)
		PrintReport(report, jsonOutput)
		return report
	}

	const ldifTemplate = `dn: olcDatabase={{.DBIndex}},cn=config
changetype: modify
replace: olcRootPW
olcRootPW: {{.Hashed}}
`
	data := struct {
		DBIndex string
		Hashed  string
	}{
		DBIndex: "{1}mdb",
		Hashed:  hashed,
	}

	tmpFile, err := os.CreateTemp("", "reset-admin-*.ldif")
	if err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to create temp LDIF: %v", err)
		PrintReport(report, jsonOutput)
		return report
	}
	defer os.Remove(tmpFile.Name())

	tpl, err := template.New("reset").Parse(ldifTemplate)
	if err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to parse LDIF template: %v", err)
		PrintReport(report, jsonOutput)
		return report
	}

	if err := tpl.Execute(tmpFile, data); err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to render LDIF: %v", err)
		PrintReport(report, jsonOutput)
		return report
	}
	tmpFile.Close()

	log.Info("🔧 Applying new admin password via ldapmodify...")
	if err := shell.RunContext(ctx, "ldapmodify", "-Y", "EXTERNAL", "-H", "ldapi:///", "-f", tmpFile.Name()); err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to apply new password: %v", err)
	} else {
		report.Success = true
		report.Message = "Admin password reset successfully. Restart slapd to apply."
	}
	report.Duration = time.Since(start).String()
	PrintReport(report, jsonOutput)
	return report
}

// ---------- Backup ----------

func Backup(ctx context.Context, jsonOutput bool) *ConfigReport {
	start := time.Now()
	report := &ConfigReport{Action: "backup", Timestamp: start}

	if err := os.MkdirAll(BackupDir, 0755); err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to create backup directory: %v", err)
		PrintReport(report, jsonOutput)
		return report
	}

	timestamp := time.Now().Format("20060102-150405")
	backupFile := filepath.Join(BackupDir, fmt.Sprintf("ldap-backup-%s.ldif", timestamp))

	log.Info("📦 Creating LDAP backup at %s", backupFile)

	if err := shell.RunContext(ctx, "slapcat", "-l", backupFile); err != nil {
		report.ErrorMsg = fmt.Sprintf("backup failed: %v", err)
	} else {
		report.Success = true
		report.Message = "Backup completed successfully"
		report.BackupFile = backupFile
	}
	report.Duration = time.Since(start).String()
	PrintReport(report, jsonOutput)
	return report
}

// ---------- Rollback ----------
func Rollback(ctx context.Context, backupFile string, jsonOutput bool) *ConfigReport {
	start := time.Now()
	report := &ConfigReport{Action: "rollback", BackupFile: backupFile, Timestamp: start}

	if backupFile == "" {
		report.ErrorMsg = "backup file path required"
		PrintReport(report, jsonOutput)
		return report
	}

	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		report.ErrorMsg = fmt.Sprintf("backup file not found: %s", backupFile)
		PrintReport(report, jsonOutput)
		return report
	}

	log.Warn("Creating pre-rollback backup for safety...")
	preBackup := Backup(ctx, jsonOutput)
	if !preBackup.Success {
		report.ErrorMsg = fmt.Sprintf("failed to create pre-rollback backup: %s", preBackup.ErrorMsg)
		PrintReport(report, jsonOutput)
		return report
	}

	log.Warn("⚠ Rolling back LDAP data from backup: %s", backupFile)
	log.Info("Stopping slapd service...")

	if err := shell.RunWithTimeout(15*time.Second, "systemctl", "stop", "slapd"); err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to stop slapd: %v", err)
		PrintReport(report, jsonOutput)
		return report
	}

	if err := shell.RunContext(ctx, "slapadd", "-l", backupFile); err != nil {
		report.ErrorMsg = fmt.Sprintf("rollback failed: %v", err)
		PrintReport(report, jsonOutput)
		return report
	}

	_ = shell.RunContext(ctx, "chown", "-R", "openldap:openldap", LdapDataDir)
	_ = shell.RunWithTimeout(10*time.Second, "systemctl", "start", "slapd")

	report.Success = true
	report.Message = "Rollback completed and slapd restarted"
	report.Duration = time.Since(start).String()
	PrintReport(report, jsonOutput)
	return report
}

// func Init(cfg *config.Config, jsonOutput bool) *LifecycleReport {
// 	report := &LifecycleReport{Action: "init"}

// 	color.Cyan("Checking for base DN: %s", cfg.LDAP.BaseDN)
// 	check := exec.Command("ldapsearch", "-x", "-LLL", "-b", cfg.LDAP.BaseDN, "dn")
// 	if err := check.Run(); err == nil {
// 		report.Success = true
// 		report.Message = fmt.Sprintf("Base DN %s already exists", cfg.LDAP.BaseDN)
// 		if !jsonOutput {
// 			color.Green("✔ %s", report.Message)
// 		}
// 		return report
// 	}

// 	color.Yellow("Base DN not found — creating initial directory structure...")

// 	adminDN := fmt.Sprintf("cn=%s,%s", cfg.LDAP.AdminUser, cfg.LDAP.BaseDN)
// 	firstPart := strings.Split(cfg.LDAP.BaseDN, ",")[0]
// 	dc := strings.TrimPrefix(firstPart, "dc=")

// 	// Use hashed password if enabled
// 	hashedPassword := maybeHashPassword(cfg)

// 	baseLDIF := fmt.Sprintf(`dn: %[1]s
// objectClass: top
// objectClass: dcObject
// objectClass: organization
// o: %[2]s
// dc: %[3]s

// dn: cn=%[5]s,%[1]s
// objectClass: simpleSecurityObject
// objectClass: organizationalRole
// cn: %[5]s
// description: Directory Manager
// userPassword: %[4]s
// `, cfg.LDAP.BaseDN, cfg.LDAP.Organization, dc, hashedPassword, cfg.LDAP.AdminUser)

// 	cmd := exec.Command("ldapadd", "-x", "-D", adminDN, "-w", cfg.LDAP.AdminPassword)
// 	stdin, _ := cmd.StdinPipe()
// 	_, _ = stdin.Write([]byte(baseLDIF))
// 	_ = stdin.Close()
// 	cmd.Stdout = color.Output
// 	cmd.Stderr = color.Output

// 	if err := cmd.Run(); err != nil {
// 		report.Success = false
// 		report.ErrorMsg = fmt.Sprintf("failed to create base DN: %v", err)
// 		return report
// 	}

// 	report.Success = true
// 	report.Message = fmt.Sprintf("Base DN %s initialized successfully", cfg.LDAP.BaseDN)
// 	if !jsonOutput {
// 		color.Green("✔ %s", report.Message)
// 	}
// 	return report
// }
