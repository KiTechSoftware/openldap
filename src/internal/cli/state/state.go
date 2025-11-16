package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// State represents persisted CLI runtime metadata.
type State struct {
	AppVersion string    `toml:"app_version"`
	ConfigPath string    `toml:"config_path"`
	Verbose    bool      `toml:"verbose"`
	JSONOutput bool      `toml:"json_output"`
	LastUsed   time.Time `toml:"last_used"`
}

// internal variables
var (
	current State
	mu      sync.RWMutex
	path    string
	once    sync.Once
)

// ---------- Initialization ----------

// Init loads the persisted state from disk (creating it if missing).
func Init() error {
	var initErr error
	once.Do(func() {
		path = stateFilePath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			initErr = fmt.Errorf("failed to create state dir: %w", err)
			return
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			current = defaultState()
			initErr = Save()
			return
		}

		data, err := os.ReadFile(path)
		if err != nil {
			initErr = fmt.Errorf("failed to read state file: %w", err)
			return
		}

		if err := toml.Unmarshal(data, &current); err != nil {
			initErr = fmt.Errorf("invalid TOML in state file: %w", err)
			return
		}
		current.LastUsed = time.Now()
		_ = Save() // refresh timestamp
	})
	return initErr
}

// Path returns the full path to the current state file.
func Path() string {
	_ = Init()
	return path
}

// ---------- Accessors ----------

func Get() State {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

func SetVerbose(v bool) {
	mu.Lock()
	defer mu.Unlock()
	current.Verbose = v
	_ = Save()
}

func IsVerbose() bool {
	mu.RLock()
	defer mu.RUnlock()
	return current.Verbose
}

func SetJSONOutput(v bool) {
	mu.Lock()
	defer mu.Unlock()
	current.JSONOutput = v
	_ = Save()
}

func JSONOutputEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return current.JSONOutput
}

func SetVersion(v string) {
	mu.Lock()
	defer mu.Unlock()
	current.AppVersion = v
	_ = Save()
}

func Version() string {
	mu.RLock()
	defer mu.RUnlock()
	return current.AppVersion
}

func SetConfigPath(p string) {
	mu.Lock()
	defer mu.Unlock()
	current.ConfigPath = p
	_ = Save()
}

func ConfigPath() string {
	mu.RLock()
	defer mu.RUnlock()
	return current.ConfigPath
}

// ---------- Persistence ----------

// Save writes the current state to disk atomically.
func Save() error {
	mu.RLock()
	defer mu.RUnlock()

	current.LastUsed = time.Now()
	data, err := toml.Marshal(current)
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ---------- Internals ----------

func stateFilePath() string {
	base := os.Getenv("LDAPPY_STATE_DIR")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".ldappy")
	}
	return filepath.Join(base, "state.toml")
}

func defaultState() State {
	return State{
		AppVersion: "dev",
		ConfigPath: "/etc/ldappy/config.toml",
		LastUsed:   time.Now(),
	}
}
