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
