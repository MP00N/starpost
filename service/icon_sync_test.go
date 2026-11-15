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

// The icon feed names models the vendor's way, so an exact-only lookup silently
// misses every renamed row and reports it as "no upstream icon".
func TestResolveIconForModelNameNormalization(t *testing.T) {
	modelIcons := map[string]string{
		"gpt-5.6-sol":               "OpenAI",
		"glm-5.3-flash":             "ZAI",
		"MiniMax-M3.1-Flash":        "Minimax.Color",
		"deepseek-v4-1-flash-1017":  "Mistral.Color",
	}
	index := make(map[string]string, len(modelIcons))
	for name := range modelIcons {
		key := normalizeModelName(name)
		if current, exists := index[key]; exists && !preferFeedName(name, current) {
			continue
		}
		index[key] = name
	}

	cases := map[string]string{
		// Exact hit.
		"gpt-5.6-sol": "OpenAI",
		// Routing prefix and case.
		"global:glm-5.3-flash":      "ZAI",
		"cn:GLM-5.3-Flash":          "ZAI",
		"global:minimax-m3.1-flash": "Minimax.Color",
		// Blocked: the feed files this name under Mistral, which is a different
		// vendor. A letter fallback beats a wrong brand mark.
		"deepseek-v4.1-flash-1017": "",
		// Genuinely absent.
		"codex-auto-review": "",
	}
	for model, want := range cases {
		if got := resolveIconForModel(modelIcons, index, model); got != want {
			t.Errorf("resolveIconForModel(%q) = %q, want %q", model, got, want)
		}
	}
}

// Already-set icons win over the feed unless overwrite is on: the caller decides
// that, but resolveIconForModel must still report the feed's answer so the
// caller can compare.
func TestResolveIconForModelPrefersExactOverNormalized(t *testing.T) {
	modelIcons := map[string]string{
		"glm-5.3-flash":  "ZAI",
		"GLM_5.3_Flash":  "SomeOtherBrand",
	}
	index := make(map[string]string)
	for name := range modelIcons {
		key := normalizeModelName(name)
		if current, exists := index[key]; exists && !preferFeedName(name, current) {
			continue
		}
		index[key] = name
	}
	// An exact entry always answers for itself, whatever the index picked.
	if got := resolveIconForModel(modelIcons, index, "glm-5.3-flash"); got != "ZAI" {
		t.Errorf("exact entry should win, got %q", got)
	}
}
