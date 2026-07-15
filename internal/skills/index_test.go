package skills

import (
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
