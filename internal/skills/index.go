package skills

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SkillMeta is the tiny description entry in the index for awareness without loading full content.
type SkillMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"` // Tiny one-liner for context awareness
	Path        string `json:"path"`        // Relative path to skill folder
}

// SkillIndex holds discovered skills for lazy loading.
type SkillIndex struct {
	Skills map[string]SkillMeta `json:"skills"`
	root   string
}

// NewSkillIndex scans the skills/ directory or loads from index.json if present.
func NewSkillIndex(root string) (*SkillIndex, error) {
	idx := &SkillIndex{
		Skills: make(map[string]SkillMeta),
		root:   root,
	}

	// Prefer index.json if it exists
	indexPath := filepath.Join(root, "skills", "index.json")
	if data, err := os.ReadFile(indexPath); err == nil {
		if err := json.Unmarshal(data, idx); err == nil {
			return idx, nil
		}
	}

	// Fallback: scan skills/ dir for folders containing SKILL.md
	skillsDir := filepath.Join(root, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return idx, nil
	}

	for _, e := range entries {
		if e.IsDir() {
			meta := SkillMeta{
				Name:        e.Name(),
				Description: "Skill: " + e.Name(),
				Path:        filepath.Join("skills", e.Name()),
			}
			mdPath := filepath.Join(skillsDir, e.Name(), "SKILL.md")
			if content, err := os.ReadFile(mdPath); err == nil && len(content) > 0 {
				lines := string(content)
				if idx := bytes.IndexByte([]byte(lines), '\n'); idx > 0 {
					lines = lines[:idx]
				}
				if len(lines) > 80 {
					lines = lines[:80]
				}
				meta.Description = lines
			}
			idx.Skills[e.Name()] = meta
		}
	}
	return idx, nil
}

// List returns all available skill metas.
func (i *SkillIndex) List() []SkillMeta {
	list := make([]SkillMeta, 0, len(i.Skills))
	for _, m := range i.Skills {
		list = append(list, m)
	}
	return list
}

// Load reads the full skill content (lazy).
func (i *SkillIndex) Load(name string) (string, error) {
	meta, ok := i.Skills[name]
	if !ok {
		return "", fmt.Errorf("skill %s not found in index", name)
	}
	mdPath := filepath.Join(i.root, meta.Path, "SKILL.md")
	content, err := os.ReadFile(mdPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// GenerateIndex creates or updates skills/index.json from the current skills directory.
func GenerateIndex(root string) error {
	idx, err := NewSkillIndex(root) // uses scan fallback
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}

	indexPath := filepath.Join(root, "skills", "index.json")
	return os.WriteFile(indexPath, data, 0644)
}
