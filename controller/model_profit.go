package controller

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// 盈亏分析 — 用真实调用记录，算每个模型到底赚还是亏。
//
// 为什么不能只看配置里的倍率：倍率是「你以为的价格」，真实成本是「上游实际
// 扣的钱」。两者之间隔着缓存命中率、输出/输入比、分组折扣、上游自己的调价。
// 一个模型标价 1.0 倍看着有毛利，实际全是长输出 + 零缓存，可能已经在亏。
//
// 算法：取窗口内的消费日志，按模型聚合 token，再用两个价格算两笔钱：
//
//	收入 = 本站倍率算出的额度（就是用户被扣的）
//	成本 = 上游倍率算出的额度（我们付给上游的）
//
// 上游倍率从哪来：日志的 other 里没有上游倍率，所以由管理员在「上游成本」
// 里配置每个模型的进价倍率（可以一次从 new-api 上游的 /api/pricing 导入）。
// 没配进价的模型不参与盈亏判定——宁可不报，不能瞎报。

type modelProfit struct {
	Model string `json:"model"`

	Requests   int   `json:"requests"`
	Prompt     int64 `json:"prompt"`
	Completion int64 `json:"completion"`
	CacheRead  int64 `json:"cache_read"`

	// 本站定价（卖出）
	SellRatio       float64 `json:"sell_ratio"`
	CompletionRatio float64 `json:"completion_ratio"`
	CacheRatio      float64 `json:"cache_ratio"`

	// 上游进价（买入），未配置时为 0 且 HasCost=false
	CostRatio      float64 `json:"cost_ratio"`
	CostCompletion float64 `json:"cost_completion_ratio"`
	CostCacheRatio float64 `json:"cost_cache_ratio"`
	HasCost        bool    `json:"has_cost"`

	Revenue float64 `json:"revenue"` // 额度
	Cost    float64 `json:"cost"`    // 额度
	Margin  float64 `json:"margin"`  // 额度，负数=亏

	// MarginRate 毛利率，收入为 0 时无意义（null）
	MarginRate *float64 `json:"margin_rate"`

	// 这些 token 里有多少是白送出去的（缓存没打折的部分）
	CacheSaving float64 `json:"cache_saving"`

	// 判定
	Verdict string `json:"verdict"` // profitable / loss / unknown
}

type profitResponse struct {
	Days      float64       `json:"days"`
	Models    []modelProfit `json:"models"`
	LossCount int           `json:"loss_count"`
	// 有定价但没有进价的模型数：这些无法判定盈亏
	UnknownCount int `json:"unknown_count"`
}

// 上游成本配置的 option key：{"模型名": {"ratio":..,"completion_ratio":..,"cache_ratio":..}}
const upstreamCostOptionKey = "model_upstream_cost"

// getUpstreamCosts 读进价表。
//
// 坑：存进去的结构里有 quota_type（数字）、source（字符串）等字段。早先这里
// 用 map[string]map[string]float64 解析，遇到字符串字段整条解析失败，然后静默
// 返回空 map——表现就是「进价表有 2 条，但盈亏页全显示未配进价」。
// 现在按真实结构解析，只取需要的三个数字。
func getUpstreamCosts() map[string]upstreamCostEntry {
	out := map[string]upstreamCostEntry{}
	raw := common.OptionMap[upstreamCostOptionKey]
	if raw == "" {
		return out
	}
	var decoded map[string]upstreamCostEntry
	if err := common.UnmarshalJsonStr(raw, &decoded); err != nil {
		logger.LogError(nil, common.LogText("parse upstream cost failed: %v", err))
		return out
	}
	return decoded
}

