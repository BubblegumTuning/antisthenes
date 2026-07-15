package config

import "sort"

// GreenPhosphorColors is a classic P1 green CRT phosphor palette (monochrome green tones).
func GreenPhosphorColors() TUIColors {
	return TUIColors{
		User:              "82",
		Assistant:         "118",
		AssistantThinking: "65",
		ToolCall:          "77",
		ToolResult:        "65",
		InputBorder:       "82",
		ThinkingBorder:    "28",
		Status:            "71",
		Title:             "82",
		Error:             "196",
		Nudge:             "71",
		Compression:       "82",
		ModalBorder:       "82",
		WindowActive:      "118",
		WindowInactive:    "71",
		WindowEmpty:       "28",
		Dim:               "65",
		EmptyChat:         "71",
	}
}

// AmberPhosphorColors is a classic P3 amber CRT phosphor palette (warm amber/orange tones).
func AmberPhosphorColors() TUIColors {
	return TUIColors{
		User:              "220",
		Assistant:         "214",
		AssistantThinking: "136",
		ToolCall:          "208",
		ToolResult:        "136",
		InputBorder:       "220",
		ThinkingBorder:    "130",
		Status:            "172",
		Title:             "220",
		Error:             "196",
		Nudge:             "172",
		Compression:       "214",
		ModalBorder:       "220",
		WindowActive:      "214",
		WindowInactive:    "172",
		WindowEmpty:       "130",
		Dim:               "136",
		EmptyChat:         "172",
	}
}

var builtinThemes = map[string]TUIColors{
	"green": GreenPhosphorColors(),
	"amber": AmberPhosphorColors(),
}

// ThemeNames returns sorted built-in theme names.
func ThemeNames() []string {
	names := make([]string, 0, len(builtinThemes))
	for name := range builtinThemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ThemeColors returns the palette for a built-in theme name (e.g. "green", "amber").
func ThemeColors(name string) (TUIColors, bool) {
	c, ok := builtinThemes[name]
	return c, ok
}
