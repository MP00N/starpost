package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// 计费异常的管理接口：列表、标记已解决（进回收站）、回收站清理、强制上架。

// GetBillingAnomalies 列表。status=open 待处理；status=resolved 回收站。
func GetBillingAnomalies(c *gin.Context) {
	status := c.DefaultQuery("status", model.AnomalyStatusOpen)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	items, total, err := model.GetBillingAnomalies(status, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ResolveBillingAnomaly 标记已解决 → 移入回收站。
// 注意：这不是删除。记录仍在库里，只是状态变成 resolved，
// 超过保留天数后才由清理任务物理删除。
func ResolveBillingAnomaly(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的异常 ID")
		return
	}
	var req struct {
		Resolution string `json:"resolution"`
	}
	_ = c.ShouldBindJSON(&req)

	operatorId := c.GetInt("id")
	if err := model.ResolveBillingAnomaly(id, operatorId, req.Resolution); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id})
}

// PurgeResolvedAnomalies 手动清理回收站（可选，清理任务也会自动跑）。
func PurgeResolvedAnomalies(c *gin.Context) {
	keepDays, _ := strconv.Atoi(c.DefaultQuery("keep_days", "30"))
	removed, err := model.PurgeResolvedAnomalies(keepDays)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"removed": removed})
}

// GetModelProfit 盈亏分析。
func GetModelProfitHandler(c *gin.Context) {
	GetModelProfit(c)
}

// GetModelLossHolds 当前因亏损被下架的模型。
func GetModelLossHolds(c *gin.Context) {
	common.ApiSuccess(c, gin.H{"holds": service.GetLossHolds()})
}

// ForceListModel 管理员选择「仍然上架」，接受亏损。
func ForceListModel(c *gin.Context) {
	var req struct {
		Model string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Model == "" {
		common.ApiErrorMsg(c, "缺少模型名")
		return
	}
	if err := service.ForceListLossModel(req.Model); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"model": req.Model, "force_listed": true})
}

// ReleaseModelHold 手动解除挂起（改完价后也可以直接点这个）。
func ReleaseModelHold(c *gin.Context) {
	var req struct {
		Model  string `json:"model"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Model == "" {
		common.ApiErrorMsg(c, "缺少模型名")
		return
	}
	if err := service.ReleaseLossModel(req.Model, req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"model": req.Model, "released": true})
}

// GetOpenAnomalyCount 侧栏徽标。
func GetOpenAnomalyCount(c *gin.Context) {
	common.ApiSuccess(c, gin.H{"count": model.CountOpenAnomalies()})
}
