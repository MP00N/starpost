package controller

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// 对账：把 usage 日志里的 token、缓存、额度核到一起，并且能追到人。
//
// 两个设计要点，都是实际用下来的教训：
//
//  1. 「额度为 0」不等于「漏计费」。自部署模型的价格本来就可以设 0，这时每条
//     请求都是 0 额度，全报成异常就变成噪声。所以先看这个模型在本站的倍率：
//     倍率为 0（或按次价格为 0）→ 判为 free，只是计数，不报异常。
//  2. 异常必须带用户。只报模型名等于「知道漏钱了但不知道谁漏的」，没法处置。
//     用户、令牌、分组、渠道、IP 全部带出来。
//
// 只读：全程不写库。

type modelAudit struct {
	Model        string  `json:"model"`
	Requests     int     `json:"requests"`
	Prompt       int64   `json:"prompt"`
	Completion   int64   `json:"completion"`
	CacheRead    int64   `json:"cache_read"`
	CacheWrite   int64   `json:"cache_write"`
	Quota        int64   `json:"quota"`
	Expected     float64 `json:"expected"`
	CacheHitRate float64 `json:"cache_hit_rate"`
	Drift        float64 `json:"drift"`
	// Free 表示本站对该模型就是零价（自部署等），额度为 0 是预期行为
	Free bool `json:"free"`
}

type auditAnomaly struct {
	Kind     string  `json:"kind"`
	Model    string  `json:"model"`
	Detail   string  `json:"detail"`
	Expected float64 `json:"expected,omitempty"`
	Actual   float64 `json:"actual,omitempty"`

	// 追人用：出问题的这次请求是谁发的
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	TokenName string `json:"token_name"`
	Group     string `json:"group"`
	ChannelID int    `json:"channel_id"`
	Channel   string `json:"channel_name"`
	IP        string `json:"ip"`
	RequestID string `json:"request_id"`
	CreatedAt int64  `json:"created_at"`
	LogID     int    `json:"log_id"`
}

type auditResponse struct {
	Days            float64        `json:"days"`
	Rows            int            `json:"rows"`
	QuotaTotal      int64          `json:"quota_total"`
	ExpectedTotal   float64        `json:"expected_total"`
	CacheSavedQuota float64        `json:"cache_saved_quota"`
	QuotaPerUnit    float64        `json:"quota_per_unit"`
	Models          []modelAudit   `json:"models"`
	Anomalies       []auditAnomaly `json:"anomalies"`
	FreeModelRows   int            `json:"free_model_rows"`
}

type auditRow struct {
	Id               int
	UserId           int
	Username         string
	TokenName        string
	ModelName        string
	PromptTokens     int
	CompletionTokens int
	Quota            int
	Group            string
	ChannelId        int
	ChannelName      string
	Ip               string
	RequestId        string
	CreatedAt        int64
	Other            string
}

func parseOtherNumbers(raw string) map[string]float64 {
	out := map[string]float64{}
	if raw == "" {
		return out
	}
	var decoded map[string]any
	if err := common.UnmarshalJsonStr(raw, &decoded); err != nil {
		return out
	}
	for key, value := range decoded {
		switch typed := value.(type) {
		case float64:
			out[key] = typed
		case string:
			if parsed, err := strconv.ParseFloat(typed, 64); err == nil {
				out[key] = parsed
			}
		}
	}
	return out
}

func firstOf(values map[string]float64, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value, true
		}
	}
	return 0, false
}

