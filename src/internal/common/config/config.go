package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/kitechsoftware/ldappy/internal/common/ui/log"
	"github.com/pelletier/go-toml/v2"
)

var (
	configDirOnce   sync.Once
	cachedConfigDir string
)

// ---------- Types ----------

type Config struct {
	LDAP    LDAP    `toml:"ldap" json:"ldap"`
	Modules Modules `toml:"modules" json:"modules"`
	TLS     TLS     `toml:"tls" json:"tls"`
	API     API     `toml:"api" json:"api"`
}

type API struct {
	Enabled bool   `toml:"enabled" json:"enabled"`
	Host    string `toml:"host" json:"host"`
	Port    int    `toml:"port" json:"port"`
}

type LDAP struct {
	Domain        string `toml:"domain" json:"domain"`
	Organization  string `toml:"organization" json:"organization"`
	BaseDN        string `toml:"base_dn" json:"base_dn"`
	AdminUser     string `toml:"admin_user" json:"admin_user"`
	AdminPassword string `toml:"admin_password" json:"admin_password"`
}

type Modules struct {
	TLS             bool `toml:"tls" json:"tls"`
	PasswordHashing bool `toml:"password_hashing" json:"password_hashing"`
	ImportLDIF      bool `toml:"import_ldif" json:"import_ldif"`
	PAMIntegration  bool `toml:"pam_integration" json:"pam_integration"`
}

type TLS struct {
	Method   string `toml:"method" json:"method"`
	CertDays int    `toml:"cert_days" json:"cert_days"`
	Domain   string `toml:"domain" json:"domain"`
	CertFile string `toml:"cert_file,omitempty" json:"cert_file,omitempty"`
	KeyFile  string `toml:"key_file,omitempty" json:"key_file,omitempty"`
}

func (a *API) Address() string {
	return fmt.Sprintf("%s:%d", a.Host, a.Port)
}

// ---------- Existence / Load / Save ----------

// Exist returns true if the config file exists.
func Exist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// LoadCtx loads the configuration file while respecting context cancellation.
func LoadCtx(ctx context.Context, path string) (*Config, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return Load(path)
}

// Load loads and validates a TOML configuration file.
func Load(path string) (*Config, error) {
	cfg := &Config{}

	if !Exist(path) {
		return nil, fmt.Errorf("config not found at %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	// Decode into a generic map first to detect unknown keys
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid TOML syntax in %s: %w", path, err)
	}

	// Now decode into typed struct
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config structure: %w", err)
	}

	// Warn for unknown top-level keys
	for key := range raw {
		if key != "ldap" && key != "modules" && key != "tls" && key != "api" {
			log.Warn("Unrecognized key in config: %s", key)
		}
	}

	cfg.fillDefaults()
	cfg.envOverride()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	log.Success("Loaded configuration from %s", path)
	return cfg, nil
}

// SaveCtx saves a configuration while respecting cancellation.
func SaveCtx(ctx context.Context, path string, c *Config) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return Save(path, c)
}

// Save writes the configuration atomically.
func Save(path string, c *Config) error {
	tmp := path + ".tmp"

	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	return os.Rename(tmp, path)
}

// ---------- Helpers ----------

func (c *Config) Save() error {
	path := ConfigPath()
	tmp := path + ".tmp"

	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	return os.Rename(tmp, path)
}

// fillDefaults populates sensible defaults for missing values.
func (c *Config) fillDefaults() {
	if c.LDAP.Domain == "" {
		c.LDAP.Domain = "example.com"
	}
	if c.LDAP.Organization == "" {
		c.LDAP.Organization = "ExampleOrg"
	}
	if c.LDAP.BaseDN == "" && c.LDAP.Domain != "" {
		c.LDAP.BaseDN = domainToBaseDN(c.LDAP.Domain)
	}
	if c.Modules.TLS {
		if c.TLS.Method == "" {
			c.TLS.Method = "openssl"
		}
		if c.TLS.CertDays == 0 {
			c.TLS.CertDays = 365
		}
		if c.TLS.Domain == "" {
			c.TLS.Domain = c.LDAP.Domain
		}
	}

	if c.LDAP.AdminUser == "" {
		c.LDAP.AdminUser = "admin"
	}

	if c.API.Host == "" {
		c.API.Host = "localhost"
	}
	if c.API.Port == 0 {
		c.API.Port = 8080
	}
}

// envOverride applies overrides from environment variables.
func (c *Config) envOverride() {
	get := os.Getenv

	if v := get("LDAP_ROOT_USER"); v != "" {
		c.LDAP.AdminUser = v
	}
	if v := get("LDAP_ROOT_PASSWORD"); v != "" {
		c.LDAP.AdminPassword = v
	}
	if v := get("LDAP_ORGANIZATION"); v != "" {
		c.LDAP.Organization = v
	}

	domainChanged := false
	if v := get("LDAP_DOMAIN"); v != "" && v != c.LDAP.Domain {
		c.LDAP.Domain = v
		domainChanged = true
	}

	if v := get("LDAP_BASE_DN"); v != "" {
		c.LDAP.BaseDN = v
	} else if domainChanged {
		c.LDAP.BaseDN = domainToBaseDN(c.LDAP.Domain)
	}

	if v := get("LDAP_TLS"); v != "" {
		c.Modules.TLS = parseBool(v)
	}
	if v := get("LDAP_PASSWORD_HASHING"); v != "" {
		c.Modules.PasswordHashing = parseBool(v)
	}
	if v := get("LDAP_IMPORT_LDIF"); v != "" {
		c.Modules.ImportLDIF = parseBool(v)
	}
	if v := get("LDAP_PAM_INTEGRATION"); v != "" {
		c.Modules.PAMIntegration = parseBool(v)
	}

	if v := get("LDAP_TLS_METHOD"); v != "" {
		c.TLS.Method = v
	}
	if v := get("LDAP_TLS_CERT_DAYS"); v != "" {
		if days, err := strconv.Atoi(v); err == nil {
			c.TLS.CertDays = days
		}
	}
	if v := get("LDAP_TLS_DOMAIN"); v != "" {
		c.TLS.Domain = v
	}
	if v := get("LDAP_TLS_CERT_FILE"); v != "" {
		c.TLS.CertFile = v
	}
	if v := get("LDAP_TLS_KEY_FILE"); v != "" {
		c.TLS.KeyFile = v
	}
}

