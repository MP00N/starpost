package service

// The published list price, used as a cost basis.
//
// ## Why this exists
//
// Profit and reconciliation both ask "what should this request have cost". The
// answer that matters is not our own selling ratio — that is what we *charged*.
// It is what the vendor publishes, because that is what we pay to serve it.
// Before this file the only cost basis was a table an administrator had to fill
// in by hand, so a fresh install judged every model "cost unknown" and the page
// looked broken.
//
// ## Where the numbers come from
//
// The same feeds the price synchronization writes from (see pricing_sync.go):
// the new-api ratio config in dollars per 1M tokens. Nothing is assumed about
// the currency — a feed that turns out to be quoted in yuan is divided by the
// exchange rate before it is turned into quota.
//
// ## Why it is cached and not fetched per request
//
// Two feeds, several hundred models, and a page that redraws on every filter
// change. The book is fetched once, held for pricingSyncCacheTTL, and reused. A
// failed fetch is remembered briefly too, so a dead network cannot turn every
// page load into a 15-second stall.
//
// The zero value is not ready to use; call ResetOfficialPriceBook after changing
// the source URL in settings, or wait for the TTL to expire.

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
)

const (
	// pricingSyncCacheTTL is how long a fetched price book is trusted. Vendor
	// list prices change weekly at most; six hours keeps the cost basis fresh
	// without hammering a static CDN.
	pricingSyncCacheTTL = 6 * time.Hour
	// pricingSyncFailureTTL is deliberately much shorter than the success TTL:
	// a transient network failure should not pin an empty book for hours.
	pricingSyncFailureTTL = 5 * time.Minute
	// officialPriceLoadTimeout bounds the one blocking fetch. The feed is a
	// static file; anything slower is a network problem worth reporting.
	officialPriceLoadTimeout = 15 * time.Second
)

// OfficialTokens is the token mix of one request or one aggregated window,
// named the way a billing expression names its variables.
type OfficialTokens struct {
	// Prompt is the total input, cache reads included (that is how logs record it).
	Prompt float64
	// Completion is the output token count.
	Completion float64
	// CacheRead is the prefix that hit the cache.
	CacheRead float64
	// CacheCreate is the prefix written to the cache.
	CacheCreate float64
	// CacheCreate1h and CacheCreate12h are the TTL-split parts of CacheCreate,
	// which some vendors price differently.
	CacheCreate1h  float64
	CacheCreate12h float64
	// Image and Audio are input sub-categories priced separately by several
	// vendors; they are part of Prompt, not additional to it.
	Image float64
	Audio float64
}

// params reduces the mix to the expression environment. Sub-categories priced
// separately are removed from the plain prompt count so they are not billed
// twice; the tier length keeps the full input, because tiers are chosen by
// context size.
func (t OfficialTokens) params() billingexpr.TokenParams {
	prompt := t.Prompt - t.CacheRead - t.CacheCreate - t.Image - t.Audio
	if prompt < 0 {
		prompt = 0
	}
	return billingexpr.TokenParams{
		P:     prompt,
		C:     t.Completion,
		Len:   t.Prompt,
		CR:    t.CacheRead,
		CC:    t.CacheCreate,
		CC1h:  t.CacheCreate1h,
		CC12h: t.CacheCreate12h,
		Img:   t.Image,
		AI:    t.Audio,
	}
}

type officialPriceBook struct {
	mu        sync.Mutex
	prices    map[string]OfficialPrice
	// usdFactor converts a feed amount into USD. It is 1 for a dollar feed and
	// 1/exchangeRate for a feed quoted in yuan.
	usdFactor float64
	currency  string
	source    string
	loadedAt  time.Time
	attemptAt time.Time
	lastErr   error
}

var officialPrices officialPriceBook

// ResetOfficialPriceBook drops the cached book. Called when the source URL
// setting changes so the next reader picks up the new feed immediately.
func ResetOfficialPriceBook() {
	officialPrices.mu.Lock()
	defer officialPrices.mu.Unlock()
	officialPrices.prices = nil
	officialPrices.loadedAt = time.Time{}
	officialPrices.attemptAt = time.Time{}
	officialPrices.lastErr = nil
}

// OfficialPriceStatus describes the book for the pages that consume it, so a
// missing cost can be explained rather than silently shown as zero.
type OfficialPriceStatus struct {
	Models   int    `json:"models"`
	Currency string `json:"currency"`
	Source   string `json:"source"`
	LoadedAt int64  `json:"loaded_at"`
	Error    string `json:"error,omitempty"`
}

// GetOfficialPriceStatus reports what the book currently holds without
// triggering a fetch.
func GetOfficialPriceStatus() OfficialPriceStatus {
	officialPrices.mu.Lock()
	defer officialPrices.mu.Unlock()
	status := OfficialPriceStatus{
		Models:   len(officialPrices.prices),
		Currency: officialPrices.currency,
		Source:   shortSourceLabel(officialPrices.source),
		LoadedAt: officialPrices.loadedAt.Unix(),
	}
	if officialPrices.lastErr != nil {
		status.Error = officialPrices.lastErr.Error()
	}
	return status
}

