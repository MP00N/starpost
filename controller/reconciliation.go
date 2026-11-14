package controller

// 对账：把 usage 日志里的 token、缓存、额度核到一起，并且能追到人。
//
// 三个口径必须分开，混在一起就会出现「我花了一块多，它说花了好多」这种对不上的账：
//
//	实收   row.Quota —— 真的从用户账上扣掉的额度
//	重算   用这条日志自己记下的计费输入（表达式 / 倍率 / token）再算一遍
//	原价   官方发布的目录价 × 同样的 token 结构
//
// 实收 vs 重算是**会计自检**：两者不等说明计费代码或日志有 bug，正常的差应该接近 0。
// 实收 vs 原价才是**经营**：差出来的是加价率，盈亏页用它算毛利。
//
// 反过来说，早先只有「重算」一列，而重算用的是日志里的 model_ratio —— 那本身就是
// 我们的卖价，所以它俩天然相等，真正有信息量的那一列反而没有。同时重算漏掉了
// 缓存写入 token，凡是带 cache 写入的请求（Claude 系）都会误报偏差。
//
// 两个设计要点，都是实际用下来的教训：
//
//  1. 「额度为 0」不等于「漏计费」。自部署模型的价格本来就可以设 0，这时每条
//     请求都是 0 额度，全报成异常就变成噪声。所以先看这个模型在本站的倍率：
//     倍率为 0（或按次价格为 0）→ 判为 free，只是计数，不报异常。
//  2. 异常必须带用户。只报模型名等于「知道漏钱了但不知道谁漏的」，没法处置。
//     用户、令牌、分组、渠道、IP 全部带出来。
//
// 只读：全程不写库（除了把异常落进 billing_anomalies 台账）。

