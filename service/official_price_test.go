package service

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
)

func floatPtr(v float64) *float64 { return &v }

// The published feed quotes dollars per 1M tokens. One dollar per 1M input
// tokens must land on exactly 0.5 quota per token, because QuotaPerUnit is
// 500000 — a figure that has been rendered as money without conversion before.
func TestCostOfOfficialPriceExpression(t *testing.T) {
	price := OfficialPrice{
		Mode:       billing_setting.BillingModeTieredExpr,
		Expression: `tier("standard", p * 2 + cr * 0.2 + c * 10)`,
	}
	tokens := OfficialTokens{Prompt: 1_000_000, Completion: 100_000, CacheRead: 200_000}

	quota, source, ok := CostOfOfficialPrice(price, tokens, 1)
	if !ok {
		t.Fatal("expected the price to be computable")
	}
	if source != "official" {
		t.Errorf("source = %q, want %q", source, "official")
	}

	// 800k fresh * $2 + 200k cache * $0.20 + 100k output * $10, per 1M tokens
	wantUSD := 800_000.0/1_000_000*2 + 200_000.0/1_000_000*0.2 + 100_000.0/1_000_000*10
	wantQuota := wantUSD * common.QuotaPerUnit
	if math.Abs(quota-wantQuota) > 1 {
		t.Errorf("quota = %.2f, want %.2f", quota, wantQuota)
	}
}

// The cache-write segment is part of prompt_tokens in the log, so it must be
// removed from the plain prompt count — otherwise it is billed twice.
func TestCostOfOfficialPriceDoesNotDoubleCountCacheWrite(t *testing.T) {
	price := OfficialPrice{
		Mode:       billing_setting.BillingModeTieredExpr,
		Expression: `tier("standard", p * 3 + cc * 3.75 + c * 15)`,
	}
	withWrite := OfficialTokens{Prompt: 1_000_000, CacheCreate: 400_000}
	quota, _, ok := CostOfOfficialPrice(price, withWrite, 1)
	if !ok {
		t.Fatal("expected a cost")
	}
	// 600k at $3 + 400k at $3.75 = 1.8 + 1.5 = $3.30
	want := 3.30 * common.QuotaPerUnit
	if math.Abs(quota-want) > 1 {
		t.Errorf("quota = %.2f, want %.2f (cache write billed twice?)", quota, want)
	}
}

func TestCostOfOfficialPriceRatioMode(t *testing.T) {
	price := OfficialPrice{
		Mode:            billing_setting.BillingModeRatio,
		ModelRatio:      floatPtr(2),
		CompletionRatio: floatPtr(4),
		CacheRatio:      floatPtr(0.1),
	}
	tokens := OfficialTokens{Prompt: 1000, CacheRead: 200, Completion: 500}
	quota, _, ok := CostOfOfficialPrice(price, tokens, 1)
	if !ok {
		t.Fatal("expected a cost")
	}
	// 800*2 + 200*2*0.1 + 500*2*4 = 1600 + 40 + 4000
	if math.Abs(quota-5640) > 0.001 {
		t.Errorf("quota = %.2f, want 5640", quota)
	}
}

// A per-request price is a different product from a per-token price; a cost
// basis must refuse rather than turn it into a per-token number.
func TestCostOfOfficialPriceRefusesPerRequestPrice(t *testing.T) {
	price := OfficialPrice{Mode: billing_setting.BillingModeRatio, ModelPrice: floatPtr(0.02)}
	if _, _, ok := CostOfOfficialPrice(price, OfficialTokens{Prompt: 1000}, 1); ok {
		t.Error("a per-request price must not be reported as a token cost")
	}
}

// A yuan-quoted feed is divided by the exchange rate before storage, so the
// cost basis must apply the same factor as the sync does.
func TestCostOfOfficialPriceAppliesCurrencyFactor(t *testing.T) {
	price := OfficialPrice{
		Mode:       billing_setting.BillingModeTieredExpr,
		Expression: `tier("standard", p * 7.3)`,
	}
	tokens := OfficialTokens{Prompt: 1_000_000}
	cny, _, ok := CostOfOfficialPrice(price, tokens, 1)
	if !ok {
		t.Fatal("expected a cost")
	}
	usd, _, ok := CostOfOfficialPrice(price, tokens, 1/7.3)
	if !ok {
		t.Fatal("expected a cost")
	}
	if math.Abs(cny/usd-7.3) > 0.01 {
		t.Errorf("factor ratio = %.4f, want 7.3", cny/usd)
	}
}

func TestOfficialTokensExcludesSubCategoriesFromPrompt(t *testing.T) {
	tokens := OfficialTokens{
		Prompt: 1000, CacheRead: 200, CacheCreate: 100, Image: 50, Audio: 25,
	}
	params := tokens.params()
	if params.P != 625 {
		t.Errorf("P = %.0f, want 625", params.P)
	}
	// The tier length keeps the whole input: tiers are chosen by context size.
	if params.Len != 1000 {
		t.Errorf("Len = %.0f, want 1000", params.Len)
	}
}

func TestOfficialTokensClampsNegativePrompt(t *testing.T) {
	// Some vendors report a cache read larger than the prompt; the plain prompt
	// count must not go negative and turn into a credit.
	tokens := OfficialTokens{Prompt: 100, CacheRead: 500}
	if params := tokens.params(); params.P != 0 {
		t.Errorf("P = %.0f, want 0", params.P)
	}
}
