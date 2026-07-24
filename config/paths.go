package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Environment variable names for path overrides.
// Precedence (highest first): ANTISTHENES_DB_PATH|ANTISTHENES_DB / ANTISTHENES_WORK_DIR
// → config.json non-empty values → ANTISTHENES_DATA_DIR derived paths → DefaultDataDir defaults.
const (
	EnvDataDir = "ANTISTHENES_DATA_DIR"
	EnvDBPath  = "ANTISTHENES_DB_PATH"
	EnvDB      = "ANTISTHENES_DB" // alias for EnvDBPath
	EnvWorkDir = "ANTISTHENES_WORK_DIR"
)

// DefaultDataDir returns the durable data root (~/.antisthenes, or $XDG_DATA_HOME/antisthenes).
// Falls back to $TMPDIR/antisthenes-data only when the home directory is unavailable.
func DefaultDataDir() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
		return filepath.Join(xdg, "antisthenes")
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(os.TempDir(), "antisthenes-data")
	}
	return filepath.Join(home, ".antisthenes")
}

// DefaultDBPath is data_dir/antisthenes.db (no "demo" suffix).
func DefaultDBPath() string {
	return filepath.Join(DefaultDataDir(), "antisthenes.db")
}

// DefaultWorkDir is data_dir/work.
func DefaultWorkDir() string {
	return filepath.Join(DefaultDataDir(), "work")
}

// applyPathDefaults fills empty DBPath/WorkDir from data-dir env or durable defaults,
// then applies specific env overrides (which always win).
func applyPathDefaults(cfg *Config) {
	dataDirEnv := strings.TrimSpace(os.Getenv(EnvDataDir))
	dbEnv := strings.TrimSpace(os.Getenv(EnvDBPath))
	if dbEnv == "" {
		dbEnv = strings.TrimSpace(os.Getenv(EnvDB))
	}
	workEnv := strings.TrimSpace(os.Getenv(EnvWorkDir))

	base := DefaultDataDir()
	if dataDirEnv != "" {
		base = dataDirEnv
	}

	if strings.TrimSpace(cfg.DBPath) == "" {
		cfg.DBPath = filepath.Join(base, "antisthenes.db")
	}
	if strings.TrimSpace(cfg.WorkDir) == "" {
		cfg.WorkDir = filepath.Join(base, "work")
	}

	// Specific env vars always override config.json.
	if dbEnv != "" {
		cfg.DBPath = dbEnv
	}
	if workEnv != "" {
		cfg.WorkDir = workEnv
	}
}

// ensureDataDirs creates parent of DBPath and WorkDir (best-effort).
func ensureDataDirs(cfg Config) {
	if p := strings.TrimSpace(cfg.DBPath); p != "" {
		_ = os.MkdirAll(filepath.Dir(p), 0o700)
	}
	if d := strings.TrimSpace(cfg.WorkDir); d != "" {
		_ = os.MkdirAll(d, 0o700)
	}
}
