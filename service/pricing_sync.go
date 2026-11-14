package service

// Official price synchronization.
//
// ## What is being synchronized
//
// new-api stores token prices as billing-expression coefficients in dollars per
// million tokens (`p * 2.5` = $2.50 per 1M prompt tokens — see pkg/billingexpr).
// basellm publishes a machine-readable new-api ratio config generated from
// models.dev, so an administrator can adopt the vendor's current list price
// instead of maintaining ratios by hand.
//
// ## Why the currency needs proving, not assuming
//
// The requirement is that a price written as USD really is USD. Rather than
// trust a comment, every run measures the feed: the models.dev catalog marks
// each model's list currency, and for the models present in both feeds the
// ratio of the feed coefficient to the declared price is either ~1 (same
// currency) or ~the exchange rate (converted). A majority vote over dozens of
// models settles it, and a feed that looks like yuan is converted before
// anything is written — or refused outright when it cannot be converted safely.
//
// Storage is always USD-denominated. A published CNY list price is stored as
// `cny / exchangeRate`, so the console — which multiplies by that same rate
// when it renders CNY — shows the vendor's own number rather than a derived one.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	pricingSyncTimeout        = 45 * time.Second
	pricingSyncMaxBytes       = 32 << 20
	pricingSyncAttempts       = 3
	pricingSyncMinVotes       = 3
	pricingSyncVoteSlack      = 0.08
	pricingSyncUserAgent      = "StarPost-PricingSync/1.0"
	pricingSyncMaxReportItems = 50
)

// The published feed and the local option table name the same two fields
// differently: the feed uses the wire names `billing_mode` / `billing_expr`,
// while the storage layer keys them as `billing_setting.billing_mode` /
// `billing_setting.billing_expr` (see model.modelPricingOptionKeys). Writing the
// wire names into a PricingValues draft fails validation with
// "unsupported pricing field", so the two must not be confused.
const (
	pricingModeKey = "billing_setting.billing_mode"
	pricingExprKey = "billing_setting.billing_expr"
)

// ErrPricingSyncBusy reports that another synchronization is already running.
var ErrPricingSyncBusy = errors.New("a price synchronization is already running")

