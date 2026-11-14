package service

import "testing"

func TestResolveVendorIcon(t *testing.T) {
	icons := map[string]string{
		"anthropic":      "Claude.Color",
		"openai":         "OpenAI",
		"moonshot ai":    "Moonshot",
		"alibaba":        "Qwen.Color",
		"z.ai":           "ZAI",
		"volcengine ark": "Volcengine.Color",
		"deepseek":       "DeepSeek.Color",
		"google":         "Google.Color",
	}
	cases := []struct {
		local string
		want  string
	}{
		// Exact match, case-insensitive.
		{"Anthropic", "Claude.Color"},
		{"OpenAI", "OpenAI"},
		{"DeepSeek", "DeepSeek.Color"},
		// Alias table: the local name is Chinese or an abbreviation.
		{"通义千问", "Qwen.Color"},
		{"智谱 GLM", "ZAI"},
		{"Moonshot", "Moonshot"},
		{"火山引擎", "Volcengine.Color"},
		// Normalized fallback: same name, different spacing.
		{"Moonshot  AI", "Moonshot"},
		{"Z.AI", "ZAI"},
		// No match is a miss, not a guess.
		{"Some Unknown Vendor", ""},
	}
	for _, tc := range cases {
		if got := resolveVendorIcon(icons, tc.local); got != tc.want {
			t.Errorf("resolveVendorIcon(%q) = %q, want %q", tc.local, got, tc.want)
		}
	}
}

func TestNormalizeVendorKey(t *testing.T) {
	cases := map[string]string{
		"Moonshot AI":  "moonshotai",
		"Z.AI":         "zai",
		"MiniMax (cn)": "minimaxcn",
		"  OpenAI  ":   "openai",
	}
	for input, want := range cases {
		if got := normalizeVendorKey(input); got != want {
			t.Errorf("normalizeVendorKey(%q) = %q, want %q", input, got, want)
		}
	}
}
