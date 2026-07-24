package config

import (
	"os"
	"path/filepath"
	"strings"
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
	if strings.Contains(cfg.DBPath, "demo") {
		t.Errorf("default DBPath must not contain demo: %s", cfg.DBPath)
	}
	if !strings.HasSuffix(cfg.DBPath, "antisthenes.db") {
		t.Errorf("expected antisthenes.db suffix, got %s", cfg.DBPath)
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
	if cfg.IterativeContextRemindPercent() != DefaultIterativeContextRemindPercent {
		t.Errorf("remind default: got %d", cfg.IterativeContextRemindPercent())
	}
	if cfg.IterativeContextSummaryPercent() != DefaultIterativeContextSummaryPercent {
		t.Errorf("summary default: got %d", cfg.IterativeContextSummaryPercent())
	}
	if cfg.IterativeMaxIterations() != DefaultIterativeMaxIterations {
		t.Errorf("max_iterations default: got %d", cfg.IterativeMaxIterations())
	}
	if cfg.ResolvedHTTPUserAgent() != DefaultHTTPUserAgent {
		t.Errorf("HTTP UA default: got %q", cfg.ResolvedHTTPUserAgent())
	}
	if cfg.HTTPUserAgent != DefaultHTTPUserAgent {
		t.Errorf("DefaultConfig.HTTPUserAgent: got %q", cfg.HTTPUserAgent)
	}
}

func TestResolvedHTTPUserAgent(t *testing.T) {
	if (Config{}).ResolvedHTTPUserAgent() != DefaultHTTPUserAgent {
		t.Fatalf("empty config should resolve default")
	}
	if (Config{HTTPUserAgent: "  "}).ResolvedHTTPUserAgent() != DefaultHTTPUserAgent {
		t.Fatalf("whitespace should resolve default")
	}
	const custom = "MyBot/2.0 (+https://example.test)"
	if (Config{HTTPUserAgent: custom}).ResolvedHTTPUserAgent() != custom {
		t.Fatalf("custom UA not preserved")
	}
}

func TestIterativeThresholdAccessors(t *testing.T) {
	// Zero values → defaults
	z := Config{}
	if z.IterativeContextRemindPercent() != 55 || z.IterativeContextSummaryPercent() != 60 || z.IterativeMaxIterations() != 40 {
		t.Fatalf("zero config accessors: remind=%d summary=%d max=%d",
			z.IterativeContextRemindPercent(), z.IterativeContextSummaryPercent(), z.IterativeMaxIterations())
	}
	// Custom values
	c := Config{Iterative: IterativeSettings{ContextRemindPercent: 40, ContextSummaryPercent: 70, MaxIterations: 12}}
	if c.IterativeContextRemindPercent() != 40 || c.IterativeContextSummaryPercent() != 70 || c.IterativeMaxIterations() != 12 {
		t.Fatalf("custom: %+v accessors r=%d s=%d m=%d", c.Iterative, c.IterativeContextRemindPercent(), c.IterativeContextSummaryPercent(), c.IterativeMaxIterations())
	}
	// Summary below remind clamps up to remind
	c2 := Config{Iterative: IterativeSettings{ContextRemindPercent: 70, ContextSummaryPercent: 50}}
	if c2.IterativeContextSummaryPercent() != 70 {
		t.Fatalf("summary should clamp to remind, got %d", c2.IterativeContextSummaryPercent())
	}
	// Out of range percent → default
	c3 := Config{Iterative: IterativeSettings{ContextRemindPercent: 150, ContextSummaryPercent: -1, MaxIterations: 0}}
	if c3.IterativeContextRemindPercent() != 55 {
		t.Fatalf("invalid remind → 55, got %d", c3.IterativeContextRemindPercent())
	}
	if c3.IterativeContextSummaryPercent() != 60 {
		t.Fatalf("invalid summary → 60, got %d", c3.IterativeContextSummaryPercent())
	}
	if c3.IterativeMaxIterations() != 40 {
		t.Fatalf("zero max → 40, got %d", c3.IterativeMaxIterations())
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

	if err := os.WriteFile("config.json", []byte(`{"agent_name":""}`), 0o600); err != nil {
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
	if err := os.WriteFile("config.json", []byte(`{ "agent_name": "bad", "broken": `), 0o600); err != nil {
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
	_ = os.WriteFile("config.json", []byte(raw), 0o600)

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
	if err := os.WriteFile("config.json", []byte(raw), 0o600); err != nil {
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

func TestPathEnvOverrides(t *testing.T) {
	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origWD) }()

	dataDir := filepath.Join(tmp, "data-root")
	dbOverride := filepath.Join(tmp, "custom.db")
	workOverride := filepath.Join(tmp, "custom-work")

	t.Setenv(EnvDataDir, dataDir)
	t.Setenv(EnvDBPath, "")
	t.Setenv(EnvDB, "")
	t.Setenv(EnvWorkDir, "")
	// Empty config → data dir defaults
	_ = os.WriteFile("config.json", []byte(`{"agent_name":"E"}`), 0o600)
	cfg := Load()
	if cfg.DBPath != filepath.Join(dataDir, "antisthenes.db") {
		t.Fatalf("data dir db: got %s", cfg.DBPath)
	}
	if cfg.WorkDir != filepath.Join(dataDir, "work") {
		t.Fatalf("data dir work: got %s", cfg.WorkDir)
	}

	t.Setenv(EnvDBPath, dbOverride)
	t.Setenv(EnvWorkDir, workOverride)
	cfg = Load()
	if cfg.DBPath != dbOverride || cfg.WorkDir != workOverride {
		t.Fatalf("specific env: db=%s work=%s", cfg.DBPath, cfg.WorkDir)
	}

	// Config path ignored when specific env set
	_ = os.WriteFile("config.json", []byte(`{"db_path":"/from/config.db","work_dir":"/from/config-work"}`), 0o600)
	cfg = Load()
	if cfg.DBPath != dbOverride {
		t.Fatalf("env must win over config db: %s", cfg.DBPath)
	}

	// No specific env → config path wins over data dir
	t.Setenv(EnvDBPath, "")
	t.Setenv(EnvDB, "")
	t.Setenv(EnvWorkDir, "")
	t.Setenv(EnvDataDir, dataDir)
	cfg = Load()
	if cfg.DBPath != "/from/config.db" || cfg.WorkDir != "/from/config-work" {
		t.Fatalf("config should win without specific env: db=%s work=%s", cfg.DBPath, cfg.WorkDir)
	}
}

func TestResolveAuxModel(t *testing.T) {
	cfg := Config{AuxModels: []AuxModel{
		{Name: "a", Model: "m1", BaseURL: "http://x", Roles: []string{"title"}},
		{Name: "b", Model: "m2", BaseURL: "http://y", Roles: []string{"summarize"}},
	}}
	m, ok := cfg.ResolveAuxModel("title")
	if !ok || m.Name != "a" {
		t.Fatalf("title: %+v ok=%v", m, ok)
	}
	m, ok = cfg.FindAuxModel("B")
	if !ok || m.Model != "m2" {
		t.Fatalf("find: %+v", m)
	}
}
