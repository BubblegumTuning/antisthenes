package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.AgentName != "Antisthenes" {
		t.Errorf("expected AgentName 'Antisthenes', got %q", cfg.AgentName)
	}
	if cfg.ActiveEndpoint != "local" {
		t.Errorf("expected ActiveEndpoint 'local', got %q", cfg.ActiveEndpoint)
	}
	if len(cfg.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(cfg.Endpoints))
	}
	if cfg.MaxTokens != 160000 {
		t.Errorf("expected MaxTokens 160000, got %d", cfg.MaxTokens)
	}
	if !cfg.ShowThinking {
		t.Error("expected ShowThinking true")
	}
	if cfg.DBPath == "" || cfg.WorkDir == "" {
		t.Error("expected DBPath and WorkDir to be set")
	}
	// check endpoints
	foundLocal := false
	foundXAI := false
	for _, ep := range cfg.Endpoints {
		if ep.Name == "local" {
			foundLocal = true
			if ep.Model == "" || ep.BaseURL == "" {
				t.Error("local endpoint missing model or base_url")
			}
		}
		if ep.Name == "xai" {
			foundXAI = true
		}
	}
	if !foundLocal || !foundXAI {
		t.Error("expected both local and xai endpoints")
	}
	if !cfg.InputHistoryEnabled {
		t.Error("expected InputHistoryEnabled true by default")
	}
	if cfg.InputHistoryMax() != 50 {
		t.Errorf("expected InputHistoryMax 50, got %d", cfg.InputHistoryMax())
	}
	if !cfg.NmapOn() {
		t.Error("expected NmapEnabled true by default")
	}
	if cfg.NetworkStatusOn() {
		t.Error("expected NetworkStatusEnabled false by default")
	}
}

func TestGetActiveEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		wantName string
	}{
		{
			name: "matches active",
			cfg: Config{
				ActiveEndpoint: "local",
				Endpoints: []Endpoint{
					{Name: "local", Model: "m1"},
					{Name: "xai", Model: "m2"},
				},
			},
			wantName: "local",
		},
		{
			name: "no match falls back to first",
			cfg: Config{
				ActiveEndpoint: "missing",
				Endpoints: []Endpoint{
					{Name: "first", Model: "m1"},
					{Name: "second", Model: "m2"},
				},
			},
			wantName: "first",
		},
		{
			name: "empty endpoints returns empty",
			cfg: Config{
				ActiveEndpoint: "foo",
				Endpoints:      []Endpoint{},
			},
			wantName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := tt.cfg.GetActiveEndpoint()
			if ep.Name != tt.wantName {
				t.Errorf("got name %q want %q", ep.Name, tt.wantName)
			}
		})
	}
}

func TestLoadAndSave(t *testing.T) {
	// Use temp dir + chdir to keep hermetic; Load/Save hardcode "config.json" in cwd
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	// No config.json: should return Default and create dirs (but we don't assert side dirs strictly)
	cfg := Load()
	if cfg.AgentName != "Antisthenes" {
		t.Errorf("Load no file: expected default AgentName, got %q", cfg.AgentName)
	}

	// Modify and Save
	cfg.AgentName = "TestAgent"
	cfg.ActiveEndpoint = "xai"
	cfg.MaxTokens = 80000
	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file written
	data, err := os.ReadFile("config.json")
	if err != nil {
		t.Fatalf("config.json not created: %v", err)
	}
	if len(data) == 0 {
		t.Error("config.json empty")
	}

	// Reload
	cfg2 := Load()
	if cfg2.AgentName != "TestAgent" {
		t.Errorf("after reload: got AgentName %q want TestAgent", cfg2.AgentName)
	}
	if cfg2.ActiveEndpoint != "xai" {
		t.Errorf("after reload: got Active %q want xai", cfg2.ActiveEndpoint)
	}
	if cfg2.MaxTokens != 80000 {
		t.Errorf("after reload: got MaxTokens %d want 80000", cfg2.MaxTokens)
	}
}

func TestLoad_EmptyAgentNameInFile(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	if err := os.WriteFile("config.json", []byte(`{"agent_name":""}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.AgentName != "Antisthenes" {
		t.Errorf("empty agent_name in file should default to Antisthenes, got %q", cfg.AgentName)
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	origWD, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWD) }()

	// Write invalid JSON
	if err := os.WriteFile("config.json", []byte(`{ "agent_name": "bad", "broken": `), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	// Should fallback to default on unmarshal error
	if cfg.AgentName != "Antisthenes" {
		t.Errorf("malformed: expected default AgentName, got %q", cfg.AgentName)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	origWD, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWD) }()

	// Legacy-style config without input_history keys; DBPath/WorkDir filled by Load.
	raw := `{
  "agent_name": "Partial",
  "active_endpoint": "local",
  "endpoints": [{"name": "local", "model": "custom", "base_url": "http://example"}]
}`
	_ = os.WriteFile("config.json", []byte(raw), 0600)

	cfg := Load()
	if cfg.AgentName != "Partial" {
		t.Errorf("partial: got name %q", cfg.AgentName)
	}
	if cfg.DBPath == "" {
		t.Error("partial: expected DBPath filled")
	}
	if cfg.WorkDir == "" {
		t.Error("partial: expected WorkDir filled")
	}
	if !cfg.InputHistoryEnabled {
		t.Error("partial: omitted input_history_enabled should default true")
	}
	if cfg.InputHistorySize != 50 {
		t.Errorf("partial: expected InputHistorySize 50, got %d", cfg.InputHistorySize)
	}
}

func TestLoad_InputHistoryExplicitFalse(t *testing.T) {
	origWD, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWD) }()

	raw := `{"agent_name":"T","input_history_enabled":false,"input_history_size":10}`
	if err := os.WriteFile("config.json", []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.InputHistoryEnabled {
		t.Error("explicit false should disable history")
	}
	if cfg.InputHistorySize != 10 {
		t.Errorf("size = %d, want 10", cfg.InputHistorySize)
	}
}
