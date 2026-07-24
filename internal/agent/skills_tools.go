package agent

import (
	"fmt"
	"os"
	"strings"

	"github.com/nanami/antisthenes/internal/skills"
)

// registerSkillsTools registers skill-related tools (list_skills, load_skill, create_skill).
func registerSkillsTools(r *ToolRegistry) {
	r.Register("list_skills", func(args map[string]any) (string, error) {
		idx, err := skills.NewSkillIndex(".")
		if err != nil {
			return "list_skills: failed to load index", nil
		}
		var names []string
		for _, s := range idx.List() {
			names = append(names, s.Name+": "+s.Description)
		}
		if len(names) == 0 {
			return "No skills available", nil
		}
		return "Available skills:\n" + strings.Join(names, "\n"), nil
	})

	r.Register("load_skill", func(args map[string]any) (string, error) {
		name, ok := args["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return "load_skill: name is required", nil
		}
		name = strings.TrimSpace(name)
		if strings.Contains(name, "/") || strings.Contains(name, "..") {
			return "load_skill: invalid skill name", nil
		}

		idx, err := skills.NewSkillIndex(".")
		if err != nil {
			return "load_skill: failed to load index", nil
		}
		content, err := idx.Load(name)
		if err != nil {
			return "load_skill: " + err.Error(), nil
		}
		return fmt.Sprintf("Skill '%s' loaded:\n\n%s", name, content), nil
	})

	r.Register("create_skill", func(args map[string]any) (string, error) {
		name, ok := args["name"].(string)
		if !ok || name == "" {
			return "create_skill: name is required", nil
		}

		desc, _ := args["description"].(string)
		if desc == "" {
			desc = "Auto-created skill"
		}

		skillDir := "skills/" + name
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return "", err
		}

		content := fmt.Sprintf("# %s\n\n%s\n", name, desc)
		if err := os.WriteFile(skillDir+"/SKILL.md", []byte(content), 0o644); err != nil {
			return "", err
		}

		_ = skills.GenerateIndex(".")

		return fmt.Sprintf("Skill '%s' created successfully. Index regenerated.", name), nil
	})
}
