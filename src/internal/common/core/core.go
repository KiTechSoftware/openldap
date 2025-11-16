package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kitechsoftware/ldappy/internal/common/config"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"
)

// ---------- Core Lifecycle ----------

func Load(ctx context.Context, path string) (*config.Config, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cfg, err := ensureConfig(path)
	if err != nil {
		return nil, err
	}

	if cfg.LDAP.Domain == "" {
		return nil, fmt.Errorf("missing ldap.domain in configuration")
	}
	if cfg.LDAP.Organization == "" {
		return nil, fmt.Errorf("missing ldap.organization in configuration")
	}

	if cfg.LDAP.BaseDN == "" && cfg.LDAP.Domain != "" {
		parts := strings.Split(cfg.LDAP.Domain, ".")
		dcs := make([]string, len(parts))
		for i, p := range parts {
			dcs[i] = fmt.Sprintf("dc=%s", p)
		}
		cfg.LDAP.BaseDN = strings.Join(dcs, ",")
		log.WarnCtx(ctx, "Derived BaseDN from domain: %s", cfg.LDAP.BaseDN)
	}

	return cfg, nil
}

// resolveConfigPath determines which config path to use based on:
// 1. CLI argument
// 2. Environment variable LDAP_CONFIG
// 3. Default application config
func resolveConfigPath(path string) (string, string) {
	if path != "" {
		return "custom", path
	}
	if envPath := os.Getenv("LDAP_CONFIG"); envPath != "" {
		return "env", envPath
	}
	return "app", config.ConfigPath()
}

// ensureConfig guarantees a valid configuration is available.
func ensureConfig(path string) (*config.Config, error) {
	version, configPath := resolveConfigPath(path)
	appConfigPath := config.ConfigPath()

	// dispatch based on source type
	switch version {
	case "app":
		return ensureAppConfig(configPath)
	default:
		return ensureExternalConfig(version, configPath, appConfigPath)
	}
}

// ensureAppConfig handles loading or creating the main app config.
func ensureAppConfig(appConfigPath string) (*config.Config, error) {
	if config.Exist(appConfigPath) {
		log.Success("Loaded app config from %s", appConfigPath)
		return config.Load(appConfigPath)
	}

	log.Warn("App config not found at %s", appConfigPath)
	return createDefaultAppConfig()
}

// ensureExternalConfig handles custom or env configs and fallbacks.
func ensureExternalConfig(version, externalPath, appConfigPath string) (*config.Config, error) {
	if !config.Exist(externalPath) {
		log.Warn("%s config not found at %s", version, externalPath)
		log.Info("Falling back to app config...")
		return ensureAppConfig(appConfigPath)
	}

	if !config.Exist(appConfigPath) {
		return mirrorExternalToApp(version, externalPath, appConfigPath)
	}

	log.Success("Using %s config from %s", version, externalPath)
	return config.Load(externalPath)
}

// mirrorExternalToApp saves the external config as the app config if missing.
func mirrorExternalToApp(version, externalPath, appConfigPath string) (*config.Config, error) {
	log.Warn("App config not found at %s", appConfigPath)
	log.Info("Saving %s config as app config...", version)

	if err := ensureWritableDir(appConfigPath); err != nil {
		return nil, fmt.Errorf("app config directory not writable: %w", err)
	}

	existingCfg, err := config.Load(externalPath)
	if err != nil {
		log.Error("Failed to load existing %s config: %v", version, err)
		return createDefaultAppConfig()
	}

	if err := config.Save(appConfigPath, existingCfg); err != nil {
		log.Error("Failed to save %s as app config: %v", version, err)
		return ensureAppConfig(appConfigPath)
	}

	log.Success("Saved %s config as app config at %s", version, appConfigPath)
	return existingCfg, nil
}

func createDefaultAppConfig() (*config.Config, error) {
	appPath := config.ConfigPath()

	if err := ensureWritableDir(appPath); err != nil {
		return nil, fmt.Errorf("config directory not writable: %w", err)
	}

	defaultCfg := config.Default()
	log.Success("Created default app config at %s", appPath)

	if err := config.Save(appPath, &defaultCfg); err != nil {
		return &defaultCfg, fmt.Errorf("failed to save default app config: %w", err)
	}
	return &defaultCfg, nil
}

// ensureWritableDir checks if a directory exists and is writable.
// If it doesn't exist, it creates it with 0755 permissions.
func ensureWritableDir(path string) error {
	dir := filepath.Dir(path)

	// 1. Check existence
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory missing → create it
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return fmt.Errorf("failed to create config directory %s: %w", dir, mkErr)
			}
			return nil
		}
		return fmt.Errorf("failed to stat config directory %s: %w", dir, err)
	}

	// 2. Must be a directory
	if !info.IsDir() {
		return fmt.Errorf("config path %s is not a directory", dir)
	}

	// 3. Check writability by creating a temp file
	testFile := filepath.Join(dir, ".state.toml")
	f, err := os.CreateTemp(dir, ".state.toml")
	if err != nil {
		return fmt.Errorf("config directory %s is not writable: %w", dir, err)
	}
	f.Close()
	_ = os.Remove(testFile)

	return nil
}
