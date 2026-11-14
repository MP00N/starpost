package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

// 亏损模型守卫 — 自动把亏钱的模型下架，改价后自动放回。
//
// 为什么需要它：一个模型标错价，可能跑几天才被发现，这几天就是纯亏。人工看
// 对账再手动关，永远慢一步。所以把判定挂到系统任务里定时跑。
//
// 三条规则：
//
//  1. 检测到某模型在当前定价下亏损 → 把所有渠道里该模型的 Ability 置为
//     enabled=false。下架立即生效，不影响其他模型。
//  2. 下架的模型记入「亏损挂起」。只要还挂着，任何重新上架的操作都会被兜底
//     压回下架状态（EnforceLossHolds）。
//  3. 管理员改了该模型的倍率后，挂起解除、自动上架，并留系统日志。
//
// 例外：管理员可以「强制上架」，写入 ForceListed，守卫不再管它，但对账里
// 仍然持续显示为亏损。

const lossHoldOptionKey = "model_loss_hold"

// lossHold 存在 option 里：模型名 → 挂起信息。
type lossHold struct {
	Model         string  `json:"model"`
	HoldSince     int64   `json:"hold_since"`
	LossQuota     float64 `json:"loss_quota"`
	Reason        string  `json:"reason"`
	ForceListed   bool    `json:"force_listed"`
	RatioWhenHeld float64 `json:"ratio_when_held"`
}

func loadLossHolds() map[string]lossHold {
	out := map[string]lossHold{}
	raw := common.OptionMap[lossHoldOptionKey]
	if raw == "" {
		return out
	}
	var decoded map[string]lossHold
	if err := common.UnmarshalJsonStr(raw, &decoded); err != nil {
		return out
	}
	return decoded
}

func saveLossHolds(holds map[string]lossHold) error {
	encoded, err := common.Marshal(holds)
	if err != nil {
		return err
	}
	return model.UpdateOption(lossHoldOptionKey, string(encoded))
}

// HoldLossModel 把某模型整体下架，并记录挂起原因。
func HoldLossModel(modelName string, lossQuota float64, reason string, ratioWhenHeld float64) error {
	holds := loadLossHolds()
	if existing, ok := holds[modelName]; ok && existing.ForceListed {
		// 管理员强制上架过，不再自动下架
		return nil
	}

	if err := model.DB.Model(&model.Ability{}).
		Where("model = ?", modelName).
		Update("enabled", false).Error; err != nil {
		return err
	}

	holds[modelName] = lossHold{
		Model:         modelName,
		HoldSince:     time.Now().Unix(),
		LossQuota:     lossQuota,
		Reason:        reason,
		RatioWhenHeld: ratioWhenHeld,
	}
	if err := saveLossHolds(holds); err != nil {
		return err
	}

	logger.LogInfo(nil, common.LogText(
		"model held off shelves for loss: model=%s loss=%.0f", modelName, lossQuota))
	return nil
}

// ReleaseLossModel 解除挂起并恢复上架。
func ReleaseLossModel(modelName, reason string) error {
	holds := loadLossHolds()
	if _, ok := holds[modelName]; !ok {
		return nil
	}
	delete(holds, modelName)
	if err := saveLossHolds(holds); err != nil {
		return err
	}
	if err := model.DB.Model(&model.Ability{}).
		Where("model = ?", modelName).
		Update("enabled", true).Error; err != nil {
		return err
	}
	logger.LogInfo(nil, common.LogText(
		"model released back on shelves: model=%s reason=%s", modelName, reason))
	return nil
}

// ForceListLossModel 管理员选择「仍然上架」。
func ForceListLossModel(modelName string) error {
	holds := loadLossHolds()
	entry, ok := holds[modelName]
	if !ok {
		return errors.New("该模型没有亏损挂起记录")
	}
	entry.ForceListed = true
	holds[modelName] = entry
	if err := saveLossHolds(holds); err != nil {
		return err
	}
	return model.DB.Model(&model.Ability{}).
		Where("model = ?", modelName).
		Update("enabled", true).Error
}

// GetLossHolds 给前端展示挂起列表。
func GetLossHolds() []lossHold {
	holds := loadLossHolds()
	out := make([]lossHold, 0, len(holds))
	for _, entry := range holds {
		out = append(out, entry)
	}
	return out
}

// EnforceLossHolds 兜底：任何重新上架的动作之后调用，把仍挂起且未强制上架
// 的模型重新压回 disabled。没有这一步，编辑一次渠道亏损模型就悄悄回来了。
func EnforceLossHolds() error {
	holds := loadLossHolds()
	if len(holds) == 0 {
		return nil
	}
	for name, entry := range holds {
		if entry.ForceListed {
			continue
		}
		if err := model.DB.Model(&model.Ability{}).
			Where("model = ? AND enabled = ?", name, true).
			Update("enabled", false).Error; err != nil {
			return fmt.Errorf("enforce hold %s: %w", name, err)
		}
	}
	return nil
}

// SyncLossHoldsWithPricing 改价后调用：倍率变过的挂起模型自动放回。
func SyncLossHoldsWithPricing(currentRatios map[string]float64) {
	holds := loadLossHolds()
	changed := false
	for name, entry := range holds {
		current, ok := currentRatios[name]
		if !ok {
			continue
		}
		if current != entry.RatioWhenHeld && current > 0 {
			delete(holds, name)
			changed = true
			_ = model.DB.Model(&model.Ability{}).
				Where("model = ?", name).
				Update("enabled", true).Error
			logger.LogInfo(nil, common.LogText(
				"model auto-relisted after ratio change: model=%s %.4f -> %.4f",
				name, entry.RatioWhenHeld, current))
		}
	}
	if changed {
		_ = saveLossHolds(holds)
	}
}

// RunAnomalyRetention 周期性清理回收站。
//
// 只清 resolved 且超过保留天数的。open 的记录一律不动——用量日志有 TTL，
// 这张表没有，否则等于把没处理完的问题自己删掉。
func RunAnomalyRetention(keepDays int) {
	removed, err := model.PurgeResolvedAnomalies(keepDays)
	if err != nil {
		logger.LogError(nil, common.LogText("anomaly retention failed: %v", err))
		return
	}
	if removed > 0 {
		logger.LogInfo(nil, common.LogText(
			"anomaly retention purged %d resolved records older than %d days",
			removed, keepDays))
	}
}
