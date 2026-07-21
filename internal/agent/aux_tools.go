package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nanami/antisthenes/config"
)

// RegisterAuxTools registers list_aux_models and complete_with_aux when cfg is provided.
// Call from bootstrap after NewToolRegistry (registry has no config at construction).
func RegisterAuxTools(r *ToolRegistry, cfg config.Config) {
	if r == nil {
		return
	}
	// Snapshot for tool closures.
	models := cfg.ListAuxModels()
	cfgCopy := cfg

	r.Register("list_aux_models", func(args map[string]any) (string, error) {
		if len(models) == 0 {
			return "No auxiliary models configured (config.aux_models is empty).", nil
		}
		type row struct {
			Name    string   `json:"name"`
			Model   string   `json:"model"`
			BaseURL string   `json:"base_url"`
			Roles   []string `json:"roles,omitempty"`
		}
		out := make([]row, 0, len(models))
		for _, m := range models {
			out = append(out, row{Name: m.Name, Model: m.Model, BaseURL: m.BaseURL, Roles: m.Roles})
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return "", err
		}
		return string(b), nil
	})

	r.Register("complete_with_aux", func(args map[string]any) (string, error) {
		name, _ := args["name"].(string)
		prompt, _ := args["prompt"].(string)
		prompt = strings.TrimSpace(prompt)
		if prompt == "" {
			return "complete_with_aux: prompt is required", nil
		}
		var m config.AuxModel
		var ok bool
		if strings.TrimSpace(name) != "" {
			m, ok = cfgCopy.FindAuxModel(name)
			if !ok {
				return fmt.Sprintf("complete_with_aux: unknown aux model %q (use list_aux_models)", name), nil
			}
		} else {
			m, ok = cfgCopy.ResolveAuxModel("")
			if !ok {
				return "complete_with_aux: no auxiliary models configured", nil
			}
		}
		system, _ := args["system"].(string)
		out, err := CompleteAux(context.Background(), m, system, prompt)
		if err != nil {
			return "", err
		}
		return out, nil
	})
}