// GetModelProfit 按模型算盈亏。
func GetModelProfit(c *gin.Context) {
	days := 7.0
	if raw := c.Query("days"); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			days = parsed
		}
	}
	if days > 365 {
		days = 365
	}
	since := time.Now().Unix() - int64(days*86400)

	var rows []auditRow
	err := model.DB.Model(&model.Log{}).
		Select("model_name, prompt_tokens, completion_tokens, quota, other").
		Where("type = ? AND created_at >= ?", 2, since).
		Find(&rows).Error
	if err != nil {
		common.ApiError(c, err)
		return
	}

	costs := getUpstreamCosts()

	type agg struct {
		modelProfit
		sellRatioSum  float64
		completionSum float64
		cacheRatioSum float64
		ratioCount    int
	}
	byModel := map[string]*agg{}

	for _, row := range rows {
		other := parseOtherNumbers(row.Other)
		cacheRead, _ := firstOf(other, "cache_tokens", "cache_read_tokens")
		sellRatio, hasSell := firstOf(other, "model_ratio")
		completionRatio, hasCompletion := firstOf(other, "completion_ratio")
		cacheRatio, hasCache := firstOf(other, "cache_ratio")
		groupRatio, hasGroup := firstOf(other, "group_ratio")
		if !hasGroup || groupRatio == 0 {
			groupRatio = 1
		}
		if !hasSell {
			continue
		}

		entry, ok := byModel[row.ModelName]
		if !ok {
			entry = &agg{modelProfit: modelProfit{Model: row.ModelName}}
			byModel[row.ModelName] = entry
		}
		entry.Requests++
		entry.Prompt += int64(row.PromptTokens)
		entry.Completion += int64(row.CompletionTokens)
		entry.CacheRead += int64(cacheRead)

		// 倍率取窗口内的算术平均：同一模型被改过价时，取均值比取最后一条准
		entry.sellRatioSum += sellRatio
		if hasCompletion {
			entry.completionSum += completionRatio
		}
		if hasCache {
			entry.cacheRatioSum += cacheRatio
		}
		entry.ratioCount++

		// 收入用日志里记录的额度，这是真扣到的钱，不是重算的
		entry.Revenue += float64(row.Quota)
	}

	for name, entry := range byModel {
		if entry.ratioCount > 0 {
			entry.SellRatio = entry.sellRatioSum / float64(entry.ratioCount)
			entry.CompletionRatio = entry.completionSum / float64(entry.ratioCount)
			entry.CacheRatio = entry.cacheRatioSum / float64(entry.ratioCount)
		}

		cost, ok := costs[name]
		if ok && cost.Ratio > 0 {
			entry.HasCost = true
			entry.CostRatio = cost.Ratio
			entry.CostCompletion = cost.CompletionRatio
			entry.CostCacheRatio = cost.CacheRatio
			if entry.CostCompletion == 0 {
				entry.CostCompletion = entry.CompletionRatio
			}
			if entry.CostCacheRatio == 0 {
				entry.CostCacheRatio = entry.CacheRatio
			}

			// 成本按同样的 token 口径算：缓存部分按缓存价，其余按输入价，
			// 输出按输出倍率。上游如果不支持缓存，缓存倍率配 1 即可。
			base := float64(entry.Prompt) - float64(entry.CacheRead)
			if base < 0 {
				base = 0
			}
			entry.Cost = base*entry.CostRatio +
				float64(entry.CacheRead)*entry.CostCacheRatio*entry.CostRatio +
				float64(entry.Completion)*entry.CostCompletion*entry.CostRatio
		}

		entry.Margin = entry.Revenue - entry.Cost
		if entry.HasCost && entry.Revenue > 0 {
			rate := entry.Margin / entry.Revenue
			entry.MarginRate = &rate
		}

		switch {
		case !entry.HasCost:
			entry.Verdict = "unknown"
		case entry.Margin < 0:
			entry.Verdict = "loss"
		default:
			entry.Verdict = "profitable"
		}
	}

	result := make([]modelProfit, 0, len(byModel))
	lossCount, unknownCount := 0, 0
	for _, entry := range byModel {
		result = append(result, entry.modelProfit)
		switch entry.Verdict {
		case "loss":
			lossCount++
		case "unknown":
			unknownCount++
		}
	}

	// 亏得最多的排前面，其次按收入降序
	sort.Slice(result, func(i, j int) bool {
		if result[i].Verdict != result[j].Verdict {
			return result[i].Verdict == "loss"
		}
		return result[i].Revenue > result[j].Revenue
	})

	// 亏损模型：记录 + 自动下架。
	//
	// 下架是这次分析的实际动作，不只是显示。但有两个前提，缺一不可：
	//   · 该模型配了进价（否则"亏损"只是没数据，不是事实）
	//   · 窗口内收入 > 0（零请求的模型无从判断）
	//
	// 下架后守卫会一直压着它，直到改价或管理员强制上架。
	for _, item := range result {
		if item.Verdict != "loss" {
			continue
		}
		if item.Revenue <= 0 {
			continue
		}

		_ = model.UpsertBillingAnomaly(&model.BillingAnomaly{
			Fingerprint: "loss:" + item.Model,
			Kind:        model.AnomalyLossModel,
			ModelName:   item.Model,
			Expected:    item.Revenue,
			Actual:      item.Cost,
			Detail: "按当前进价，该模型在窗口内亏损 " +
				strconv.FormatFloat(math.Abs(item.Margin), 'f', 0, 64) + " 额度",
			OccurredAt: time.Now().Unix(),
		})

		// 只有管理员开了自动下架才动手
		if common.OptionMap["model_loss_auto_hold"] != "true" {
			continue
		}
		_ = service.HoldLossModel(
			item.Model, math.Abs(item.Margin),
			"窗口内亏损，已自动下架", item.SellRatio)
	}

	common.ApiSuccess(c, profitResponse{
		Days:         days,
		Models:       result,
		LossCount:    lossCount,
		UnknownCount: unknownCount,
	})
}
