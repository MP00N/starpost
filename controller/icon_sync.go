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

// iconSyncPreviewTimeout bounds a preview request. The feed is one static file;
// anything slower is a network problem the administrator should see as an
// error rather than as a hanging page.
const iconSyncPreviewTimeout = 60 * time.Second

// PreviewIconSync reports what an icon run would change, without writing
// anything. An administrator enabling the schedule can see how many models
// still lack a brand mark first.
func PreviewIconSync(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), iconSyncPreviewTimeout)
	defer cancel()

	result, err := service.RunIconSyncExclusive(ctx, true)
	if errors.Is(err, service.ErrIconSyncBusy) {
		common.ApiErrorStatus(c, http.StatusConflict, common.NewMessage("An icon synchronization is already running"))
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// TriggerIconSync queues one synchronization through the system task runner, so
// a manual run and a scheduled run go through the same code path and both leave
// a task row an administrator can inspect afterwards.
//
// Manual runs are allowed while the schedule is off: that is how an
// administrator fills in icons once and then decides whether to keep the
// schedule.
func TriggerIconSync(c *gin.Context) {
	task, created, err := service.EnqueueSystemTask(model.SystemTaskTypeIconSync, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "model.icon.sync", map[string]any{"created": created})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    task.ToResponse(),
	})
}

// GetIconSyncStatus reports the last run and whether the schedule is on, so the
// settings card can render without scanning the task list.
func GetIconSyncStatus(c *gin.Context) {
	setting := operation_setting.GetIconSyncSetting()
	latest, err := model.GetLatestSystemTasks([]string{model.SystemTaskTypeIconSync})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var last any
	if task := latest[model.SystemTaskTypeIconSync]; task != nil {
		last = task.ToResponse()
	}
	active, err := model.GetActiveSystemTask(model.SystemTaskTypeIconSync)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"enabled":        setting.Enabled,
			"interval_hours": setting.IntervalHours,
			"overwrite":      setting.Overwrite,
			"last_run":       last,
			"running":        active != nil,
		},
	})
}
