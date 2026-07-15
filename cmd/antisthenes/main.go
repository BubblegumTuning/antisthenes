package main

import (
	"os"

	"github.com/nanami/antisthenes/config"
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "0.1.5"

func main() {
	cfg := config.Load()
	if cfg.DebugLogging {
		_ = os.MkdirAll("log", 0o700)
	}

	if handleSubcommand(os.Args, cfg) {
		return
	}

	// --prompt / -P one-shot non-interactive mode (extracted to run.go in Phase 2)
	if tryRunOneShot(os.Args, cfg) {
		return
	}

	runTUI(cfg)
}
