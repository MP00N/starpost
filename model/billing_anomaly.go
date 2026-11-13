package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// 计费异常 — a persisted record of something that did not add up.
//
// 为什么要有这张表，而不是只在对账页实时算：
//
//	实时算只能看到「当前窗口内的账」，历史窗口一过就消失了。真正的账目问题
//	需要留痕——谁、什么时候、哪条请求、差额多少、后来怎么处置的。
//
// 生命周期（三期，不可跳过）：
//
//	open      发现，待处理
//	resolved  管理员点了「已解决」，进回收站（resolved_at 记录时间）
//	（删除）   回收站里超过保留天数的，由清理任务物理删除
//
// 关键约束：**不做定时清理 open 的记录**。用量日志有 TTL，这张表没有——
// 没处理完的账目问题永远留着，否则等于把问题自己删掉了。

const (
	AnomalyStatusOpen     = "open"
	AnomalyStatusResolved = "resolved"
)

// 异常类型
const (
	AnomalyQuotaZero     = "quota_zero"           // 有 token 但额度为 0（非零价模型）
	AnomalyBillingDrift  = "billing_drift"        // 实收与按倍率重算不符
	AnomalyCacheOverflow = "cache_exceeds_prompt" // 缓存读 > prompt
	AnomalyNegativeToken = "negative_tokens"      // token 为负
	AnomalyBrokenOther   = "broken_other"         // other 字段无法解析
	AnomalyNoRatio       = "no_ratio"             // 缺倍率字段
	AnomalyLossModel     = "loss_model"           // 该模型按当前定价是亏损的
)

type BillingAnomaly struct {
	Id int `json:"id" gorm:"primaryKey"`

	// 指纹：同一条日志的同一类异常只留一条，重复对账不会刷屏
	Fingerprint string `json:"fingerprint" gorm:"type:varchar(80);uniqueIndex"`

	Kind   string `json:"kind" gorm:"type:varchar(32);index"`
	Status string `json:"status" gorm:"type:varchar(16);index;default:open"`

	ModelName string `json:"model_name" gorm:"type:varchar(128);index"`
	LogId     int    `json:"log_id" gorm:"index"`

	// 责任人与上下文：追人靠这些
	UserId      int    `json:"user_id" gorm:"index"`
	Username    string `json:"username" gorm:"type:varchar(64);index"`
	TokenName   string `json:"token_name" gorm:"type:varchar(128)"`
	Group       string `json:"group" gorm:"type:varchar(64)"`
	ChannelId   int    `json:"channel_id" gorm:"index"`
	ChannelName string `json:"channel_name" gorm:"type:varchar(128)"`
	Ip          string `json:"ip" gorm:"type:varchar(64)"`
	RequestId   string `json:"request_id" gorm:"type:varchar(64);index"`

	// 金额与依据
	Expected float64 `json:"expected"`
	Actual   float64 `json:"actual"`
	Detail   string  `json:"detail" gorm:"type:varchar(255)"`

	// 精确到秒的请求时间，和发现时间分开：前者是事实，后者是处理轨迹
	OccurredAt int64 `json:"occurred_at" gorm:"bigint;index"`
	CreatedAt  int64 `json:"created_at" gorm:"bigint;index"`

	// 处理轨迹
	ResolvedAt int64  `json:"resolved_at" gorm:"bigint;index"`
	ResolvedBy int    `json:"resolved_by"`
	Resolution string `json:"resolution" gorm:"type:varchar(255)"`
}

func (BillingAnomaly) TableName() string { return "billing_anomalies" }

// UpsertBillingAnomaly 按指纹去重写入。已存在的（含已解决的）不覆盖状态，
// 否则一次重新对账会把删掉的异常又变回「待处理」。
func UpsertBillingAnomaly(entry *BillingAnomaly) error {
	if entry.Fingerprint == "" {
		return errors.New("fingerprint required")
	}
	var existing BillingAnomaly
	err := DB.Where("fingerprint = ?", entry.Fingerprint).First(&existing).Error
	if err == nil {
		return nil // 已记录过，保留原状态
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}
	if entry.Status == "" {
		entry.Status = AnomalyStatusOpen
	}
	return DB.Create(entry).Error
}

// GetBillingAnomalies 列表。status 为空时返回待处理；"resolved" 返回回收站。
func GetBillingAnomalies(status string, page, pageSize int) ([]BillingAnomaly, int64, error) {
	if status == "" {
		status = AnomalyStatusOpen
	}
	query := DB.Model(&BillingAnomaly{}).Where("status = ?", status)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	var items []BillingAnomaly
	err := query.Order("occurred_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&items).Error
	return items, total, err
}

// ResolveBillingAnomaly 标记已解决并移入回收站。
func ResolveBillingAnomaly(id int, operatorId int, resolution string) error {
	now := time.Now().Unix()
	result := DB.Model(&BillingAnomaly{}).
		Where("id = ? AND status = ?", id, AnomalyStatusOpen).
		Updates(map[string]any{
			"status":      AnomalyStatusResolved,
			"resolved_at": now,
			"resolved_by": operatorId,
			"resolution":  resolution,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("anomaly not found or already resolved")
	}
	return nil
}

// PurgeResolvedAnomalies 只清回收站里超过保留天数的记录。
// 待处理的一律不动——这是这张表和用量日志最大的区别。
func PurgeResolvedAnomalies(keepDays int) (int64, error) {
	if keepDays <= 0 {
		keepDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays).Unix()
	result := DB.Where("status = ? AND resolved_at > 0 AND resolved_at < ?",
		AnomalyStatusResolved, cutoff).Delete(&BillingAnomaly{})
	return result.RowsAffected, result.Error
}

// CountOpenAnomalies 给侧栏徽标用。
func CountOpenAnomalies() int64 {
	var count int64
	DB.Model(&BillingAnomaly{}).Where("status = ?", AnomalyStatusOpen).Count(&count)
	return count
}