// PricingSyncResult is what one run leaves in the task row: the numbers an
// administrator needs to judge the run without opening a log file.
type PricingSyncResult struct {
	Currency         string  `json:"currency"`
	Scope            string  `json:"scope"`
	Source           string  `json:"source"`
	FeedCurrency     string  `json:"feed_currency"`
	CurrencyVerified bool    `json:"currency_verified"`
	USDVotes         int     `json:"usd_votes"`
	CNYVotes         int     `json:"cny_votes"`
	OfficialModels   int     `json:"official_models"`
	Considered       int     `json:"considered"`
	Updated          int     `json:"updated"`
	Unchanged        int     `json:"unchanged"`
	Skipped          int     `json:"skipped"`
	Failed           int     `json:"failed"`
	ExchangeRate     float64 `json:"exchange_rate"`
	PublishedCNYUsed int     `json:"published_cny_used"`
	// Unclassified lists up to five models the two feeds price differently.
	// They are excluded from the currency vote, not from the sync.
	Unclassified  []string `json:"unclassified,omitempty"`
	SkippedModels []string `json:"skipped_models,omitempty"`
	FailedModels  []string `json:"failed_models,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	FinishedAt    int64    `json:"finished_at"`
}

// OfficialPrice is one model's price as published upstream, in the feed's own
// currency and unit (per 1M tokens, or a fixed per-request price).
type OfficialPrice struct {
	Mode             string
	Expression       string
	ModelRatio       *float64
	CompletionRatio  *float64
	CacheRatio       *float64
	CreateCacheRatio *float64
	ModelPrice       *float64
}

type catalogCost struct {
	Currency        string                 `json:"currency"`
	Input           *float64               `json:"input"`
	Output          *float64               `json:"output"`
	CacheRead       *float64               `json:"cache_read"`
	CacheWrite      *float64               `json:"cache_write"`
	CurrencyOptions map[string]catalogCost `json:"currency_options"`
}

type catalogProvider struct {
	Models map[string]struct {
		Cost catalogCost `json:"cost"`
	} `json:"models"`
}

// pricingCatalog is the per-model cost catalog split by declared currency.
// Keeping the two apart is the whole point: a CNY figure must never be
// compared against — or written as — a USD one.
type pricingCatalog struct {
	// CNY holds the cheapest yuan-listed cost per model name.
	CNY map[string]catalogCost
	// USD holds the cheapest dollar-listed cost per model name.
	USD map[string]catalogCost
}

func (c pricingCatalog) empty() bool { return len(c.CNY) == 0 && len(c.USD) == 0 }

// FetchPricingSyncCatalog downloads the per-model cost catalog, split by the
// currency each entry is actually denominated in.
//
// models.dev documents `cost` as USD per 1M tokens and sets `currency` only to
// mark an exception, so a missing `currency` means USD. That default is not
// taken on trust downstream: detectFeedCurrency still has to confirm it by
// comparing the two feeds model by model, and a catalog that disagreed with the
// feed would flip the vote rather than be silently believed.
func FetchPricingSyncCatalog(ctx context.Context, url string) (pricingCatalog, error) {
	body, err := fetchJSONBytes(ctx, url)
	if err != nil {
		return pricingCatalog{}, err
	}
	var providers map[string]catalogProvider
	if err := common.Unmarshal(body, &providers); err != nil {
		return pricingCatalog{}, fmt.Errorf("decode catalog: %w", err)
	}
	if len(providers) == 0 {
		return pricingCatalog{}, errors.New("catalog is empty")
	}
	catalog := pricingCatalog{CNY: make(map[string]catalogCost), USD: make(map[string]catalogCost)}
	for _, provider := range providers {
		for name, entry := range provider.Models {
			cost := entry.Cost
			currency := cost.Currency
			if currency == "" {
				currency = "USD"
			}
			switch {
			case strings.EqualFold(currency, "USD"):
				if usableCost(cost) {
					keepCheapest(catalog.USD, name, cost)
				}
			case strings.EqualFold(currency, "CNY"):
				if usableCost(cost) {
					keepCheapest(catalog.CNY, name, cost)
				}
			}
			// A vendor may publish a second list in another currency; those are
			// genuine published prices, not conversions, so they join the CNY
			// pool.
			for optionCurrency, option := range cost.CurrencyOptions {
				if !strings.EqualFold(optionCurrency, "CNY") || !usableCost(option) {
					continue
				}
				option.Currency = "CNY"
				keepCheapest(catalog.CNY, name, option)
			}
		}
	}
	return catalog, nil
}

func usableCost(cost catalogCost) bool {
	if cost.Input == nil {
		return false
	}
	return *cost.Input > 0 && !math.IsNaN(*cost.Input) && !math.IsInf(*cost.Input, 0)
}

func keepCheapest(target map[string]catalogCost, name string, cost catalogCost) {
	current, exists := target[name]
	if !exists || *cost.Input < *current.Input {
		target[name] = cost
	}
}

// FetchOfficialPricing downloads the ratio config and normalizes it. The
// returned prices keep the feed's own unit; currency is handled by the caller
// once the feed's currency has been measured.
func FetchOfficialPricing(ctx context.Context, url string) (map[string]OfficialPrice, error) {
	body, err := fetchJSONBytes(ctx, url)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode ratio config: %w", err)
	}
	if !envelope.Success {
		return nil, fmt.Errorf("upstream refused the request: %s", envelope.Message)
	}
	var data map[string]any
	if err := common.Unmarshal(envelope.Data, &data); err != nil {
		return nil, fmt.Errorf("decode ratio config data: %w", err)
	}

	prices := make(map[string]OfficialPrice)
	modes := valueMapOf(data[billing_setting.BillingModeField])
	for name, raw := range valueMapOf(data[billing_setting.BillingExprField]) {
		expression, ok := raw.(string)
		if !ok || strings.TrimSpace(expression) == "" {
			continue
		}
		if modes[name] != billing_setting.BillingModeTieredExpr {
			continue
		}
		prices[name] = OfficialPrice{Mode: billing_setting.BillingModeTieredExpr, Expression: strings.TrimSpace(expression)}
	}
	ratioFields := map[string]func(*OfficialPrice, float64){
		"model_ratio":        func(p *OfficialPrice, v float64) { p.ModelRatio = &v },
		"completion_ratio":   func(p *OfficialPrice, v float64) { p.CompletionRatio = &v },
		"cache_ratio":        func(p *OfficialPrice, v float64) { p.CacheRatio = &v },
		"create_cache_ratio": func(p *OfficialPrice, v float64) { p.CreateCacheRatio = &v },
		"model_price":        func(p *OfficialPrice, v float64) { p.ModelPrice = &v },
	}
	for field, assign := range ratioFields {
		for name, raw := range valueMapOf(data[field]) {
			number, ok := toFloat(raw)
			if !ok || number < 0 || math.IsNaN(number) || math.IsInf(number, 0) {
				continue
			}
			price := prices[name]
			if price.Mode == billing_setting.BillingModeTieredExpr {
				// An expression wins: the ratio fields beside it are not what
				// the upstream bills with.
				continue
			}
			price.Mode = billing_setting.BillingModeRatio
			assign(&price, number)
			prices[name] = price
		}
	}
	return prices, nil
}

// detectFeedCurrency measures which currency the feed's coefficients are in.
//
// For every model present in both the feed and the catalog, the ratio of the
// feed's prompt price to the catalog's declared price is ~1 when the two quote
// the same currency, ~1/rate when the catalog quotes yuan and the feed quotes
// dollars, and ~rate when the catalog quotes dollars and the feed quotes yuan.
// Each comparison is one vote; a feed nobody can vote on is reported as
// unverified rather than assumed.
func detectFeedCurrency(prices map[string]OfficialPrice, catalog pricingCatalog, exchangeRate float64) (usdVotes, cnyVotes int, unclassified int, examples []string) {
	if exchangeRate <= 0 {
		exchangeRate = 7.3
	}
	addExample := func(text string) {
		unclassified++
		if len(examples) < 5 {
			examples = append(examples, text)
		}
	}
	near := func(a, b float64) bool {
		if b == 0 {
			return false
		}
		return math.Abs(a-b)/math.Abs(b) <= pricingSyncVoteSlack
	}
	compare := func(name string, coefficient float64, declared float64, declaredCurrency string) {
		if declared <= 0 || coefficient <= 0 {
			return
		}
		ratio := coefficient / declared
		switch declaredCurrency {
		case "USD":
			switch {
			case near(ratio, 1):
				usdVotes++
			case near(ratio, exchangeRate):
				cnyVotes++
			default:
				addExample(fmt.Sprintf("%s: feed %.6g vs USD list %.6g", name, coefficient, declared))
			}
		case "CNY":
			switch {
			case near(ratio, 1):
				cnyVotes++
			case near(ratio, 1/exchangeRate):
				usdVotes++
			default:
				addExample(fmt.Sprintf("%s: feed %.6g vs CNY list %.6g", name, coefficient, declared))
			}
		}
	}
	names := make([]string, 0, len(prices))
	for name := range prices {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		price := prices[name]
		if price.Mode != billing_setting.BillingModeTieredExpr {
			continue
		}
		coefficient, ok := simplePromptCoefficient(price.Expression)
		if !ok {
			continue
		}
		// A model listed in both currencies is compared against its dollar
		// list, which is what the aggregate feed normally carries.
		if cost, exists := catalog.USD[name]; exists && cost.Input != nil {
			compare(name, coefficient, *cost.Input, "USD")
			continue
		}
		if cost, exists := catalog.CNY[name]; exists && cost.Input != nil {
			compare(name, coefficient, *cost.Input, "CNY")
		}
	}
	return usdVotes, cnyVotes, unclassified, examples
}

// simplePromptCoefficient reads the prompt price out of an unconditional
// `tier("...", p * X ...)` expression. Anything conditional or request-priced
// is skipped: its first literal is a branch price, not the list price.
func simplePromptCoefficient(expression string) (float64, bool) {
	body := strings.TrimPrefix(expression, "v1:")
	if !strings.HasPrefix(body, `tier("`) {
		return 0, false
	}
	closing := strings.Index(body, `", `)
	if closing < 0 {
		return 0, false
	}
	inner := strings.TrimSuffix(strings.TrimSpace(body[closing+3:]), ")")
	if strings.ContainsAny(inner, "?:") || strings.Contains(inner, "param(") || strings.Contains(inner, "fixed(") {
		return 0, false
	}
	for _, part := range strings.Split(inner, "+") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "p * ") {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(part, "p * ")), 64)
		if err != nil {
			return 0, false
		}
		return value, true
	}
	return 0, false
}