// load refreshes the book when it is stale. The caller must not hold the lock.
func (b *officialPriceBook) load() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	ttl := pricingSyncCacheTTL
	if b.lastErr != nil {
		ttl = pricingSyncFailureTTL
	}
	if !b.attemptAt.IsZero() && now.Sub(b.attemptAt) < ttl {
		return
	}
	b.attemptAt = now

	setting := operation_setting.GetPricingSyncSetting()
	ctx, cancel := context.WithTimeout(context.Background(), officialPriceLoadTimeout)
	defer cancel()

	prices, err := FetchOfficialPricing(ctx, setting.SourceURL)
	if err != nil || len(prices) == 0 {
		if err == nil {
			err = errEmptyPriceFeed
		}
		b.lastErr = err
		b.attemptAt = now
		// Keep whatever was loaded before: a stale list price is a far better
		// cost basis than none at all.
		return
	}

	// Measure the feed's currency instead of trusting a comment. The catalog
	// costs a second download, so it is fetched only when it can change the
	// answer — a feed that already reads as dollars needs no conversion.
	usdFactor := 1.0
	currency := operation_setting.PricingSyncCurrencyUSD
	if catalog, catalogErr := FetchPricingSyncCatalog(ctx, setting.CatalogURL); catalogErr == nil {
		exchangeRate := operation_setting.USDExchangeRate
		if exchangeRate <= 0 {
			exchangeRate = 7.3
		}
		usdVotes, cnyVotes, _, _ := detectFeedCurrency(prices, catalog, exchangeRate)
		if cnyVotes > usdVotes {
			currency = operation_setting.PricingSyncCurrencyCNY
			usdFactor = 1 / exchangeRate
		}
	} else {
		logger.LogWarn(ctx, "official price book: catalog unavailable, assuming the feed is denominated in USD: %v", catalogErr)
	}

	b.prices = prices
	b.usdFactor = usdFactor
	b.currency = currency
	b.source = setting.SourceURL
	b.loadedAt = now
	b.lastErr = nil
}

var errEmptyPriceFeed = errOfficialPrice("the price feed contained no prices")

type errOfficialPrice string

func (e errOfficialPrice) Error() string { return string(e) }

// OfficialCostQuota returns what the published list price says the given token
// mix costs, in quota units, plus a short source label for the UI.
//
// The second return value is false when the feed has no price for the model —
// the caller must report "unknown" rather than a zero cost.
func OfficialCostQuota(modelName string, tokens OfficialTokens) (float64, string, bool) {
	officialPrices.load()

	officialPrices.mu.Lock()
	price, ok := officialPrices.prices[modelName]
	factor := officialPrices.usdFactor
	officialPrices.mu.Unlock()
	if !ok {
		return 0, "", false
	}
	return CostOfOfficialPrice(price, tokens, factor)
}

// CostOfOfficialPrice is the pure part of OfficialCostQuota: one published
// price applied to one token mix. Split out so it can be tested without a
// network fetch or the global cache.
func CostOfOfficialPrice(price OfficialPrice, tokens OfficialTokens, usdFactor float64) (float64, string, bool) {
	if usdFactor <= 0 {
		usdFactor = 1
	}
	params := tokens.params()
	if price.Mode == billing_setting.BillingModeTieredExpr {
		// The expression's own output is dollars × 1e6 (coefficients are per 1M
		// tokens), which is exactly the unit quotaConversion expects.
		cost, _, err := billingexpr.RunExpr(price.Expression, params)
		if err != nil || cost < 0 || math.IsInf(cost, 0) || math.IsNaN(cost) {
			return 0, "", false
		}
		return cost / 1_000_000 * common.QuotaPerUnit * usdFactor, "official", true
	}

	if price.ModelPrice != nil {
		// A per-request price does not depend on the token mix, so it cannot be
		// charged per token. Refuse instead of inventing a number.
		return 0, "", false
	}
	if price.ModelRatio == nil || *price.ModelRatio <= 0 {
		return 0, "", false
	}

	// Ratio mode uses the same unit as the local ratio table: 1 = $2 per 1M
	// tokens, which is one quota unit per token.
	ratio := *price.ModelRatio
	quota := params.P * ratio
	if price.CacheRatio != nil {
		quota += params.CR * ratio * *price.CacheRatio
	} else {
		quota += params.CR * ratio
	}
	if price.CreateCacheRatio != nil {
		quota += params.CC * ratio * *price.CreateCacheRatio
	} else {
		quota += params.CC * ratio
	}
	if params.CC1h > 0 || params.CC12h > 0 {
		// Split TTL writes have no published separate ratio in this feed; charging
		// them at the base write price understates cost slightly, which is the
		// safe direction for a loss guard.
		quota += (params.CC1h + params.CC12h) * ratio
	}
	if price.CompletionRatio != nil {
		quota += params.C * ratio * *price.CompletionRatio
	} else {
		quota += params.C * ratio
	}
	return quota * usdFactor, "official", true
}

// shortSourceLabel trims a URL to host+path for display.
func shortSourceLabel(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	source = strings.TrimPrefix(source, "https://")
	source = strings.TrimPrefix(source, "http://")
	if index := strings.Index(source, "/"); index > 0 {
		// Keep the host only: the path is long and carries no extra meaning.
		return source[:index]
	}
	return source
}
