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
//	收入 = 日志里记录的额度（就是用户被扣的，不是重算的）
//	成本 = 上游进价 × 实际 token 结构
//
// 进价从哪来，两级：
//
//	1. 管理员配置的「上游成本」（就是真实进价，可含分组折扣）—— 优先
//	2. 官方发布的按 1M token 单价（service.OfficialCostQuota）—— 兜底
//
// 只配了官方兜底价时判出的「亏损」不会触发自动下架：官方价是目录价，你实际
// 拿到的进价可能更低，拿目录价去下架模型会误伤。只有配了真实进价的亏损才动手。

type modelProfit struct {
	Model string `json:"model"`

	Requests   int   `json:"requests"`
	Prompt     int64 `json:"prompt"`
	Completion int64 `json:"completion"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`

	// 本站定价（卖出）
	SellRatio       float64 `json:"sell_ratio"`
	CompletionRatio float64 `json:"completion_ratio"`
	CacheRatio      float64 `json:"cache_ratio"`

	// 上游进价（买入）；CostSource 说明这个数字从哪来
	CostRatio      float64 `json:"cost_ratio"`
	CostCompletion float64 `json:"cost_completion_ratio"`
	CostCacheRatio float64 `json:"cost_cache_ratio"`
	HasCost        bool    `json:"has_cost"`
	// CostSource: configured（你填的进价）/ official（官方目录价兜底）/ ""
	CostSource string `json:"cost_source"`

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

	// 合计：界面上直接告诉管理员「这段时间收了多少钱、付出去多少」
	RevenueTotal float64 `json:"revenue_total"`
	CostTotal    float64 `json:"cost_total"`
	MarginTotal  float64 `json:"margin_total"`
	// 参与合计的模型里，有多少个成本来自官方目录价兜底
	OfficialCostModels int `json:"official_cost_models"`
	QuotaPerUnit       float64 `json:"quota_per_unit"`
}

