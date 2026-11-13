package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// 上游进价导入 — 把一个 new-api 系上游的价格表读进来，作为成本口径。
//
// 为什么必须能导入：盈亏判定要用「上游对我们收多少」，这个数据只有上游自己知
// 道。手填 200 个模型不现实，而 new-api 系的上游会把价格表挂在 /api/pricing，
// 直接读过来是最省事也最不容易错的。
//
// 口径对齐（读下来的结构，实测于 api.furry.vg）：
//
//	quota_type = 0  按倍率计费：model_ratio / completion_ratio / cache_ratio
//	quota_type = 1  按次计费：model_price（每次请求固定价，单位是"每 token 美元"口径）
//
// 这里只取倍率口径。按次计费的模型如果混进倍率成本，会把成本算成 0 或天文数字，
// 所以单独标记为 unsupported，由界面提示人工处理。

type upstreamCostEntry struct {
	Ratio           float64 `json:"ratio"`
	CompletionRatio float64 `json:"completion_ratio"`
	CacheRatio      float64 `json:"cache_ratio"`
	CreateCache     float64 `json:"create_cache_ratio"`
	// QuotaType 原样保留，便于以后支持按次计费
	QuotaType int `json:"quota_type"`
	// ModelPrice 按次计费时的单价，倍率口径下不使用
	ModelPrice float64 `json:"model_price"`
	// BillingMode 记下上游的计费方式。tiered_expr 的原样保留表达式，
	// 让盈亏页能明确说"这个模型算不了"，而不是拿占位倍率瞎算。
	BillingMode string `json:"billing_mode,omitempty"`
	BillingExpr string `json:"billing_expr,omitempty"`
	ImportedAt  int64  `json:"imported_at"`
	Source      string `json:"source"`
}

// ImportUpstreamCost 从一个 new-api 上游的 /api/pricing 导入进价。
//
// 请求体：{"base_url":"https://api.example.com","key":"sk-..."}
// key 可以留空——很多站点的 /api/pricing 是公开的。
func ImportUpstreamCost(c *gin.Context) {
	var req struct {
		BaseURL string `json:"base_url"`
		Key     string `json:"key"`
		// Group 指定按哪个分组的倍率算成本。上游不同分组价格差很多
		// （实测同一上游 ClaudeAnti 0.42、ClaudeMax 0.85），必须让管理员选。
		Group string `json:"group"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.BaseURL == "" {
		common.ApiErrorMsg(c, "缺少上游地址")
		return
	}

	base := strings.TrimRight(req.BaseURL, "/")
	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequest(http.MethodGet, base+"/api/pricing", nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.Key)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		common.ApiErrorMsg(c, "连不上上游："+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		common.ApiErrorMsg(c, fmt.Sprintf("上游返回 %d", resp.StatusCode))
		return
	}

	var payload struct {
		Data []struct {
			ModelName       string  `json:"model_name"`
			QuotaType       int     `json:"quota_type"`
			ModelRatio      float64 `json:"model_ratio"`
			ModelPrice      float64 `json:"model_price"`
			CompletionRatio float64 `json:"completion_ratio"`
			CacheRatio      float64 `json:"cache_ratio"`
			CreateCache     float64 `json:"create_cache_ratio"`
			// EnableGroups 哪些分组能用这个模型。同一份价目表是全站的，
			// 但一条 key 往往只能进某个分组，按分组过滤才是这条 key 的真实成本。
			EnableGroups []string `json:"enable_groups"`
			// BillingMode 空或 "ratio" 才是倍率计费；
			// "tiered_expr" 是分段表达式，model_ratio 只是占位值，不能当价格用。
			BillingMode string `json:"billing_mode"`
			BillingExpr string `json:"billing_expr"`
		} `json:"data"`
		GroupRatio map[string]float64 `json:"group_ratio"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		common.ApiErrorMsg(c, "上游返回的不是 new-api 的价格表")
		return
	}

	// 分组倍率：选定的分组要给成本再乘一次，否则算出来的成本会偏小
	groupMultiplier := 1.0
	if req.Group != "" {
		if ratio, ok := payload.GroupRatio[req.Group]; ok && ratio > 0 {
			groupMultiplier = ratio
		}
	}

	imported := map[string]upstreamCostEntry{}
	unsupported := make([]string, 0)
	skipped := 0
	now := time.Now().Unix()

	for _, item := range payload.Data {
		if item.ModelName == "" {
			continue
		}
		// 只导这个分组真能用的模型。API 返回的是全站价目表（实测 201 条），
		// 而一条 key 常常只能进一个分组（实测 Codex (Special Offer) 只有 2 个
		// 模型）。不过滤的话，成本表里会塞进一堆这条 key 永远调不到的模型，
		// 盈亏分析也会把它们算进来。
		if req.Group != "" && len(item.EnableGroups) > 0 &&
			!containsString(item.EnableGroups, req.Group) {
			skipped++
			continue
		}
		// 分段表达式计费：model_ratio 是占位值，按倍率算会差好几倍。
		// 原样保留表达式，交给盈亏页判定为"无法按倍率计算"。
		//
		// 实测：api.furry.vg 的 gpt-6-astra 标 ratio=37.5，但真实价是
		// $0.8/1M（有效倍率 0.4）。按 37.5×0.08=3 算会得到 $6/1M，
		// 高估 7.5 倍——一个盈利的模型会被判成亏损然后被自动下架。
		if item.BillingMode == "tiered_expr" || item.BillingExpr != "" {
			imported[item.ModelName] = upstreamCostEntry{
				QuotaType:   item.QuotaType,
				BillingMode: "tiered_expr",
				BillingExpr: item.BillingExpr,
				ImportedAt:  now,
				Source:      base,
			}
			unsupported = append(unsupported, item.ModelName)
			continue
		}

		// 按次计费的模型不进倍率成本，单独列出来让管理员知道
		if item.QuotaType == 1 {
			unsupported = append(unsupported, item.ModelName)
			imported[item.ModelName] = upstreamCostEntry{
				QuotaType:  1,
				ModelPrice: item.ModelPrice,
				ImportedAt: now,
				Source:     base,
			}
			continue
		}
		imported[item.ModelName] = upstreamCostEntry{
			Ratio:           item.ModelRatio * groupMultiplier,
			CompletionRatio: item.CompletionRatio,
			CacheRatio:      item.CacheRatio,
			CreateCache:     item.CreateCache,
			QuotaType:       item.QuotaType,
			ImportedAt:      now,
			Source:          base,
		}
	}

	encoded, err := common.Marshal(imported)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption(upstreamCostOptionKey, string(encoded)); err != nil {
		common.ApiError(c, err)
		return
	}
	// UpdateOption 只写库，内存里的 OptionMap 要同步，否则本次请求内看不到
	common.OptionMap[upstreamCostOptionKey] = string(encoded)

	common.ApiSuccess(c, gin.H{
		"imported":    len(imported),
		"unsupported": unsupported,
		"group":       req.Group,
		"group_ratio": groupMultiplier,
		"source":      base,
	})
}

