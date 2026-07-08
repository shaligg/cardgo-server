package gamedata

import (
	"encoding/json"
	"fmt"
	"os"
)

// WorkshopData 是工坊配置的运行时只读集合。
type WorkshopData struct {
	Facilities map[string]FacilityConfig
}

// FacilityConfig 定义一个工坊设施及其各等级升级消耗。
type FacilityConfig struct {
	FacilityID string                `json:"facility_id"`
	Name       string                `json:"name"`
	MaxLevel   int                   `json:"max_level"`
	Levels     []FacilityLevelConfig `json:"levels"`
}

// FacilityLevelConfig 表示升到某个等级时需要的消耗。
//
// Level=1 表示初始等级，通常不配置消耗；Level=2 表示从 1 级升到 2 级。
type FacilityLevelConfig struct {
	Level        int          `json:"level"`
	UpgradeCosts []CostConfig `json:"upgrade_costs,omitempty"`
}

type facilitiesFile struct {
	Facilities []FacilityConfig `json:"facilities"`
}

// LoadWorkshopData 加载并校验工坊设施配置。
func LoadWorkshopData(path string, items ItemCatalog) (*WorkshopData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read workshop config: %w", err)
	}
	var file facilitiesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("unmarshal workshop config: %w", err)
	}
	return NewWorkshopData(file.Facilities, items)
}

// NewWorkshopData 构建工坊配置索引，并在启动期发现配置错误。
func NewWorkshopData(facilities []FacilityConfig, items ItemCatalog) (*WorkshopData, error) {
	out := &WorkshopData{Facilities: map[string]FacilityConfig{}}
	for _, facility := range facilities {
		if facility.FacilityID == "" || facility.Name == "" {
			return nil, fmt.Errorf("facility missing required fields: %+v", facility)
		}
		if facility.MaxLevel <= 1 {
			return nil, fmt.Errorf("facility %s max_level must be greater than 1", facility.FacilityID)
		}
		if len(facility.Levels) == 0 {
			return nil, fmt.Errorf("facility %s levels are required", facility.FacilityID)
		}
		if _, ok := out.Facilities[facility.FacilityID]; ok {
			return nil, fmt.Errorf("duplicate facility_id: %s", facility.FacilityID)
		}
		seenLevel := map[int]bool{}
		for _, level := range facility.Levels {
			if level.Level < 1 || level.Level > facility.MaxLevel {
				return nil, fmt.Errorf("facility %s level %d out of range", facility.FacilityID, level.Level)
			}
			if seenLevel[level.Level] {
				return nil, fmt.Errorf("facility %s duplicate level %d", facility.FacilityID, level.Level)
			}
			seenLevel[level.Level] = true
			if level.Level > 1 {
				if err := validateCosts(fmt.Sprintf("facility %s upgrade to level %d", facility.FacilityID, level.Level), level.UpgradeCosts, items); err != nil {
					return nil, err
				}
			}
		}
		out.Facilities[facility.FacilityID] = facility
	}
	if len(out.Facilities) == 0 {
		return nil, fmt.Errorf("workshop data must include facilities")
	}
	return out, nil
}
