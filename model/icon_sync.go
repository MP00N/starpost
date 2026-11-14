package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// Icon sync writes only the `icon` column.
//
// Routing it through Model.Update would be wrong: that method rewrites every
// metadata column from a struct that a background job has not fully loaded, so
// a concurrent edit to a description or a vendor would be reverted. It also
// bumps the record version, which would make an administrator's open editor
// sheet fail its optimistic check for a change they never made.
//
// Both tables are touched in one transaction and one sync topic bump, so the
// other nodes reload once rather than twice.

// IconUpdate is one record's pending icon assignment.
type IconUpdate struct {
	ModelName string `json:"model_name"`
	Icon      string `json:"icon"`
}

// IconSyncResult reports what one run changed, for the task row and the card.
type IconSyncResult struct {
	// ModelsScanned and VendorsScanned count rows examined, not rows changed.
	ModelsScanned  int `json:"models_scanned"`
	VendorsScanned int `json:"vendors_scanned"`
	ModelsUpdated  int `json:"models_updated"`
	VendorsUpdated int `json:"vendors_updated"`
	// AlreadySet counts records whose icon was present and left alone.
	AlreadySet int `json:"already_set"`
	// NoUpstreamIcon counts records the feed has no icon for.
	NoUpstreamIcon int `json:"no_upstream_icon"`
	// Sample lists a few changed names so the task row is readable.
	Sample []string `json:"sample,omitempty"`
}

// ApplyIconSync writes the given icon assignments.
//
// A model's icon is only written when the row still exists and — unless
// overwrite is set — still has an empty icon. The check happens inside the
// transaction against the locked row, not against a snapshot read earlier, so
// an administrator who set an icon while the job was fetching keeps it.
func ApplyIconSync(modelIcons []IconUpdate, vendorIcons map[string]string, overwrite bool) (*IconSyncResult, error) {
	result := &IconSyncResult{}
	err := metadataTransaction(func(tx *gorm.DB) error {
		now := common.GetTimestamp()

		if len(modelIcons) > 0 {
			names := make([]string, 0, len(modelIcons))
			for _, update := range modelIcons {
				names = append(names, update.ModelName)
			}
			var rows []Model
			if err := lockForUpdate(tx).Select("id", "model_name", "icon").
				Where("model_name IN ?", names).Find(&rows).Error; err != nil {
				return err
			}
			existing := make(map[string]Model, len(rows))
			for _, row := range rows {
				existing[row.ModelName] = row
			}
			for _, update := range modelIcons {
				row, ok := existing[update.ModelName]
				if !ok {
					continue
				}
				if strings.TrimSpace(row.Icon) != "" && !overwrite {
					result.AlreadySet++
					continue
				}
				if err := tx.Model(&Model{}).Where("id = ?", row.Id).
					Updates(map[string]any{"icon": update.Icon, "updated_time": now}).Error; err != nil {
					return err
				}
				result.ModelsUpdated++
				if len(result.Sample) < 5 {
					result.Sample = append(result.Sample, update.ModelName)
				}
			}
		}

		if len(vendorIcons) > 0 {
			var rows []Vendor
			if err := lockForUpdate(tx).Select("id", "name", "icon").Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				icon, ok := vendorIcons[strings.ToLower(strings.TrimSpace(row.Name))]
				if !ok || icon == "" {
					continue
				}
				if strings.TrimSpace(row.Icon) != "" && !overwrite {
					result.AlreadySet++
					continue
				}
				if err := tx.Model(&Vendor{}).Where("id = ?", row.Id).
					Updates(map[string]any{"icon": icon, "updated_time": now}).Error; err != nil {
					return err
				}
				result.VendorsUpdated++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// The icon is display metadata, but the model square and the pricing cache
	// both carry it, so refresh them the same way a metadata edit would.
	RefreshPricing()
	return result, nil
}

// IconTargets returns the models and vendors a sync may write, with their
// current icons, so the caller can decide what is missing without loading the
// whole table again inside the transaction.
func IconTargets() (map[string]string, map[string]string, error) {
	modelIcons := make(map[string]string)
	vendorIcons := make(map[string]string)

	var models []Model
	if err := DB.Select("model_name", "icon").Find(&models).Error; err != nil {
		return nil, nil, err
	}
	for _, item := range models {
		modelIcons[item.ModelName] = item.Icon
	}

	var vendors []Vendor
	if err := DB.Select("name", "icon").Find(&vendors).Error; err != nil {
		return nil, nil, err
	}
	for _, vendor := range vendors {
		vendorIcons[strings.ToLower(strings.TrimSpace(vendor.Name))] = vendor.Icon
	}
	return modelIcons, vendorIcons, nil
}

// ErrIconSyncDisabled reports a run requested while the schedule is off. Manual
// runs bypass it; the scheduler does not need to, since it checks Enabled first.
var ErrIconSyncDisabled = errors.New("icon synchronization is disabled")