// SynthesizeTokenExpression builds a billing expression from a published CNY
// list price. Values arrive in CNY per 1M tokens and are stored divided by the
// exchange rate, so the console's own CNY rendering reproduces the vendor's
// number instead of a rounded conversion of it.
func SynthesizeTokenExpression(cost catalogCost, exchangeRate float64) (string, bool) {
	if cost.Input == nil || exchangeRate <= 0 {
		return "", false
	}
	input := *cost.Input / exchangeRate
	if input <= 0 || math.IsNaN(input) || math.IsInf(input, 0) {
		return "", false
	}
	terms := []string{"p * " + formatCoefficient(input)}
	if cost.CacheRead != nil && *cost.CacheRead > 0 {
		terms = append(terms, "cr * "+formatCoefficient(*cost.CacheRead/exchangeRate))
	}
	if cost.CacheWrite != nil && *cost.CacheWrite > 0 {
		terms = append(terms, "cc * "+formatCoefficient(*cost.CacheWrite/exchangeRate))
	}
	if cost.Output != nil && *cost.Output > 0 {
		terms = append(terms, "c * "+formatCoefficient(*cost.Output/exchangeRate))
	}
	return `tier("standard", ` + strings.Join(terms, " + ") + ")", true
}

func formatCoefficient(value float64) string {
	rounded := math.Round(value*1e6) / 1e6
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}