import (
	"encoding/base64"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type modelAudit struct {
	Model        string  `json:"model"`
	Requests     int     `json:"requests"`
	Prompt       int64   `json:"prompt"`
	Completion   int64   `json:"completion"`
	CacheRead    int64   `json:"cache_read"`
	CacheWrite   int64   `json:"cache_write"`
	Quota        int64   `json:"quota"`
	Expected     float64 `json:"expected"`
	Official     float64 `json:"official"`
	HasOfficial  bool    `json:"has_official"`
	CacheHitRate float64 `json:"cache_hit_rate"`
	// Drift 是实收与重算的相对差，会计自检用
	Drift float64 `json:"drift"`
	// Markup 实收 / 原价，加价倍数；1.0 表示按原价卖，0 表示没有原价可比
	Markup float64 `json:"markup"`
	// PricedBy 这个模型靠什么计费：expr / ratio / price / none
	PricedBy string `json:"priced_by"`
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
	OfficialTotal   float64        `json:"official_total"`
	CacheSavedQuota float64        `json:"cache_saved_quota"`
	QuotaPerUnit    float64        `json:"quota_per_unit"`
	Models          []modelAudit   `json:"models"`
	Anomalies       []auditAnomaly `json:"anomalies"`
	FreeModelRows   int            `json:"free_model_rows"`
	// UnverifiableRows 是日志里没有任何计费输入（老日志）的行数。它们无法自检，
	// 汇总成一行说明，而不是每行报一条异常刷满台账。
	UnverifiableRows int `json:"unverifiable_rows"`
	// OfficialModels 是价格簿里能查到原价的模型数，0 表示还没拉到官方价。
	OfficialModels int `json:"official_models"`
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

// parseOtherRaw keeps the non-numeric fields (billing_mode, expr_b64, 计费 token 表)
// that the recompute needs. parseOtherNumbers deliberately drops them.
func parseOtherRaw(raw string) map[string]any {
	out := map[string]any{}
	if raw == "" {
		return out
	}
	_ = common.UnmarshalJsonStr(raw, &out)
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

// billedTokens extracts the expression-mode token table the log recorded under
// other.billing_tokens. Recomputing from it is exact: it is the same numbers the
// settlement used, not a reconstruction from prompt/completion.
func billedTokens(raw map[string]any) (map[string]float64, bool) {
	nested, ok := raw["billing_tokens"].(map[string]any)
	if !ok || len(nested) == 0 {
		return nil, false
	}
	out := make(map[string]float64, len(nested))
	for key, value := range nested {
		if number, ok := toFloatValue(value); ok {
			out[key] = number
		}
	}
	return out, len(out) > 0
}

func toFloatValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		return parsed, err == nil
	}
	return 0, false
}

// logExpression decodes other.expr_b64 back into a billing expression.
func logExpression(raw map[string]any) (string, bool) {
	encoded, ok := raw["expr_b64"].(string)
	if !ok || encoded == "" {
		return "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) == 0 {
		return "", false
	}
	return string(decoded), true
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
	unverifiableRows := 0

	// 「日志里没记计费依据」是**同一类**历史遗留问题，不是几百个独立事件。
	// 每条日志各报一条，台账一次对账就被几百行同样的话淹掉，真正需要处理
	// 的个别异常反而看不见。这类记录按模型汇总成一条，带上条数和示例请求。
	type legacyMiss struct {
		count      int
		first      auditRow
		quotaTotal float64
	}
	legacyMisses := map[string]*legacyMiss{}

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
		rawOther := parseOtherRaw(row.Other)
		if row.Other != "" && len(other) == 0 && len(rawOther) == 0 {
			addAnomaly("broken_other", row, "日志的 other 字段不是合法 JSON，这条请求的计费输入全部丢失，无法复核。", 0, 0)
		}

		cacheRead, _ := firstOf(other, "cache_tokens", "cache_read_tokens")
		cacheWrite, _ := firstOf(other, "cache_creation_tokens", "cache_write_tokens")
		modelRatio, hasModelRatio := firstOf(other, "model_ratio")
		completionRatio, hasCompletionRatio := firstOf(other, "completion_ratio")
		cacheRatio, hasCacheRatio := firstOf(other, "cache_ratio")
		createCacheRatio, hasCreateCacheRatio := firstOf(other, "cache_creation_ratio")
		groupRatio, hasGroupRatio := firstOf(other, "group_ratio")
		if !hasGroupRatio || groupRatio == 0 {
			groupRatio = 1
		}
		modelPrice, hasModelPrice := firstOf(other, "model_price")
		imageTokens, _ := firstOf(other, "image_output", "image_tokens")
		audioTokens, _ := firstOf(other, "audio_input_token_count", "audio_tokens")
		audioPrice, hasAudioPrice := firstOf(other, "audio_input_price")

		billingMode, _ := rawOther["billing_mode"].(string)
		isTieredExpr := billingMode == "tiered_expr"

		// 计费方式先判清楚，再谈价格。日志里这三个字段的取值各有约定，
		// 混着用会得出荒谬的结果：
		//
		//	倍率计费   model_price = -1（占位），model_ratio > 0
		//	按次计费   model_price > 0，model_ratio = 0
		//	表达式计费 model_price = 0，model_ratio = 0，另带 billing_mode
		//
		// 所以「model_price 存在」什么都不说明——它每条日志都有；只有它大于 0
		// 才是真的按次计费。早先按「存在即按次」跳过重算，等于把所有倍率请求
		// 的重算列都留成空；按「等于 0 即免费」又会把表达式计费的模型全判成零价。
		perRequestPriced := hasModelPrice && modelPrice > 0
		ratioPriced := !perRequestPriced && hasModelRatio && modelRatio > 0

		// 本站对该模型就是零价：额度为 0 是预期，不是异常。必须有明确证据
		// （记了倍率且为 0，或记了单次价且为 0）才算免费；字段整个缺失是
		// 「不知道」，那要报异常，不能当成免费放过去。
		zeroRatio := hasModelRatio && modelRatio == 0
		zeroPrice := hasModelPrice && modelPrice == 0
		isFreeModel := !isTieredExpr && !perRequestPriced && (zeroRatio || zeroPrice)
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

		// 原价：官方发布的目录价 × 这条请求的 token 结构。和盈亏页同源。
		if official, _, ok := service.OfficialCostQuota(row.ModelName, service.OfficialTokens{
			Prompt:      float64(row.PromptTokens),
			Completion:  float64(row.CompletionTokens),
			CacheRead:   cacheRead,
			CacheCreate: cacheWrite,
			Image:       imageTokens,
			Audio:       audioTokens,
		}); ok {
			entry.Official += official
			entry.HasOfficial = true
		}

		if row.PromptTokens < 0 || row.CompletionTokens < 0 {
			addAnomaly("negative_tokens", row, "日志里记的 token 数是负数，这条请求的计费数据不可信。", 0, 0)
		}
		// 免费模型跳过这条，否则自部署站点每次对账都是满屏红字
		if row.Quota == 0 && row.PromptTokens+row.CompletionTokens > 0 && !isFreeModel {
			addAnomaly("quota_zero", row,
				"这条请求有 "+strconv.Itoa(row.PromptTokens+row.CompletionTokens)+
					" 个 token，却扣了 0 额度，而该模型在本站不是零价——说明计费没生效。",
				0, 0)
		}
		if int64(cacheRead) > int64(row.PromptTokens) {
			addAnomaly("cache_exceeds_prompt", row,
				"缓存命中的 "+strconv.FormatFloat(cacheRead, 'f', 0, 64)+
					" 个 token 比输入总数 "+strconv.Itoa(row.PromptTokens)+" 还多，这一行的 token 统计有问题。",
				0, 0)
		}

		// ── 重算：优先用表达式口径，其次倍率口径 ──────────────────────────
		var expected float64
		verifiable := false
		switch {
		case isTieredExpr:
			entry.PricedBy = "expr"
			if expression, ok := logExpression(rawOther); ok {
				if tokens, ok := billedTokens(rawOther); ok {
					if value, ok := evaluateLoggedExpression(expression, tokens, groupRatio); ok {
						expected = value
						verifiable = true
					}
				}
			}
			if !verifiable {
				// 表达式计费没有倍率可选，重算不了就说清楚，别报成「缺倍率」
				unverifiableRows++
			}
		case perRequestPriced:
			entry.PricedBy = "price"
			// 按次计费：模型价格就是单次美元价，乘配额单位与分组倍率
			expected = modelPrice * common.QuotaPerUnit * groupRatio
			verifiable = true
		case ratioPriced:
			entry.PricedBy = "ratio"
			// 输入部分要减掉缓存读**和**缓存写：日志里的 prompt_tokens 含这两段，
			// 只用它们各自的价格计一次，否则同一段 token 会被收两遍或漏一遍。
			base := float64(row.PromptTokens) - cacheRead - cacheWrite - imageTokens - audioTokens
			if base < 0 {
				base = 0
			}
			effectiveCacheRatio := 1.0
			if hasCacheRatio {
				effectiveCacheRatio = cacheRatio
			}
			effectiveWriteRatio := 1.0
			if hasCreateCacheRatio {
				effectiveWriteRatio = createCacheRatio
			}
			if hasCompletionRatio {
				expected = (base +
					cacheRead*effectiveCacheRatio +
					cacheWrite*effectiveWriteRatio +
					imageTokens +
					audioTokens +
					float64(row.CompletionTokens)*completionRatio) * modelRatio
			}
			// 音频按独立单价计费时，那部分不参与文本倍率
			if hasAudioPrice && audioPrice > 0 && audioTokens > 0 {
				expected -= audioTokens * modelRatio
				expected += audioPrice / 1_000_000 * audioTokens * common.QuotaPerUnit
			}
			expected *= groupRatio
			verifiable = true
		default:
			entry.PricedBy = "none"
			unverifiableRows++
			// 一条日志既没有表达式、也没有倍率或单次价，说明它写入时就没有
			// 记下计费依据（多为旧版本记录），或者计费环节根本没跑到。归到
			// 按模型的汇总里，见下面的 legacyMisses。
			if !isFreeModel {
				bucket, ok := legacyMisses[row.ModelName]
				if !ok {
					// 用这个模型的**第一条**日志做代表：它的 id 进指纹，
					// 所以重复对账不会重复入库。
					bucket = &legacyMiss{first: row}
					legacyMisses[row.ModelName] = bucket
				}
				bucket.count++
				bucket.quotaTotal += float64(row.Quota)
			}
		}

		if verifiable {
			entry.Expected += expected
			if hasCacheRatio && cacheRatio < 1 && cacheRead > 0 {
				cacheSaved += cacheRead * (1 - cacheRatio) * modelRatio * groupRatio
			}

			// 零价模型的预期值本来就是 0，不做偏差判定
			if !isFreeModel && expected > 0 {
				actual := float64(row.Quota)
				drift := math.Abs(actual-expected) / expected
				if drift > 0.03 {
					addAnomaly("billing_drift", row, driftDetail(row, expected, actual, drift), expected, actual)
				}
			}
		}
	}

	// 历史遗留的「日志没记计费依据」按模型汇总成一条，排在其它异常之后。
	// 它说的是同一个事实 N 次，合并后管理员一眼能看出「哪些模型的历史账
	// 核不了」，而不是被几百条一模一样的话挡住视线。
	legacyModels := make([]string, 0, len(legacyMisses))
	for name := range legacyMisses {
		legacyModels = append(legacyModels, name)
	}
	sort.Strings(legacyModels)
	for _, name := range legacyModels {
		bucket := legacyMisses[name]
		quotaUSD := bucket.quotaTotal / common.QuotaPerUnit
		detail := "这个模型有 " + strconv.Itoa(bucket.count) + " 条日志没有记下计费依据，" +
			"合计扣了 " + strconv.FormatFloat(quotaUSD, 'f', 4, 64) + " 美元：" +
			"核不了这些钱是怎么算出来的。多半是写入这些日志时的旧版本还没记录计费信息，" +
			"新产生的日志不会再这样。"
		anomalies = append(anomalies, auditAnomaly{
			Kind: "no_ratio", Model: name, Detail: detail,
			Actual: bucket.quotaTotal,
			UserID: bucket.first.UserId, Username: bucket.first.Username,
			TokenName: bucket.first.TokenName, Group: bucket.first.Group,
			ChannelID: bucket.first.ChannelId, Channel: bucket.first.ChannelName,
			RequestID: bucket.first.RequestId, CreatedAt: bucket.first.CreatedAt,
			LogID: bucket.first.Id,
		})
	}

	result := make([]modelAudit, 0, len(byModel))
	var quotaTotal int64
	expectedTotal, officialTotal := 0.0, 0.0
	for _, entry := range byModel {
		denominator := entry.Prompt + entry.CacheRead
		if denominator > 0 {
			entry.CacheHitRate = float64(entry.CacheRead) / float64(denominator)
		}
		if entry.Expected > 0 {
			entry.Drift = math.Abs(float64(entry.Quota)-entry.Expected) / entry.Expected
		}
		if entry.Official > 0 {
			entry.Markup = float64(entry.Quota) / entry.Official
		}
		quotaTotal += entry.Quota
		expectedTotal += entry.Expected
		officialTotal += entry.Official
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
		Days:             days,
		Rows:             len(rows),
		QuotaTotal:       quotaTotal,
		ExpectedTotal:    expectedTotal,
		OfficialTotal:    officialTotal,
		CacheSavedQuota:  cacheSaved,
		QuotaPerUnit:     common.QuotaPerUnit,
		Models:           result,
		Anomalies:        anomalies,
		FreeModelRows:    freeRows,
		UnverifiableRows: unverifiableRows,
		OfficialModels:   service.GetOfficialPriceStatus().Models,
	})
}

