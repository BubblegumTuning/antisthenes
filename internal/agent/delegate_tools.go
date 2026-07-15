package agent

// registerDelegateTools registers delegation-related tools.
func registerDelegateTools(r *ToolRegistry) {
	r.Register("delegate_task", func(args map[string]any) (string, error) {
		goal, ok := args["goal"].(string)
		if !ok || goal == "" {
			return "delegate_task: goal is required", nil
		}
		cfg := DefaultDelegateConfig()
		if exec, ok := args["executor"].(string); ok && exec != "" {
			cfg.ExecutorName = exec
		}
		result := DelegateTaskWithConfig(goal, cfg)
		if result.Error != nil {
			return "", result.Error
		}
		return result.Result, nil
	})
}