// GetUpstreamCost 读当前进价配置。
func GetUpstreamCost(c *gin.Context) {
	raw := common.OptionMap[upstreamCostOptionKey]
	if raw == "" {
		common.ApiSuccess(c, gin.H{"entries": map[string]any{}, "count": 0})
		return
	}
	var entries map[string]upstreamCostEntry
	if err := common.UnmarshalJsonStr(raw, &entries); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"entries": entries, "count": len(entries)})
}

// SetUpstreamCost 手工改单个模型的进价（界面里编辑用）。
func SetUpstreamCost(c *gin.Context) {
	var req struct {
		Model           string  `json:"model"`
		Ratio           float64 `json:"ratio"`
		CompletionRatio float64 `json:"completion_ratio"`
		CacheRatio      float64 `json:"cache_ratio"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Model == "" {
		common.ApiErrorMsg(c, "缺少模型名")
		return
	}

	entries := map[string]upstreamCostEntry{}
	if raw := common.OptionMap[upstreamCostOptionKey]; raw != "" {
		_ = common.UnmarshalJsonStr(raw, &entries)
	}
	// 0 表示删除这条进价，让该模型回到"未配置"
	if req.Ratio == 0 {
		delete(entries, req.Model)
	} else {
		entries[req.Model] = upstreamCostEntry{
			Ratio:           req.Ratio,
			CompletionRatio: req.CompletionRatio,
			CacheRatio:      req.CacheRatio,
			ImportedAt:      time.Now().Unix(),
			Source:          "manual",
		}
	}

	encoded, err := common.Marshal(entries)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption(upstreamCostOptionKey, string(encoded)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.OptionMap[upstreamCostOptionKey] = string(encoded)
	common.ApiSuccess(c, gin.H{"model": req.Model, "count": len(entries)})
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
