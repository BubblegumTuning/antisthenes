package agent

// ToolFunc and ToolRegistry (plus core methods) moved to registry.go per refactoring_plan.md Phase 3.
// NewToolRegistry remains the single source of truth for registration (all impls + helpers will be added here later).
// Do not add types or core methods back here.

// NewToolRegistry creates a registry with core shell + file tools + autonomous skill creation + context compression + delegation + policy.
func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{
		tools:  make(map[string]ToolFunc),
		policy: NewPolicy(),
		jobs:   newJobManager(),
	}
	registerFSTools(r)
	registerExecTools(r)
	registerSkillsTools(r)
	registerContextTools(r)
	registerMiscTools(r)
	registerApprovalTools(r)
	registerDelegateTools(r)
	registerOtherTools(r)
	registerModernTools(r)
	registerInstallTools(r)
	registerGitTools(r)
	registerProcessTools(r)
	registerHTTPTools(r)
	registerTmuxTools(r)

	return r
}

// Core methods (Register, Call, Execute, ToOpenAITools) moved to registry.go (Phase 3 first micro-step per refactoring_plan.md).
// NewToolRegistry remains the single source and entry point for all registrations.

// Thin public wrappers for config-gated registration (2026-07-22)
// These allow newToolRegistry() to conditionally register tool families.

func RegisterTmuxTools(r *ToolRegistry, enabled bool) {
	if enabled {
		registerTmuxTools(r)
	}
}

func RegisterAnsibleTools(r *ToolRegistry, enabled bool) {
	if enabled {
		registerAnsibleTools(r)
	}
}

func RegisterGitTools(r *ToolRegistry, enabled bool) {
	if enabled {
		registerGitTools(r)
	}
}

func RegisterInstallTool(r *ToolRegistry, enabled bool) {
	if enabled {
		registerInstallTools(r)
	}
}