// 上游成本配置的 option key：{"模型名": {"ratio":..,"completion_ratio":..,"cache_ratio":..}}
const upstreamCostOptionKey = "model_upstream_cost"

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

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
		sellRatioSum      float64
		sellCompletions   float64
		sellCaches        float64
		ratioCount        int
		// officialCost 是官方目录价算出来的成本，只在没配进价时才用得到。
		// 这里不直接覆盖 Cost，是为了让「配置的进价」始终优先。
		officialCost float64
		officialOK   bool
	}
	byModel := map[string]*agg{}

	for _, row := range rows {
		other := parseOtherNumbers(row.Other)
		cacheRead, _ := firstOf(other, "cache_tokens", "cache_read_tokens")
		cacheWrite, _ := firstOf(other, "cache_creation_tokens", "cache_write_tokens")
		cacheWrite1h, _ := firstOf(other, "cache_creation_tokens_1h")
		cacheWrite12h, _ := firstOf(other, "cache_creation_tokens_12h")
		imageTokens, _ := firstOf(other, "image_output", "image_tokens")
		audioTokens, _ := firstOf(other, "audio_input_token_count", "audio_tokens")
		sellRatio, hasSell := firstOf(other, "model_ratio")
		completionRatio, hasCompletion := firstOf(other, "completion_ratio")
		cacheRatio, hasCache := firstOf(other, "cache_ratio")
		groupRatio, hasGroup := firstOf(other, "group_ratio")
		if !hasGroup || groupRatio == 0 {
			groupRatio = 1
		}
		// 日志没记 model_ratio 就不跳过这一行。
		//
		// 早先这里是 `if !hasSell { continue }`，理由是「没有卖价就算不了盈亏」。
		// 但收入根本不需要卖价——它就是日志里的 quota，真扣到的钱。跳过等于把
		// 所有旧日志的真实收入从合计里删掉：管理员明明花了钱，盈亏页却显示
		// 收入为零，两个页面对着干。缺卖价只影响「倍率是多少」这一列的展示，
		// 那是可以留空的。
		//
		// 官方原价和实际扣费都不依赖这个字段，所以这些行照样能算出成本与毛利。

		entry, ok := byModel[row.ModelName]
		if !ok {
			entry = &agg{modelProfit: modelProfit{Model: row.ModelName}}
			byModel[row.ModelName] = entry
		}
		entry.Requests++
		entry.Prompt += int64(row.PromptTokens)
		entry.Completion += int64(row.CompletionTokens)
		entry.CacheRead += int64(cacheRead)
		entry.CacheWrite += int64(cacheWrite)

		// 倍率取窗口内的算术平均：同一模型被改过价时，取均值比取最后一条准
		if hasSell {
			entry.sellRatioSum += sellRatio
			entry.sellCompletions += boolToFloat(hasCompletion) * completionRatio
			entry.sellCaches += boolToFloat(hasCache) * cacheRatio
			entry.ratioCount++
		}

		// 收入用日志里记录的额度，这是真扣到的钱，不是重算的
		entry.Revenue += float64(row.Quota)

		// 官方目录价：逐条请求累加，这样缓存命中率、长短输出这些结构差异
		// 会自然体现在成本里，而不是用窗口平均值糊一遍。
		cost, _, ok := service.OfficialCostQuota(row.ModelName, service.OfficialTokens{
			Prompt:         float64(row.PromptTokens),
			Completion:     float64(row.CompletionTokens),
			CacheRead:      cacheRead,
			CacheCreate:    cacheWrite,
			CacheCreate1h:  cacheWrite1h,
			CacheCreate12h: cacheWrite12h,
			Image:          imageTokens,
			Audio:          audioTokens,
		})
		if ok {
			entry.officialCost += cost
			entry.officialOK = true
		}
	}

	for name, entry := range byModel {
		if entry.ratioCount > 0 {
			entry.SellRatio = entry.sellRatioSum / float64(entry.ratioCount)
			entry.CompletionRatio = entry.sellCompletions / float64(entry.ratioCount)
			entry.CacheRatio = entry.sellCaches / float64(entry.ratioCount)
		}

		cost, ok := costs[name]
		switch {
		case ok && cost.Ratio > 0:
			// 配了真实进价：这是「我实际花多少钱」，最可信
			entry.HasCost = true
			entry.CostSource = "configured"
			entry.CostRatio = cost.Ratio
			entry.CostCompletion = cost.CompletionRatio
			entry.CostCacheRatio = cost.CacheRatio
			if entry.CostCompletion == 0 {
				entry.CostCompletion = entry.CompletionRatio
			}
			if entry.CostCacheRatio == 0 {
				entry.CostCacheRatio = entry.CacheRatio
			}

			// 成本按同样的 token 口径算：缓存命中部分按缓存价，缓存写入部分按
			// 写入价，其余按输入价，输出按输出倍率。上游如果不支持缓存，倍率配
			// 1 即可。写入价单独取值：把它按命中价算会低估成本（写入通常更贵）。
			costWriteRatio := cost.CreateCache
			if costWriteRatio <= 0 {
				costWriteRatio = entry.CostCacheRatio
			}
			base := float64(entry.Prompt) - float64(entry.CacheRead) - float64(entry.CacheWrite)
			if base < 0 {
				base = 0
			}
			entry.Cost = base*entry.CostRatio +
				float64(entry.CacheRead)*entry.CostCacheRatio*entry.CostRatio +
				float64(entry.CacheWrite)*costWriteRatio*entry.CostRatio +
				float64(entry.Completion)*entry.CostCompletion*entry.CostRatio
		case entry.officialOK:
			// 没配进价，用官方目录价兜底：数字有了，但它不是你的成交价
			entry.HasCost = true
			entry.CostSource = "official"
			entry.Cost = entry.officialCost
		}

		entry.Margin = entry.Revenue - entry.Cost
		if entry.HasCost && entry.Revenue > 0 {
			rate := entry.Margin / entry.Revenue
			entry.MarginRate = &rate
		}

		// 缓存省下的钱：命中部分本来按输入价收，实际按缓存价收
		if entry.CacheRatio > 0 && entry.CacheRatio < 1 && entry.CacheRead > 0 {
			entry.CacheSaving = float64(entry.CacheRead) * (1 - entry.CacheRatio) * entry.SellRatio
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
	lossCount, unknownCount, officialCostModels := 0, 0, 0
	var revenueTotal, costTotal float64
	for _, entry := range byModel {
		result = append(result, entry.modelProfit)
		revenueTotal += entry.Revenue
		if entry.HasCost {
			costTotal += entry.Cost
			if entry.CostSource == "official" {
				officialCostModels++
			}
		}
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
	// 下架是这次分析的实际动作，不只是显示。前提必须齐：
	//   · 该模型配了真实进价（官方目录价兜底不算——那是目录价，不是你付的价）
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
		if item.CostSource != "configured" {
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
		Days:               days,
		Models:             result,
		LossCount:          lossCount,
		UnknownCount:       unknownCount,
		RevenueTotal:       revenueTotal,
		CostTotal:          costTotal,
		MarginTotal:        revenueTotal - costTotal,
		OfficialCostModels: officialCostModels,
		QuotaPerUnit:       common.QuotaPerUnit,
	})
}
