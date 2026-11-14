package controller

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// pricingSyncPreviewTimeout bounds a preview request. The feeds are two static
// files; anything slower than this is a network problem the administrator
// should see as an error rather than as a hanging page.
const pricingSyncPreviewTimeout = 90 * time.Second

// PreviewPricingSync fetches the official list and reports what a run would
// change, without writing anything. It is the "check first" half of the
// feature: an administrator can see the exact model count, currency and
// exchange rate before enabling the schedule.
func PreviewPricingSync(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), pricingSyncPreviewTimeout)
	defer cancel()

	result, err := service.RunPricingSyncExclusive(ctx, true, nil)
	if err != nil && errors.Is(err, service.ErrPricingSyncBusy) {
		common.ApiErrorStatus(c, http.StatusConflict, common.NewMessage("A price synchronization is already running"))
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// TriggerPricingSync queues one synchronization through the system task runner,
// so a manual run and a scheduled run go through exactly the same code path and
// both leave a task row an administrator can inspect afterwards.
//
// Manual runs are allowed while the schedule is off: that is how an
// administrator tries the feature before committing to it.
func TriggerPricingSync(c *gin.Context) {
	task, created, err := service.EnqueueSystemTask(model.SystemTaskTypePricingSync, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "model.pricing.sync", map[string]any{"created": created})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    task.ToResponse(),
	})
}

// GetPricingSyncStatus reports the last run and whether the schedule is on, so
// the settings card can render without scanning the task list.
func GetPricingSyncStatus(c *gin.Context) {
	setting := operation_setting.GetPricingSyncSetting()
	latest, err := model.GetLatestSystemTasks([]string{model.SystemTaskTypePricingSync})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var last any
	if task := latest[model.SystemTaskTypePricingSync]; task != nil {
		last = task.ToResponse()
	}
	active, err := model.GetActiveSystemTask(model.SystemTaskTypePricingSync)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	currency := operation_setting.ResolvePricingSyncCurrency(setting.Currency)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"enabled":               setting.Enabled,
			"interval_hours":        setting.IntervalHours,
			"currency":              setting.Currency,
			"resolved_currency":     currency,
			"scope":                 setting.Scope,
			"exchange_rate":         operation_setting.USDExchangeRate,
			"site_display_currency": operation_setting.GetQuotaDisplayType(),
			"last_run":              last,
			"running":               active != nil,
		},
	})
}
