package agent

import (
	"github.com/nanami/antisthenes/internal/context"
)

// registerContextTools registers context-related tools (dump_work_summary, load_work_summary).
func registerContextTools(r *ToolRegistry) {
	r.Register("dump_work_summary", func(args map[string]any) (string, error) {
		summary, ok := args["summary"].(string)
		if !ok || summary == "" {
			return "dump_work_summary: summary is required", nil
		}
		sessionID := "unknown"
		if s, ok := args["session_id"].(string); ok {
			sessionID = s
		}
		path, err := context.DumpWorkSummary(sessionID, summary)
		if err != nil {
			return "", err
		}
		return "Work summary dumped to: " + path, nil
	})

	r.Register("load_work_summary", func(args map[string]any) (string, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return "load_work_summary: path is required", nil
		}
		content, err := context.LoadWorkSummary(path)
		if err != nil {
			return "", err
		}
		return content, nil
	})
}