// pricingSyncMu serializes runs. Two overlapping runs would both read the same
// snapshot and the second would fail its version check, so a manual "sync now"
// while the scheduler is running waits instead of erroring.
var pricingSyncMu sync.Mutex

// RunPricingSyncExclusive refuses to queue: a second caller returns immediately
// with ErrPricingSyncBusy rather than blocking a request handler behind a slow
// run.
func RunPricingSyncExclusive(ctx context.Context, dryRun bool, progress func(processed, total int)) (*PricingSyncResult, error) {
	if !pricingSyncMu.TryLock() {
		return nil, ErrPricingSyncBusy
	}
	defer pricingSyncMu.Unlock()
	return RunPricingSync(ctx, dryRun, progress)
}

// RunPricingSync performs one synchronization. progress may be nil.
//
// It never writes a model whose official price it could not parse, and it
// always writes through model.UpdateModelPricing so every existing validation
// (expression smoke tests, plugin schemas, version conflict detection) applies
// exactly as it does for a hand edit.
//
// With dryRun set it stops before the write: `Updated` then counts the models a
// real run would change. The preview endpoint uses that, so an administrator can
// see the blast radius without touching billing data.
func RunPricingSync(ctx context.Context, dryRun bool, progress func(processed, total int)) (*PricingSyncResult, error) {
	setting := operation_setting.GetPricingSyncSetting()
	currency := operation_setting.ResolvePricingSyncCurrency(setting.Currency)
	exchangeRate := operation_setting.USDExchangeRate
	if exchangeRate <= 0 {
		exchangeRate = 7.3
	}

	result := &PricingSyncResult{
		Currency:     currency,
		Scope:        setting.Scope,
		Source:       setting.SourceURL,
		ExchangeRate: exchangeRate,
	}

	catalog, catalogErr := FetchPricingSyncCatalog(ctx, setting.CatalogURL)
	if catalogErr != nil {
		result.Warnings = append(result.Warnings, "cost catalog unavailable, the feed's currency could not be measured: "+catalogErr.Error())
	}

	prices, err := fetchOfficialPricingWithFallback(ctx, setting.SourceURL, result)
	if err != nil {
		return result, err
	}
	result.OfficialModels = len(prices)

	usdVotes, cnyVotes, unclassified, examples := detectFeedCurrency(prices, catalog, exchangeRate)
	result.USDVotes, result.CNYVotes = usdVotes, cnyVotes
	switch {
	case catalog.empty():
		result.FeedCurrency = operation_setting.PricingSyncCurrencyUSD
		result.Warnings = append(result.Warnings, "no catalog entries were available to measure the feed's currency; treating it as USD as documented")
	case usdVotes+cnyVotes < pricingSyncMinVotes:
		result.FeedCurrency = operation_setting.PricingSyncCurrencyUSD
		result.Warnings = append(result.Warnings, "too few comparable models to measure the feed's currency; treating it as USD as documented")
	case cnyVotes > usdVotes:
		result.FeedCurrency = operation_setting.PricingSyncCurrencyCNY
		result.Warnings = append(result.Warnings, fmt.Sprintf("the price feed is published in CNY (%d of %d measured models agree); amounts are converted to USD before storage", cnyVotes, usdVotes+cnyVotes))
	default:
		result.FeedCurrency = operation_setting.PricingSyncCurrencyUSD
		result.CurrencyVerified = true
	}
	// A handful of models are priced differently by the two feeds — usually a
	// regional variant or a vendor that renegotiated. That is a fact about the
	// source, not a fault in this run, and it does not affect the currency
	// verdict (hundreds of other models settle that). Report the count and the
	// first few names once, instead of one warning line each.
	if unclassified > 0 {
		result.Unclassified = examples
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%d models are priced differently by the two feeds and were left out of the currency check (e.g. %s)",
			unclassified, strings.Join(examples[:min(2, len(examples))], "; ")))
	}

	feedIsCNY := result.FeedCurrency == operation_setting.PricingSyncCurrencyCNY
	// cnyFactor converts a feed amount into the stored USD-denominated amount.
	cnyFactor := 1.0
	if feedIsCNY {
		cnyFactor = 1 / exchangeRate
	}

	names := make([]string, 0, len(prices))
	for name := range prices {
		names = append(names, name)
	}
	sort.Strings(names)

	served := make(map[string]struct{})
	if setting.Scope == operation_setting.PricingSyncScopeServed {
		for _, name := range model.GetEnabledModels() {
			served[name] = struct{}{}
		}
	}

	// The snapshot supplies both the current version (required by
	// UpdateModelPricing's optimistic check) and the configured values, so an
	// unchanged price is skipped instead of rewritten.
	snapshot, err := model.GetModelPricingSnapshot(nil)
	if err != nil {
		return result, fmt.Errorf("read current pricing: %w", err)
	}
	current := make(map[string]model.ModelPricingEntry, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		current[entry.ModelName] = entry
	}

	changes := make([]model.ModelPricingChange, 0, len(names))
	for _, name := range names {
		if setting.Scope == operation_setting.PricingSyncScopeServed {
			_, isServed := served[name]
			_, hasPrice := current[name]
			if !isServed && !hasPrice {
				continue
			}
		}
		result.Considered++

		pricing, usedCNY, ok := buildPricingValues(name, prices[name], catalog, currency, exchangeRate, cnyFactor)
		if !ok {
			result.Skipped++
			if len(result.SkippedModels) < pricingSyncMaxReportItems {
				result.SkippedModels = append(result.SkippedModels, name)
			}
			continue
		}
		if usedCNY {
			result.PublishedCNYUsed++
		}

		entry, exists := current[name]
		if exists && pricingValuesMatch(entry.Configured, pricing) {
			result.Unchanged++
			continue
		}
		expectedVersion := snapshot.EmptyVersion
		if exists {
			expectedVersion = entry.Version
		}
		changes = append(changes, model.ModelPricingChange{
			ModelName:       name,
			ExpectedVersion: expectedVersion,
			Pricing:         pricing,
		})
	}

	if len(changes) == 0 {
		result.FinishedAt = common.GetTimestamp()
		return result, nil
	}

	if dryRun {
		result.Updated = len(changes)
		result.FinishedAt = common.GetTimestamp()
		return result, nil
	}

	if progress != nil {
		progress(0, len(changes))
	}
	// The batch is one transaction, so a single model that fails validation
	// would roll back every other model with it — one unparseable upstream
	// expression would silently stop the whole sync. Bisect on failure: a batch
	// that cannot commit is split until only the offending models remain, and
	// those are reported individually. The common case (everything valid) still
	// costs exactly one transaction.
	writePricingChanges(ctx, changes, result, 0)
	if progress != nil {
		progress(len(changes), len(changes))
	}

	if result.Failed > 0 && result.Updated == 0 {
		return result, fmt.Errorf("no model could be updated: %s", strings.Join(result.FailedModels, ", "))
	}

	result.FinishedAt = common.GetTimestamp()
	logger.LogInfo(ctx, common.LogText("pricing sync finished: currency=%s feed=%s updated=%d unchanged=%d skipped=%d failed=%d official=%d",
		result.Currency, result.FeedCurrency, result.Updated, result.Unchanged, result.Skipped, result.Failed, result.OfficialModels))
	return result, nil
}

