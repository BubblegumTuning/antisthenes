package config

import "testing"

func TestThemeColors_GreenAndAmber(t *testing.T) {
	green, ok := ThemeColors("green")
	if !ok {
		t.Fatal("green theme missing")
	}
	if green.User != "82" || green.Assistant != "118" {
		t.Errorf("unexpected green phosphor palette: user=%q assistant=%q", green.User, green.Assistant)
	}

	amber, ok := ThemeColors("amber")
	if !ok {
		t.Fatal("amber theme missing")
	}
	if amber.User != "220" || amber.Assistant != "214" {
		t.Errorf("unexpected amber phosphor palette: user=%q assistant=%q", amber.User, amber.Assistant)
	}
}

func TestThemeNames(t *testing.T) {
	names := ThemeNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 themes, got %v", names)
	}
	if names[0] != "amber" || names[1] != "green" {
		t.Errorf("unexpected sort order: %v", names)
	}
}

func TestThemeColors_Unknown(t *testing.T) {
	if _, ok := ThemeColors("pink"); ok {
		t.Error("expected unknown theme to fail")
	}
}
