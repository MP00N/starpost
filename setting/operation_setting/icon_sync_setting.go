package operation_setting

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// Official icon synchronization.
//
// A model or vendor record carries an `icon` field holding a @lobehub/icons
// name (e.g. "Claude.Color"). The upstream metadata feed publishes that name
// for most models, but nothing fills the field on a fresh install, so the
// console falls back to the first letter of the model name — which is why a
// new deployment shows letters where the design calls for brand marks.
//
// This setting drives a scheduled job that copies those names in. It is
// deliberately conservative about overwriting: an administrator who picked a
// different icon by hand should not have it replaced on the next tick.

const (
	DefaultIconSyncIntervalHours = 24
	MinIconSyncIntervalHours     = 1
	MaxIconSyncIntervalHours     = 24 * 30

	// DefaultIconSyncSource is the same metadata base the manual "Sync
	// metadata" wizard reads, so an automatic run and a manual one can never
	// disagree about what the official icon for a model is.
	DefaultIconSyncSource = "https://basellm.github.io/llm-metadata"
)

type IconSyncSetting struct {
	Enabled       bool `json:"enabled"`
	IntervalHours int  `json:"interval_hours"`
	// Overwrite replaces an icon that is already set. Off by default: the
	// common case is filling in blanks, and silently repainting a hand-picked
	// icon is the kind of change an administrator would not expect from a job
	// they turned on to "add missing icons".
	Overwrite  bool   `json:"overwrite"`
	SourceBase string `json:"source_base"`
}

var iconSyncSetting = IconSyncSetting{
	Enabled:       false,
	IntervalHours: DefaultIconSyncIntervalHours,
	Overwrite:     false,
	SourceBase:    DefaultIconSyncSource,
}

func init() {
	config.GlobalConfig.Register("icon_sync_setting", &iconSyncSetting)
}

// GetIconSyncSetting returns the live settings with every field normalized, so
// a hand-edited database row cannot put the scheduler into a bad state.
func GetIconSyncSetting() IconSyncSetting {
	setting := iconSyncSetting
	if setting.IntervalHours < MinIconSyncIntervalHours {
		setting.IntervalHours = DefaultIconSyncIntervalHours
	}
	if setting.IntervalHours > MaxIconSyncIntervalHours {
		setting.IntervalHours = MaxIconSyncIntervalHours
	}
	if strings.TrimSpace(setting.SourceBase) == "" {
		setting.SourceBase = DefaultIconSyncSource
	}
	return setting
}

// ValidateIconSyncOption rejects values the scheduler cannot use. Option writes
// reach the database before any code reads the field back, so this is the only
// place a typo can be caught.
func ValidateIconSyncOption(key, value string) error {
	switch key {
	case "icon_sync_setting.interval_hours":
		hours, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("icon sync interval must be a whole number of hours")
		}
		if hours < MinIconSyncIntervalHours || hours > MaxIconSyncIntervalHours {
			return fmt.Errorf("icon sync interval must be between %d and %d hours", MinIconSyncIntervalHours, MaxIconSyncIntervalHours)
		}
	case "icon_sync_setting.source_base":
		if !strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "http://") {
			return fmt.Errorf("icon sync source must be an http(s) URL")
		}
	}
	return nil
}
