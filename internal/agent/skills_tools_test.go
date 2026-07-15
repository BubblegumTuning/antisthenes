package agent

import (
	"os"
	"strings"
	"testing"
)

func TestToolRegistry_SkillsTools(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)
	os.MkdirAll("skills", 0755)
	defer os.RemoveAll("skills")

	r := NewToolRegistry()

	res, err := r.Call("list_skills", map[string]any{})
	if err != nil {
		t.Logf("list_skills: %v %s", err, res)
	}

	res, err = r.Call("create_skill", map[string]any{})
	if err != nil || !strings.Contains(res, "name is required") {
		t.Errorf("create missing: %s", res)
	}

	res, err = r.Call("create_skill", map[string]any{"name": "regtestskill", "description": "desc"})
	if err != nil || !strings.Contains(res, "created successfully") {
		t.Errorf("create_skill: %v %s", err, res)
	}

	res, err = r.Call("load_skill", map[string]any{})
	if err != nil || !strings.Contains(res, "name is required") {
		t.Errorf("load_skill missing: %v %s", err, res)
	}

	res, err = r.Call("load_skill", map[string]any{"name": "regtestskill"})
	if err != nil || !strings.Contains(res, "Skill 'regtestskill' loaded:") || !strings.Contains(res, "desc") {
		t.Errorf("load_skill: %v %s", err, res)
	}

	res, err = r.Call("load_skill", map[string]any{"name": "nonexistent"})
	if err != nil || !strings.Contains(res, "not found") {
		t.Errorf("load_skill not found: %v %s", err, res)
	}
}
