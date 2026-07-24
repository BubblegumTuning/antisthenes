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

// NewSkillIndex loads skills/index.json when present and valid; otherwise scans skills/ for SKILL.md dirs.
func NewSkillIndex(root string) (*SkillIndex, error) {
	idx := &SkillIndex{
		Skills: make(map[string]SkillMeta),
		root:   root,
	}

	// Prefer index.json if it exists and unmarshals cleanly.
	indexPath := filepath.Join(root, "skills", "index.json")
	if data, err := os.ReadFile(indexPath); err == nil {
		if err := json.Unmarshal(data, idx); err == nil && idx.Skills != nil {
			idx.root = root
			return idx, nil
		}
	}

	return scanSkillsDir(root), nil
}

// scanSkillsDir discovers directories under root/skills that contain SKILL.md.
func scanSkillsDir(root string) *SkillIndex {
	idx := &SkillIndex{
		Skills: make(map[string]SkillMeta),
		root:   root,
	}

	skillsDir := filepath.Join(root, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return idx
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		mdPath := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		content, err := os.ReadFile(mdPath)
		if err != nil || len(content) == 0 {
			continue
		}
		meta := SkillMeta{
			Name:        e.Name(),
			Description: "Skill: " + e.Name(),
			Path:        filepath.Join("skills", e.Name()),
		}
		lines := string(content)
		if i := bytes.IndexByte([]byte(lines), '\n'); i > 0 {
			lines = lines[:i]
		}
		if len(lines) > 80 {
			lines = lines[:80]
		}
		meta.Description = lines
		idx.Skills[e.Name()] = meta
	}
	return idx
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

// GenerateIndex creates or updates skills/index.json by scanning the skills directory
// (always rescans; does not reuse a stale index.json).
func GenerateIndex(root string) error {
	idx := scanSkillsDir(root)

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}

	indexPath := filepath.Join(root, "skills", "index.json")
	return os.WriteFile(indexPath, data, 0o644)
}
