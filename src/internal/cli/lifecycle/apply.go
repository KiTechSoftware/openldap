package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/kitechsoftware/ldappy/internal/common/config"
	"github.com/kitechsoftware/ldappy/internal/common/shell"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"
)

// Apply performs the full lifecycle setup flow: Install → Init → optional TLS → password hashing → PAM integration.
func Apply(ctx context.Context, cfg *config.Config, isContainer, jsonOutput bool) []*LifecycleReport {
	var reports []*LifecycleReport

	start := time.Now()
	log.Info("=== Beginning LDAP Lifecycle Apply ===")

	// --- Step 1: Install ---
	log.Info("=== Step 1: Install OpenLDAP (slapd) ===")
	installReport := Install(ctx, cfg, isContainer, jsonOutput)
	reports = append(reports, installReport)
	if !installReport.Success {
		log.Error("Installation failed, aborting apply sequence")
		return reports
	}

	// --- Step 2: Initialize Directory Structure ---
	log.Info("=== Step 2: Initialize BaseDN ===")
	initReport := Init(ctx, cfg, jsonOutput, false)
	reports = append(reports, initReport)
	if !initReport.Success {
		log.Error("Initialization failed, aborting apply sequence")
		return reports
	}

	// --- Step 3: Optional TLS Configuration ---
	if cfg.Modules.TLS {
		log.Info("=== Step 3: Configure TLS ===")
		tlsReport := configureTLS(ctx, cfg)
		reports = append(reports, tlsReport)
		if !tlsReport.Success {
			log.Error("TLS configuration failed, aborting apply sequence")
			return reports
		}
	}

	// --- Step 4: Optional Password Hashing Policy ---
	if cfg.Modules.PasswordHashing {
		log.Info("=== Step 4: Set Password Hash Scheme ===")
		hashReport := setPasswordHash(ctx, "{SSHA}")
		reports = append(reports, hashReport)
		if !hashReport.Success {
			log.Error("Password hashing policy configuration failed, aborting apply sequence")
			return reports
		}
	}

	// --- Step 5: Optional PAM/NSS Integration ---
	if cfg.Modules.PAMIntegration {
		log.Info("=== Step 5: Configure PAM/NSS Integration ===")
		pamReport := setupPAM(ctx, cfg)
		reports = append(reports, pamReport)
		if !pamReport.Success {
			log.Warn("PAM integration failed (non-critical), continuing...")
		}
	}

	log.Success("Lifecycle apply completed in %s", time.Since(start).Round(time.Second))
	return reports
}

// ---------- Substeps ----------

func configureTLS(ctx context.Context, cfg *config.Config) *LifecycleReport {
	start := time.Now()
	report := &LifecycleReport{Action: "configure_tls"}

	certFile := cfg.TLS.CertFile
	keyFile := cfg.TLS.KeyFile
	if certFile == "" {
		certFile = "/etc/ssl/certs/ldap.crt"
	}
	if keyFile == "" {
		keyFile = "/etc/ssl/private/ldap.key"
	}

	if cfg.TLS.Method == "letsencrypt" {
		log.Info("Using Let's Encrypt for certificate provisioning")
		if err := shell.RunContext(ctx, "apt", "install", "-y", "certbot"); err != nil {
			report.ErrorMsg = fmt.Sprintf("failed to install certbot: %v", err)
			report.SetDuration(start)
			return report
		}
		if err := shell.RunContext(ctx, "certbot", "certonly", "--standalone", "-d", cfg.TLS.Domain); err != nil {
			report.ErrorMsg = fmt.Sprintf("certbot failed: %v", err)
			report.SetDuration(start)
			return report
		}
		certFile = fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", cfg.TLS.Domain)
		keyFile = fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", cfg.TLS.Domain)
	} else {
		log.Info("Generating self-signed TLS certificate for %s", cfg.TLS.Domain)
		days := "365"
		if cfg.TLS.CertDays > 0 {
			days = fmt.Sprintf("%d", cfg.TLS.CertDays)
		}
		err := shell.RunContext(ctx, "openssl", "req", "-x509", "-newkey", "rsa:4096",
			"-days", days,
			"-keyout", keyFile,
			"-out", certFile,
			"-nodes",
			"-subj", "/CN="+cfg.TLS.Domain)
		if err != nil {
			report.ErrorMsg = fmt.Sprintf("failed to create self-signed certificate: %v", err)
			report.SetDuration(start)
			return report
		}
	}

	ldif := fmt.Sprintf(`dn: cn=config
add: olcTLSCertificateFile
olcTLSCertificateFile: %s
-
add: olcTLSCertificateKeyFile
olcTLSCertificateKeyFile: %s
`, certFile, keyFile)

	log.Info("Applying TLS configuration via ldapmodify...")
	if err := shell.EchoToContext(ctx, "ldapmodify", ldif, "-Y", "EXTERNAL", "-H", "ldapi:///"); err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to apply TLS config: %v", err)
		report.SetDuration(start)
		return report
	}

	_ = shell.RunContext(ctx, "systemctl", "restart", "slapd")

	report.Success = true
	report.Message = "TLS configured and slapd restarted"
	report.SetDuration(start)
	return report
}

