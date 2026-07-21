package config

import "strings"

// AuxModel is a secondary OpenAI-compatible endpoint for cheap/async work
// (session titles, summarization, optional delegate executors).
type AuxModel struct {
	Name    string `json:"name"`
	Model   string `json:"model"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
	// Roles tags this model for automatic selection (e.g. "title", "summarize", "delegate").
	Roles []string `json:"roles,omitempty"`
}

// ResolveAuxModel returns the first aux model that lists role (case-insensitive),
// or the first aux model when role is empty. ok is false when none are configured.
func (c Config) ResolveAuxModel(role string) (AuxModel, bool) {
	role = strings.ToLower(strings.TrimSpace(role))
	if len(c.AuxModels) == 0 {
		return AuxModel{}, false
	}
	if role == "" {
		return c.AuxModels[0], true
	}
	for _, m := range c.AuxModels {
		for _, r := range m.Roles {
			if strings.ToLower(strings.TrimSpace(r)) == role {
				return m, true
			}
		}
	}
	// Fall back to first model so optional aux still works without roles.
	return c.AuxModels[0], true
}

// ListAuxModels returns a copy of configured auxiliary models.
func (c Config) ListAuxModels() []AuxModel {
	if len(c.AuxModels) == 0 {
		return nil
	}
	out := make([]AuxModel, len(c.AuxModels))
	copy(out, c.AuxModels)
	return out
}

// FindAuxModel returns an aux model by name (case-insensitive).
func (c Config) FindAuxModel(name string) (AuxModel, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return AuxModel{}, false
	}
	for _, m := range c.AuxModels {
		if strings.ToLower(strings.TrimSpace(m.Name)) == name {
			return m, true
		}
	}
	return AuxModel{}, false
}
