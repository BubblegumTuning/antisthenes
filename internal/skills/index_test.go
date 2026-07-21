package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSkillIndex_JSON(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	idxJSON := `{"skills":{"test":{"name":"test","description":"Test skill","path":"skills/test"}}}`
	if err := os.WriteFile(filepath.Join(skillsDir, "index.json"), []byte(idxJSON), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := NewSkillIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Skills) != 1 || idx.Skills["test"].Name != "test" {
		t.Errorf("expected 1 skill from JSON, got %d", len(idx.Skills))
	}
}

func TestNewSkillIndex_Scan(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(filepath.Join(skillsDir, "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "demo", "SKILL.md"), []byte("Demo skill line one\nrest"), 0644); err != nil {
		t.Fatal(err)
	}
	// Dir without SKILL.md must be ignored.
	if err := os.MkdirAll(filepath.Join(skillsDir, "empty"), 0755); err != nil {
		t.Fatal(err)
	}

	idx, err := NewSkillIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Skills) != 1 {
		t.Fatalf("expected 1 skill from scan, got %d", len(idx.Skills))
	}
	if idx.Skills["demo"].Description != "Demo skill line one" {
		t.Error("description not trimmed from first line")
	}
}

func TestListAndLoad(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(filepath.Join(skillsDir, "alpha"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "alpha", "SKILL.md"), []byte("Alpha"), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := NewSkillIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	list := idx.List()
	if len(list) != 1 {
		t.Fatalf("List len = %d", len(list))
	}
	content, err := idx.Load("alpha")
	if err != nil || content != "Alpha" {
		t.Errorf("Load failed: %v", err)
	}
}

func TestGenerateIndex(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(filepath.Join(skillsDir, "beta"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "beta", "SKILL.md"), []byte("Beta"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateIndex(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "index.json")); err != nil {
		t.Error("index.json not created")
	}
}

// TestGenerateIndex_RescansStaleJSON ensures GenerateIndex discovers new skill dirs
// even when a stale index.json already exists (create_skill / `antisthenes index` path).
func TestGenerateIndex_RescansStaleJSON(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(filepath.Join(skillsDir, "old"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "old", "SKILL.md"), []byte("# Old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Stale index omits "new" skill that will be added on disk.
	stale := `{"skills":{"old":{"name":"old","description":"# Old","path":"skills/old"}}}`
	if err := os.WriteFile(filepath.Join(skillsDir, "index.json"), []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(skillsDir, "new"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "new", "SKILL.md"), []byte("# New skill\nbody"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateIndex(root); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(skillsDir, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got SkillIndex
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Skills) != 2 {
		t.Fatalf("expected 2 skills after rescan, got %d: %+v", len(got.Skills), got.Skills)
	}
	if got.Skills["new"].Description != "# New skill" {
		t.Errorf("new skill description = %q", got.Skills["new"].Description)
	}
	if got.Skills["new"].Path != "skills/new" {
		t.Errorf("new skill path = %q", got.Skills["new"].Path)
	}

	// Load path via index after regenerate.
	idx, err := NewSkillIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	content, err := idx.Load("new")
	if err != nil || content != "# New skill\nbody" {
		t.Errorf("Load new after GenerateIndex: content=%q err=%v", content, err)
	}
}