// evaluateLoggedExpression re-runs an expression-mode request from the exact
// token table the settlement recorded. Returns quota after the group ratio.
func evaluateLoggedExpression(expression string, tokens map[string]float64, groupRatio float64) (float64, bool) {
	params := billingexpr.TokenParams{
		P:     tokens["p"],
		C:     tokens["c"],
		Len:   tokens["len"],
		CR:    tokens["cr"],
		CC:    tokens["cc"],
		CC1h:  tokens["cc1h"],
		CC12h: tokens["cc12h"],
		Img:   tokens["img"],
		ImgCR: tokens["img_cr"],
		ImgO:  tokens["img_o"],
		AI:    tokens["ai"],
		AO:    tokens["ao"],
	}
	cost, _, err := billingexpr.RunExpr(expression, params)
	if err != nil || cost < 0 || math.IsInf(cost, 0) || math.IsNaN(cost) {
		return 0, false
	}
	// v1 expressions emit dollars × 1e6; see pkg/billingexpr.quotaConversion.
	return cost / 1_000_000 * common.QuotaPerUnit * groupRatio, true
}

// driftDetail explains a billing drift in one sentence an administrator can act
// on: how much was charged, how much the rule says, and by how much they differ.
// The model name is not repeated here — the ledger renders it as its own column.
func driftDetail(row auditRow, expected, actual, drift float64) string {
	return "实收 " + formatQuotaAmount(actual) + "，按这条日志记的计费规则应为 " +
		formatQuotaAmount(expected) + "，差 " + strconv.FormatFloat(drift*100, 'f', 1, 64) +
		"%。"
}

// formatQuotaAmount renders a quota figure as both the raw count and its dollar
// equivalent, because a bare number means nothing to whoever reads the ledger.
func formatQuotaAmount(quota float64) string {
	if common.QuotaPerUnit <= 0 {
		return strconv.FormatFloat(quota, 'f', 0, 64)
	}
	usd := quota / common.QuotaPerUnit
	return strconv.FormatFloat(usd, 'f', 4, 64) + " 美元（" +
		strconv.FormatFloat(quota, 'f', 0, 64) + " 额度）"
}