// writePricingChanges commits changes, bisecting on any non-conflict error so a
// single bad model cannot discard the rest of the batch.
func writePricingChanges(ctx context.Context, changes []model.ModelPricingChange, result *PricingSyncResult, depth int) {
	if len(changes) == 0 {
		return
	}
	if err := commitPricingChanges(ctx, changes); err == nil {
		result.Updated += len(changes)
		return
	} else if len(changes) == 1 && len(result.Warnings) < pricingSyncMaxReportItems {
		// Keep the validator's own words: "failed" alone tells an administrator
		// nothing about which field the upstream feed got wrong.
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", changes[0].ModelName, err))
	}
	if len(changes) == 1 {
		result.Failed++
		if len(result.FailedModels) < pricingSyncMaxReportItems {
			result.FailedModels = append(result.FailedModels, changes[0].ModelName)
		}
		return
	}
	if depth >= 12 {
		// Bisection depth is bounded so a pathological feed cannot turn this
		// into thousands of transactions; whatever is left is reported as failed.
		result.Failed += len(changes)
		for _, change := range changes {
			if len(result.FailedModels) < pricingSyncMaxReportItems {
				result.FailedModels = append(result.FailedModels, change.ModelName)
			}
		}
		return
	}
	middle := len(changes) / 2
	writePricingChanges(ctx, changes[:middle], result, depth+1)
	writePricingChanges(ctx, changes[middle:], result, depth+1)
}