// domainToBaseDN converts "example.com" → "dc=example,dc=com"
func domainToBaseDN(domain string) string {
	parts := strings.Split(domain, ".")
	dcs := make([]string, len(parts))
	for i, p := range parts {
		dcs[i] = fmt.Sprintf("dc=%s", p)
	}
	return strings.Join(dcs, ",")
}

// parseBool accepts flexible truthy values.
func parseBool(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// Validate checks required fields and consistency.
func (c *Config) Validate() error {
	if c.LDAP.Domain == "" {
		return fmt.Errorf("missing ldap.domain")
	}
	if c.LDAP.Organization == "" {
		return fmt.Errorf("missing ldap.organization")
	}
	if c.LDAP.BaseDN == "" {
		return fmt.Errorf("missing ldap.base_dn")
	}
	if c.Modules.TLS {
		if c.TLS.Domain == "" {
			return fmt.Errorf("missing tls.domain")
		}
		if c.TLS.Method != "openssl" && c.TLS.Method != "letsencrypt" {
			return fmt.Errorf("invalid tls.method: %s", c.TLS.Method)
		}
	}
	return nil
}

// Summary prints a human-readable configuration overview.
func (c Config) Summary() string {
	return fmt.Sprintf(
		"%s (%s) — BaseDN: %s\nModules: TLS=%t, Hashing=%t, LDIF=%t, PAM=%t",
		c.LDAP.Domain,
		c.LDAP.Organization,
		c.LDAP.BaseDN,
		c.Modules.TLS,
		c.Modules.PasswordHashing,
		c.Modules.ImportLDIF,
		c.Modules.PAMIntegration,
	)
}

// SummaryJSON returns a machine-readable summary for JSON mode.
func (c Config) SummaryJSON() string {
	data, _ := json.MarshalIndent(c, "", "  ")
	return string(data)
}

func (c *Config) LDAPURL() string {
	return ldapUrl("localhost", c.Modules.TLS)
}

// Default returns a valid default configuration.
func Default() Config {
	return Config{
		LDAP: LDAP{
			Domain:        "example.com",
			Organization:  "Example Org",
			BaseDN:        "dc=example,dc=com",
			AdminUser:     "admin",
			AdminPassword: "",
		},
		Modules: Modules{
			TLS:             false,
			PasswordHashing: true,
			ImportLDIF:      false,
			PAMIntegration:  false,
		},
		TLS: TLS{
			Method:   "openssl",
			CertDays: 365,
			Domain:   "example.com",
			CertFile: "",
			KeyFile:  "",
		},
		API: API{
			Enabled: false,
			Host:    "localhost",
			Port:    8080,
		},
	}
}

// ConfigPath returns the standard ldappy config file path.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.toml")
}

// ConfigDir returns the base configuration directory for ldappy.
// It prefers /etc/ldappy for system-wide installs if writable.
// Directory creation is concurrency-safe.
func ConfigDir() string {
	configDirOnce.Do(func() {
		cachedConfigDir = resolveConfigDir()
		log.Debug("Using configuration directory: %s", cachedConfigDir)
	})
	return cachedConfigDir
}

func resolveConfigDir() string {
	// 1. Explicit override
	if dir := os.Getenv("LDAP_DATA_DIR"); dir != "" {
		if ensureDirWritableSafe(dir) == nil {
			return dir
		}
	}

	// 2. System-wide preferred location
	systemDir := "/etc/ldappy"
	if ensureDirWritableSafe(systemDir) == nil {
		return systemDir
	}

	// 3. User-specific config dirs
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		userDir := filepath.Join(dir, "ldappy")
		if ensureDirWritableSafe(userDir) == nil {
			return userDir
		}
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		userDir := filepath.Join(home, ".config", "ldappy")
		if ensureDirWritableSafe(userDir) == nil {
			return userDir
		}
	}

	// 4. Last resort for minimal containers
	fallback := "/tmp/ldappy"
	_ = ensureDirWritableSafe(fallback)
	return fallback
}

// ensureDirWritableSafe atomically ensures a directory exists and is writable.
// It tolerates concurrent creations (no race conditions).
func ensureDirWritableSafe(dir string) error {
	// Try to stat first
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists but is not a directory", dir)
		}
		// Directory exists; test write access
		testFile := filepath.Join(dir, ".writetest")
		if err := os.WriteFile(testFile, []byte{}, 0o600); err == nil {
			_ = os.Remove(testFile)
			return nil
		}
		return fmt.Errorf("no write permission for %s", dir)
	}

	if !os.IsNotExist(err) {
		return err // stat error other than ENOENT
	}

	// Attempt atomic directory creation.
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		// Retry once — another process may have created it simultaneously.
		if st, stErr := os.Stat(dir); stErr == nil && st.IsDir() {
			return nil
		}
		return fmt.Errorf("failed to create directory %s: %w", dir, mkErr)
	}
	return nil
}

func ldapUrl(domain string, tls bool) string {
	if tls {
		return fmt.Sprintf("ldaps://%s:636", domain)
	}
	return fmt.Sprintf("ldap://%s:389", domain)
}
