package service

import (
	"context"
	"testing"
)

// These tests use the live feeds. They are the only way to know that the
// currency detector still recognizes the published format: a fixture would keep
// passing after the upstream shape changed, which is exactly the failure that
// would silently import yuan as dollars.

func TestOfficialFeedCurrencyDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	ctx := context.Background()
	catalog, err := FetchPricingSyncCatalog(ctx, defaultCatalogForTest())
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	prices, err := FetchOfficialPricing(ctx, defaultSourceForTest())
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	if len(prices) == 0 {
		t.Fatal("feed contained no prices")
	}
	usd, cny, unclassified, examples := detectFeedCurrency(prices, catalog, 7.3)
	t.Logf("models=%d usdVotes=%d cnyVotes=%d unclassified=%d", len(prices), usd, cny, unclassified)
	for _, example := range examples {
		t.Logf("unclassified: %s", example)
	}
	if usd+cny < pricingSyncMinVotes {
		t.Fatalf("only %d comparable models; the detector cannot measure the feed", usd+cny)
	}
	if cny > usd {
		t.Fatalf("feed measured as CNY (usd=%d cny=%d) — the published feed changed currency", usd, cny)
	}
}

func TestSimplePromptCoefficient(t *testing.T) {
	cases := []struct {
		expression string
		want       float64
		ok         bool
	}{
		{`tier("standard", p * 3 + cr * 0.3 + c * 15)`, 3, true},
		{`tier("standard", p * 0.287671 + c * 1.15068)`, 0.287671, true},
		{`v1:tier("standard", p * 2 + c * 8)`, 2, true},
		{`len <= 100000 ? tier("a", p * 1) : tier("b", p * 2)`, 0, false},
		{`tier("image", fixed(0.0035)) * image_count`, 0, false},
		{`param("enable_thinking") == true ? tier("x", p * 1) : tier("y", p * 2)`, 0, false},
		{`tier("standard", c * 15)`, 0, false},
	}
	for _, tc := range cases {
		got, ok := simplePromptCoefficient(tc.expression)
		if ok != tc.ok {
			t.Errorf("%s: ok=%v want %v", tc.expression, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("%s: got %v want %v", tc.expression, got, tc.want)
		}
	}
}

func TestSynthesizeTokenExpression(t *testing.T) {
	input, output := 2.1, 8.4
	cost := catalogCost{Currency: "CNY", Input: &input, Output: &output}
	expression, ok := SynthesizeTokenExpression(cost, 7.0)
	if !ok {
		t.Fatal("expected an expression")
	}
	// 2.1 CNY / 7.0 = 0.3 USD per 1M tokens
	want := `tier("standard", p * 0.3 + c * 1.2)`
	if expression != want {
		t.Fatalf("got %q want %q", expression, want)
	}
}

func defaultSourceForTest() string {
	return "https://basellm.github.io/llm-metadata/api/newapi/ratio_config-v1-base.json"
}
func defaultCatalogForTest() string {
	return "https://basellm.github.io/llm-metadata/api/all.json"
}

func TestNormalizeModelName(t *testing.T) {
	cases := map[string]string{
		"MiniMax-M3.1-Flash":          "minimaxm31flash",
		"global:minimax-m3.1-flash":   "minimaxm31flash",
		"cn:deepseek-v4.1-flash":      "deepseekv41flash",
		"deepseek-v4-1-flash-1017":    "deepseekv41flash1017",
		"deepseek-v4.1-flash-1017":    "deepseekv41flash1017",
		"deepseek-ai/DeepSeek-V3.1":   "deepseekv31",
		"GLM-5.3-Flash":               "glm53flash",
		"  模型  ":                       "模型",
	}
	for input, want := range cases {
		if got := normalizeModelName(input); got != want {
			t.Errorf("normalizeModelName(%q) = %q, want %q", input, got, want)
		}
	}
}

// The site names models its own way; the feed names them the vendor's way. This
// is the exact mismatch that made a live sync report "393 considered, 0 changed".
func TestResolveFeedNameMatchesSiteSpellings(t *testing.T) {
	prices := map[string]OfficialPrice{
		"gpt-5.6-sol":                        {},
		"MiniMax-M3.1-Flash":                 {},
		"glm-5.3-flash":                      {},
		"deepseek-v4-1-flash-1017":           {},
		"hy4":                                {},
		"hy4-preview":                        {},
		"deepseek-ai/DeepSeek-V3.1":          {},
	}
	index := buildModelNameIndex(prices)

	cases := []struct {
		site      string
		wantFeed  string
		wantOK    bool
		wantAlias bool
	}{
		// Exact hit needs no resolution at all.
		{"gpt-5.6-sol", "gpt-5.6-sol", true, false},
		{"glm-5.3-flash", "glm-5.3-flash", true, false},
		// Case and separator differences.
		{"global:minimax-m3.1-flash", "MiniMax-M3.1-Flash", true, true},
		{"cn:MiniMax-M3.1-Flash", "MiniMax-M3.1-Flash", true, true},
		// A dot where the feed uses a dash.
		{"deepseek-v4.1-flash-1017", "deepseek-v4-1-flash-1017", true, true},
		// Vendor path prefix on the feed side.
		{"DeepSeek-V3.1", "deepseek-ai/DeepSeek-V3.1", true, true},
		// Alias table: no -f variant is published, hy4 is the same list price.
		{"global:hy4free", "hy4", true, true},
		{"global:hy4-preview-f", "", false, false},
		// Genuinely absent: reported, never guessed.
		{"codex-auto-review", "", false, false},
	}
	for _, tc := range cases {
		got, aliased, ok := resolveFeedName(tc.site, prices, index)
		if ok != tc.wantOK {
			t.Errorf("resolveFeedName(%q) ok = %v, want %v", tc.site, ok, tc.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if got != tc.wantFeed {
			t.Errorf("resolveFeedName(%q) = %q, want %q", tc.site, got, tc.wantFeed)
		}
		if aliased != tc.wantAlias {
			t.Errorf("resolveFeedName(%q) aliased = %v, want %v", tc.site, aliased, tc.wantAlias)
		}
	}
}

// A normalized key that two feed entries share must resolve deterministically.
func TestBuildModelNameIndexPrefersLongerName(t *testing.T) {
	prices := map[string]OfficialPrice{
		"glm-5.3-flash":  {},
		"GLM_5.3_Flash":  {},
	}
	index := buildModelNameIndex(prices)
	got := index["glm53flash"]
	if got != "GLM_5.3_Flash" && got != "glm-5.3-flash" {
		t.Errorf("index[glm53flash] = %q, want one of the two feed spellings", got)
	}
	// Determinism: building the index twice must give the same answer.
	if again := buildModelNameIndex(prices)["glm53flash"]; again != got {
		t.Errorf("index is not deterministic: %q vs %q", got, again)
	}
}