// commitPricingChanges retries once against a fresh snapshot when another
// writer touched one of the models in the meantime.
func commitPricingChanges(ctx context.Context, changes []model.ModelPricingChange) error {
	err := model.UpdateModelPricing(changes)
	if err == nil || !errors.Is(err, model.ErrModelPricingConflict) {
		return err
	}
	refreshed, refreshErr := model.GetModelPricingSnapshot(nil)
	if refreshErr != nil {
		return refreshErr
	}
	versions := make(map[string]string, len(refreshed.Entries))
	for _, entry := range refreshed.Entries {
		versions[entry.ModelName] = entry.Version
	}
	for index := range changes {
		if version, ok := versions[changes[index].ModelName]; ok {
			changes[index].ExpectedVersion = version
		} else {
			changes[index].ExpectedVersion = refreshed.EmptyVersion
		}
	}
	return model.UpdateModelPricing(changes)
}

func fetchOfficialPricingWithFallback(ctx context.Context, source string, result *PricingSyncResult) (map[string]OfficialPrice, error) {
	sources := []string{source}
	if source != operation_setting.DefaultPricingSyncSource {
		sources = append(sources, operation_setting.DefaultPricingSyncSource)
	}
	var lastErr error
	for _, candidate := range sources {
		prices, err := FetchOfficialPricing(ctx, candidate)
		if err == nil && len(prices) > 0 {
			result.Source = candidate
			return prices, nil
		}
		if err == nil {
			err = errors.New("the price feed contained no prices")
		}
		lastErr = err
		if len(sources) > 1 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s failed: %v", candidate, err))
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no price source configured")
	}
	return nil, lastErr
}