func setPasswordHash(ctx context.Context, scheme string) *LifecycleReport {
	start := time.Now()
	report := &LifecycleReport{Action: "set_password_hash"}

	ldif := fmt.Sprintf(`dn: olcDatabase={1}mdb,cn=config
replace: olcPasswordHash
olcPasswordHash: %s
`, scheme)

	log.Info("Applying password hash scheme: %s", scheme)
	if err := shell.EchoToContext(ctx, "ldapmodify", ldif, "-Y", "EXTERNAL", "-H", "ldapi:///"); err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to set password hash: %v", err)
		report.SetDuration(start)
		return report
	}

	report.Success = true
	report.Message = fmt.Sprintf("Password hashing scheme set to %s", scheme)
	report.SetDuration(start)
	return report
}

func setupPAM(ctx context.Context, cfg *config.Config) *LifecycleReport {
	start := time.Now()
	report := &LifecycleReport{Action: "setup_pam"}

	log.Info("Installing PAM/NSS integration packages...")
	if err := shell.RunContext(ctx, "apt", "install", "-y", "libnss-ldapd", "libpam-ldapd", "nslcd"); err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to install PAM packages: %v", err)
		report.SetDuration(start)
		return report
	}

	log.Info("Configuring LDAP and NSS integration files...")
	_ = shell.AppendIfMissing("/etc/ldap/ldap.conf", "BASE "+cfg.LDAP.BaseDN)
	_ = shell.AppendIfMissing("/etc/ldap/ldap.conf", "URI ldap://localhost")
	_ = shell.ReplaceLineContains("/etc/nsswitch.conf", "passwd:", "passwd:         files systemd ldap")
	_ = shell.ReplaceLineContains("/etc/nsswitch.conf", "group:", "group:          files systemd ldap")
	_ = shell.ReplaceLineContains("/etc/nsswitch.conf", "shadow:", "shadow:         files ldap")

	_ = shell.AppendIfMissing("/etc/pam.d/common-auth", "auth    sufficient      pam_ldap.so")
	_ = shell.AppendIfMissing("/etc/pam.d/common-account", "account sufficient    pam_ldap.so")
	_ = shell.AppendIfMissing("/etc/pam.d/common-password", "password sufficient   pam_ldap.so")
	_ = shell.AppendIfMissing("/etc/pam.d/common-session", "session sufficient    pam_ldap.so")

	if err := shell.RunContext(ctx, "systemctl", "restart", "nslcd"); err != nil {
		report.ErrorMsg = fmt.Sprintf("failed to restart nslcd: %v", err)
		report.SetDuration(start)
		return report
	}

	report.Success = true
	report.Message = "PAM/NSS integration configured successfully"
	report.SetDuration(start)
	return report
}