// GetReconciliationAudit 汇总近 N 天的消费日志，反查计费偏差，并带出责任人。
func GetReconciliationAudit(c *gin.Context) {
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
		Select("id, user_id, username, token_name, model_name, prompt_tokens, "+
			"completion_tokens, quota, \"group\", channel_id, channel_name, ip, "+
			"request_id, created_at, other").
		Where("type = ? AND created_at >= ?", 2, since).
		Find(&rows).Error
	if err != nil {
		common.ApiError(c, err)
		return
	}

	byModel := map[string]*modelAudit{}
	anomalies := make([]auditAnomaly, 0)
	cacheSaved := 0.0
	freeRows := 0

	addAnomaly := func(kind string, row auditRow, detail string, expected, actual float64) {
		if len(anomalies) >= 300 {
			return
		}
		anomalies = append(anomalies, auditAnomaly{
			Kind: kind, Model: row.ModelName, Detail: detail,
			Expected: expected, Actual: actual,
			UserID: row.UserId, Username: row.Username, TokenName: row.TokenName,
			Group: row.Group, ChannelID: row.ChannelId, Channel: row.ChannelName,
			IP: row.Ip, RequestID: row.RequestId, CreatedAt: row.CreatedAt,
			LogID: row.Id,
		})
	}

	for _, row := range rows {
		other := parseOtherNumbers(row.Other)
		if row.Other != "" && len(other) == 0 {
			addAnomaly("broken_other", row, "other 字段无法解析", 0, 0)
		}

		cacheRead, _ := firstOf(other, "cache_tokens", "cache_read_tokens")
		cacheWrite, _ := firstOf(other, "cache_creation_tokens", "cache_write_tokens")
		modelRatio, hasModelRatio := firstOf(other, "model_ratio")
		completionRatio, hasCompletionRatio := firstOf(other, "completion_ratio")
		cacheRatio, hasCacheRatio := firstOf(other, "cache_ratio")
		groupRatio, hasGroupRatio := firstOf(other, "group_ratio")
		if !hasGroupRatio || groupRatio == 0 {
			groupRatio = 1
		}
		modelPrice, hasModelPrice := firstOf(other, "model_price")

		// 本站对该模型就是零价：额度为 0 是预期，不是异常
		isFreeModel := (hasModelPrice && modelPrice == 0) ||
			(hasModelRatio && modelRatio == 0)
		if isFreeModel {
			freeRows++
		}

		entry, ok := byModel[row.ModelName]
		if !ok {
			entry = &modelAudit{Model: row.ModelName, Free: isFreeModel}
			byModel[row.ModelName] = entry
		}
		entry.Requests++
		entry.Prompt += int64(row.PromptTokens)
		entry.Completion += int64(row.CompletionTokens)
		entry.CacheRead += int64(cacheRead)
		entry.CacheWrite += int64(cacheWrite)
		entry.Quota += int64(row.Quota)

		if row.PromptTokens < 0 || row.CompletionTokens < 0 {
			addAnomaly("negative_tokens", row, "token 为负", 0, 0)
		}
		// 免费模型跳过这条，否则自部署站点每次对账都是满屏红字
		if row.Quota == 0 && row.PromptTokens+row.CompletionTokens > 0 && !isFreeModel {
			addAnomaly("quota_zero", row, "有 token 但额度为 0（该模型不是零价）", 0, 0)
		}
		if int64(cacheRead) > int64(row.PromptTokens) {
			addAnomaly("cache_exceeds_prompt", row, "缓存读 token 大于 prompt token", 0, 0)
		}

		if hasModelPrice {
			continue
		}
		if !hasModelRatio || !hasCompletionRatio {
			if !isFreeModel {
				addAnomaly("no_ratio", row, "日志里没有倍率字段", 0, 0)
			}
			continue
		}

		base := float64(row.PromptTokens) - cacheRead
		if base < 0 {
			base = 0
		}
		effectiveCacheRatio := 1.0
		if hasCacheRatio {
			effectiveCacheRatio = cacheRatio
		}
		expected := (base*modelRatio +
			cacheRead*effectiveCacheRatio*modelRatio +
			float64(row.CompletionTokens)*completionRatio*modelRatio) * groupRatio
		entry.Expected += expected

		if hasCacheRatio && cacheRatio < 1 && cacheRead > 0 {
			cacheSaved += cacheRead * (1 - cacheRatio) * modelRatio * groupRatio
		}

		// 零价模型的预期值本来就是 0，不做偏差判定
		if isFreeModel {
			continue
		}
		actual := float64(row.Quota)
		if expected > 0 {
			drift := math.Abs(actual-expected) / expected
			if drift > 0.03 {
				addAnomaly("billing_drift", row, "实收与按倍率重算不一致", expected, actual)
			}
		}
	}

	result := make([]modelAudit, 0, len(byModel))
	var quotaTotal int64
	expectedTotal := 0.0
	for _, entry := range byModel {
		denominator := entry.Prompt + entry.CacheRead
		if denominator > 0 {
			entry.CacheHitRate = float64(entry.CacheRead) / float64(denominator)
		}
		if entry.Expected > 0 {
			entry.Drift = math.Abs(float64(entry.Quota)-entry.Expected) / entry.Expected
		}
		quotaTotal += entry.Quota
		expectedTotal += entry.Expected
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Quota > result[j].Quota })

	// 按时间倒序，最近的先看
	sort.Slice(anomalies, func(i, j int) bool {
		return anomalies[i].CreatedAt > anomalies[j].CreatedAt
	})

	// 持久化：异常不能只活在这一次请求里。用量日志会按 TTL 清掉，
	// 但账目问题必须留痕，直到有人确认处置过。
	//
	// 指纹 = 类型 + 日志 id：同一条日志的同一类异常只存一条。重复对账、
	// 或者页面刷新，都只会命中已有记录，不会堆出几百条重复。
	for _, item := range anomalies {
		fingerprint := item.Kind + ":" + strconv.Itoa(item.LogID)
		_ = model.UpsertBillingAnomaly(&model.BillingAnomaly{
			Fingerprint: fingerprint,
			Kind:        item.Kind,
			ModelName:   item.Model,
			LogId:       item.LogID,
			UserId:      item.UserID,
			Username:    item.Username,
			TokenName:   item.TokenName,
			Group:       item.Group,
			ChannelId:   item.ChannelID,
			ChannelName: item.Channel,
			Ip:          item.IP,
			RequestId:   item.RequestID,
			Expected:    item.Expected,
			Actual:      item.Actual,
			Detail:      item.Detail,
			OccurredAt:  item.CreatedAt,
		})
	}

	common.ApiSuccess(c, auditResponse{
		Days:            days,
		Rows:            len(rows),
		QuotaTotal:      quotaTotal,
		ExpectedTotal:   expectedTotal,
		CacheSavedQuota: cacheSaved,
		QuotaPerUnit:    common.QuotaPerUnit,
		Models:          result,
		Anomalies:       anomalies,
		FreeModelRows:   freeRows,
	})
}
