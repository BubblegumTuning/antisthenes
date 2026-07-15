package agent

import (
	"os"
	"time"
)

// registerMiscTools registers basic/misc tools (get_current_time, echo, get_env).
func registerMiscTools(r *ToolRegistry) {
	r.Register("get_current_time", func(args map[string]any) (string, error) {
		return time.Now().Format(time.RFC3339), nil
	})

	r.Register("echo", func(args map[string]any) (string, error) {
		if msg, ok := args["message"].(string); ok {
			return msg, nil
		}
		return "echo: no message provided", nil
	})

	r.Register("get_env", func(args map[string]any) (string, error) {
		key, ok := args["key"].(string)
		if !ok || key == "" {
			return "get_env: key is required", nil
		}
		return os.Getenv(key), nil
	})
}