// buildPricingValues turns one official price into the stored draft.
//
// cnyFactor scales a feed amount into USD storage. When the administrator asked
// for CNY, a vendor-published yuan list price takes precedence over any
// conversion, because it is the vendor's own number rather than somebody else's
// arithmetic.
func buildPricingValues(name string, price OfficialPrice, catalog pricingCatalog, currency string, exchangeRate, cnyFactor float64) (model.PricingValues, bool, bool) {
	// A published yuan list is only comparable when the official price is
	// token-priced. The catalog quotes input/output per 1M tokens, so using it
	// for a per-request or per-image model would replace a request price with a
	// token price — a different product, not a different currency.
	if currency == operation_setting.PricingSyncCurrencyCNY && isTokenPriced(price) {
		if cost, ok := catalog.CNY[name]; ok && cost.Input != nil && *cost.Input > 0 {
			if expression, built := SynthesizeTokenExpression(cost, exchangeRate); built {
				return model.PricingValues{
					pricingModeKey: billing_setting.BillingModeTieredExpr,
					pricingExprKey: expression,
				}, true, true
			}
		}
		// No published yuan list: the USD figure stays and the console derives
		// the yuan amount from it, which is the honest thing to display.
	}

	if price.Mode == billing_setting.BillingModeTieredExpr {
		if cnyFactor != 1 {
			// A yuan-denominated expression cannot be rescaled safely by string
			// surgery — every coefficient would need rewriting. Refuse rather
			// than guess.
			return nil, false, false
		}
		return model.PricingValues{
			pricingModeKey: billing_setting.BillingModeTieredExpr,
			pricingExprKey: price.Expression,
		}, false, true
	}

	if price.ModelPrice != nil {
		return model.PricingValues{
			"ModelPrice":   *price.ModelPrice * cnyFactor,
			pricingModeKey: billing_setting.BillingModeRatio,
		}, false, true
	}

	if price.ModelRatio == nil || *price.ModelRatio <= 0 {
		return nil, false, false
	}
	values := model.PricingValues{
		"ModelRatio":   *price.ModelRatio * cnyFactor,
		pricingModeKey: billing_setting.BillingModeRatio,
	}
	if price.CompletionRatio != nil {
		values["CompletionRatio"] = *price.CompletionRatio
	}
	if price.CacheRatio != nil {
		values["CacheRatio"] = *price.CacheRatio * cnyFactor
	}
	if price.CreateCacheRatio != nil {
		values["CreateCacheRatio"] = *price.CreateCacheRatio * cnyFactor
	}
	return values, false, true
}

// isTokenPriced reports whether the official price charges per token rather
// than per request or per image. A per-request price (`fixed(...)`) or an
// image-count multiplier is a different unit, and no currency conversion can
// turn one into the other.
func isTokenPriced(price OfficialPrice) bool {
	switch price.Mode {
	case billing_setting.BillingModeTieredExpr:
		return strings.Contains(price.Expression, "p * ")
	case billing_setting.BillingModeRatio:
		return price.ModelPrice == nil && price.ModelRatio != nil
	default:
		return false
	}
}

// pricingValuesMatch reports whether every value in draft is already configured.
// Extra configured keys are allowed: defaults and plugin overrides legitimately
// sit beside the fields a sync owns.
func pricingValuesMatch(configured, draft model.PricingValues) bool {
	for key, value := range draft {
		current, exists := configured[key]
		if !exists || !pricingValueEqual(current, value) {
			return false
		}
	}
	return true
}

func pricingValueEqual(a, b any) bool {
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		return math.Abs(af-bf) <= 1e-9
	}
	return a == b
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func valueMapOf(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

// fetchJSONBytes downloads a feed with retries. The retry matters: these are
// static files on a CDN and a single 5xx must not fail an unattended run.
func fetchJSONBytes(ctx context.Context, rawURL string) ([]byte, error) {
	transport := &http.Transport{
		MaxIdleConns:          4,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig.Clone()
	}
	client := &http.Client{Transport: transport}

	var lastErr error
	for attempt := 0; attempt < pricingSyncAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(300*(1<<attempt)) * time.Millisecond):
			}
		}
		requestCtx, cancel := context.WithTimeout(ctx, pricingSyncTimeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, rawURL, nil)
		if err != nil {
			cancel()
			return nil, err
		}
		req.Header.Set("User-Agent", pricingSyncUserAgent)
		req.Header.Set("Accept", "application/json")
		// A sync must never adopt a CDN's stale copy of the list.
		req.Header.Set("Cache-Control", "no-cache")

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			cancel()
			lastErr = fmt.Errorf("%s returned %s", rawURL, resp.Status)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, pricingSyncMaxBytes))
		resp.Body.Close()
		cancel()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		return body, nil
	}
	return nil, lastErr
}
