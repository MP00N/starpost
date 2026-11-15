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
		"gpt-5.6-sol":              "OpenAI",
		"glm-5.3-flash":            "ZAI",
		"MiniMax-M3.1-Flash":       "Minimax.Color",
		"deepseek-v4-1-flash-1017": "Mistral.Color",
	}
	index := buildIconIndex(modelIcons)

	cases := map[string]struct {
		icon   string
		source iconMatchSource
	}{
		// Exact hit.
		"gpt-5.6-sol": {"OpenAI", iconMatchExact},
		// Routing prefix and case, answered by the feed's normalized row.
		"global:glm-5.3-flash":      {"ZAI", iconMatchFeed},
		"cn:GLM-5.3-Flash":          {"ZAI", iconMatchFeed},
		"global:minimax-m3.1-flash": {"Minimax.Color", iconMatchFeed},
		// The feed files this name under Mistral — a different vendor. The feed
		// row is refused, but the name still says DeepSeek, so the reading pass
		// answers instead of leaving the row brandless.
		"deepseek-v4.1-flash-1017": {"DeepSeek.Color", iconMatchName},
		// Absent from the feed, brand legible in the name.
		"codex-auto-review": {"Codex.Color", iconMatchName},
		// Neither feed nor name: an honest miss.
		"zzz-mystery-model": {"", iconMatchNone},
	}
	for model, want := range cases {
		icon, source := resolveIconForModel(modelIcons, index, model)
		if icon != want.icon || source != want.source {
			t.Errorf("resolveIconForModel(%q) = (%q, %q), want (%q, %q)",
				model, icon, source, want.icon, want.source)
		}
	}
}

// Already-set icons win over the feed unless overwrite is on: the caller decides
// that, but resolveIconForModel must still report the feed's answer so the
// caller can compare.
func TestResolveIconForModelPrefersExactOverNormalized(t *testing.T) {
	modelIcons := map[string]string{
		"glm-5.3-flash": "ZAI",
		"GLM_5.3_Flash": "SomeOtherBrand",
	}
	index := buildIconIndex(modelIcons)
	// An exact entry always answers for itself, whatever the index picked.
	if icon, _ := resolveIconForModel(modelIcons, index, "glm-5.3-flash"); icon != "ZAI" {
		t.Errorf("exact entry should win, got %q", icon)
	}
}

// Reading a brand off the name is the last tier, so published data always wins.
func TestInferIconFromModelName(t *testing.T) {
	cases := map[string]string{
		// The two the production run could not place.
		"codex-auto-review":         "Codex.Color",
		"global:hy4-preview-f":      "Hunyuan.Color",
		"global:hy4free":            "Hunyuan.Color",
		"hy4":                       "Hunyuan.Color",
		// Routing prefixes and vendor paths do not hide the brand.
		"cn:deepseek-v4.1-flash":    "DeepSeek.Color",
		"deepseek-ai/DeepSeek-V3.1": "DeepSeek.Color",
		// Longest stem wins: chatglm must not be decided by glm.
		"chatglm-4-plus": "ZAI",
		"glm-5.3":        "ZAI",
		// A name with no brand token stays unset rather than guessing.
		"":                  "",
		"zzz-mystery-model": "",
	}
	for model, want := range cases {
		if got := inferIconFromModelName(model); got != want {
			t.Errorf("inferIconFromModelName(%q) = %q, want %q", model, got, want)
		}
	}
}

// buildIconIndex lives in icon_sync.go so the tests exercise the same
// construction the run uses rather than a copy of it.
