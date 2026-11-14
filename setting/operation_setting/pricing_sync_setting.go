package operation_setting

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// Official price synchronization.
//
// The upstream price list is denominated in US dollars: a billing expression's
// token coefficients are dollars per million tokens (see pkg/billingexpr). The
// console, however, may display quota in CNY or a custom currency, and a
// Chinese vendor's own price list is published in CNY. This setting records
// which of those lists an automatic sync should follow — never what unit is
// stored, because the stored unit is always USD.

const (
	PricingSyncCurrencyAuto = "auto" // follow the site's display currency
	PricingSyncCurrencyUSD  = "USD"
	PricingSyncCurrencyCNY  = "CNY"

	// PricingSyncScopeServed limits a run to the models this site actually
	// serves (plus any that already carry a configured price). PricingSyncScopeAll
	// imports every model the official list publishes.
	PricingSyncScopeServed = "served"
	PricingSyncScopeAll    = "all"

	// DefaultPricingSyncSource is the new-api ratio config that basellm publishes
	// from models.dev. Its coefficients are already dollars per million tokens.
	DefaultPricingSyncSource = "https://basellm.github.io/llm-metadata/api/newapi/ratio_config-v1-base.json"
	// DefaultPricingSyncCatalog carries per-model costs with an explicit
	// `currency` field. A sync uses it to (a) prove the ratio config really is
	// USD and (b) obtain the CNY list price for vendors that publish one.
	DefaultPricingSyncCatalog = "https://basellm.github.io/llm-metadata/api/all.json"

	DefaultPricingSyncIntervalHours = 24
	MinPricingSyncIntervalHours     = 1
	MaxPricingSyncIntervalHours     = 24 * 30
)

type PricingSyncSetting struct {
	Enabled       bool   `json:"enabled"`
	IntervalHours int    `json:"interval_hours"`
	Currency      string `json:"currency"`
	Scope         string `json:"scope"`
	SourceURL     string `json:"source_url"`
	CatalogURL    string `json:"catalog_url"`
}

var pricingSyncSetting = PricingSyncSetting{
	Enabled:       false,
	IntervalHours: DefaultPricingSyncIntervalHours,
	Currency:      PricingSyncCurrencyAuto,
	Scope:         PricingSyncScopeServed,
	SourceURL:     DefaultPricingSyncSource,
	CatalogURL:    DefaultPricingSyncCatalog,
}

func init() {
	config.GlobalConfig.Register("pricing_sync_setting", &pricingSyncSetting)
}

// GetPricingSyncSetting returns the live settings with every field normalized,
// so a hand-edited database row cannot put the scheduler into a bad state.
func GetPricingSyncSetting() PricingSyncSetting {
	setting := pricingSyncSetting
	switch setting.Currency {
	case PricingSyncCurrencyUSD, PricingSyncCurrencyCNY, PricingSyncCurrencyAuto:
	default:
		setting.Currency = PricingSyncCurrencyAuto
	}
	switch setting.Scope {
	case PricingSyncScopeServed, PricingSyncScopeAll:
	default:
		setting.Scope = PricingSyncScopeServed
	}
	if setting.IntervalHours < MinPricingSyncIntervalHours {
		setting.IntervalHours = DefaultPricingSyncIntervalHours
	}
	if setting.IntervalHours > MaxPricingSyncIntervalHours {
		setting.IntervalHours = MaxPricingSyncIntervalHours
	}
	if strings.TrimSpace(setting.SourceURL) == "" {
		setting.SourceURL = DefaultPricingSyncSource
	}
	if strings.TrimSpace(setting.CatalogURL) == "" {
		setting.CatalogURL = DefaultPricingSyncCatalog
	}
	return setting
}

// ResolvePricingSyncCurrency turns `auto` into the currency the site displays.
func ResolvePricingSyncCurrency(currency string) string {
	switch currency {
	case PricingSyncCurrencyUSD, PricingSyncCurrencyCNY:
		return currency
	}
	if GetGeneralSetting().QuotaDisplayType == QuotaDisplayTypeCNY {
		return PricingSyncCurrencyCNY
	}
	return PricingSyncCurrencyUSD
}

// ValidatePricingSyncOption rejects values the scheduler cannot use. Option
// writes reach the database before any code reads the field back, so this is
// the only place a typo can be caught.
func ValidatePricingSyncOption(key, value string) error {
	switch key {
	case "pricing_sync_setting.interval_hours":
		hours, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("sync interval must be a whole number of hours")
		}
		if hours < MinPricingSyncIntervalHours || hours > MaxPricingSyncIntervalHours {
			return fmt.Errorf("sync interval must be between %d and %d hours", MinPricingSyncIntervalHours, MaxPricingSyncIntervalHours)
		}
	case "pricing_sync_setting.currency":
		switch value {
		case PricingSyncCurrencyAuto, PricingSyncCurrencyUSD, PricingSyncCurrencyCNY:
		default:
			return fmt.Errorf("sync currency must be auto, USD or CNY")
		}
	case "pricing_sync_setting.scope":
		switch value {
		case PricingSyncScopeServed, PricingSyncScopeAll:
		default:
			return fmt.Errorf("sync scope must be served or all")
		}
	case "pricing_sync_setting.source_url", "pricing_sync_setting.catalog_url":
		if !strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "http://") {
			return fmt.Errorf("sync source must be an http(s) URL")
		}
	}
	return nil
}
