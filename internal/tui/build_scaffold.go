package tui

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// buildScaffold holds paths for design, definition-of-done, and work-log files.
type buildScaffold struct {
	DesignFile string
	DodFile    string
	LogFile    string
}

func scaffoldSlug(text string, maxLen int, defaultSlug string) string {
	if len(text) > maxLen {
		return strings.ToLower(strings.ReplaceAll(text[:maxLen], " ", "_"))
	}
	return defaultSlug
}

func scaffoldFilePaths(targetDir, slug string) (designFile, dodFile, logFile string) {
	timestamp := time.Now().Format("20060102-150405")
	designFile = fmt.Sprintf("%s/%s_%s_design.md", targetDir, timestamp, slug)
	dodFile = fmt.Sprintf("%s/%s_%s_definition_of_done.md", targetDir, timestamp, slug)
	logFile = fmt.Sprintf("%s/%s_%s_work.log", targetDir, timestamp, slug)
	return
}

func writeScaffoldFiles(targetDir string, designFile, dodFile, logFile, designContent, dodContent, logContent string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	if designContent != "" {
		_ = os.WriteFile(designFile, []byte(designContent), 0o644)
	}
	if dodContent != "" {
		_ = os.WriteFile(dodFile, []byte(dodContent), 0o644)
	}
	if logContent != "" {
		_ = os.WriteFile(logFile, []byte(logContent), 0o644)
	}
	return nil
}
