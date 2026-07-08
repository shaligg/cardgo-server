// Package gamedata 负责加载和校验由策划 Excel 导出的 JSON 配置。
//
// 运行时只读取已经校验过的配置，业务模块不直接解析配置文件。
package gamedata

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	// ItemIDGold 是基础金币的全局道具 ID，当前存储在玩家主表字段中。
	ItemIDGold int64 = 1
	// ItemIDBasicMaterial 是示例基础材料的全局道具 ID，当前存储在通用背包中。
	ItemIDBasicMaterial int64 = 10001

	// StoragePlayerField 表示道具数量由玩家主表字段承载，例如金币。
	StoragePlayerField = "player_field"
	// StorageInventoryStack 表示道具数量由通用可堆叠背包承载。
	StorageInventoryStack = "inventory_stack"
)

// ItemConfig 是策划配置中的道具定义。
//
// ItemID 给策划、日志和服务端逻辑统一识别道具；StorageType 决定资产模块把变更路由到哪里。
type ItemConfig struct {
	ItemID       int64  `json:"item_id"`
	Key          string `json:"key"`
	Name         string `json:"name"`
	ItemType     string `json:"item_type"`
	StorageType  string `json:"storage_type"`
	StorageKey   string `json:"storage_key"`
	System       string `json:"system"`
	Stackable    bool   `json:"stackable"`
	VisibleInBag bool   `json:"visible_in_bag"`
}

// ItemCatalog 提供运行时查询道具配置的最小接口。
//
// 业务模块只依赖这个接口，方便测试时注入内存配置，也方便未来替换配置来源。
type ItemCatalog interface {
	GetItem(itemID int64) (ItemConfig, bool)
}

// Catalog 是 ItemConfig 的只读索引。
//
// 当前按 item_id 建立索引，启动时完成重复 ID、重复 key 和存储规则校验。
type Catalog struct {
	items map[int64]ItemConfig
}

type itemConfigFile struct {
	Items []ItemConfig `json:"items"`
}

// LoadItemCatalog 从 JSON 文件加载道具配置并构建只读索引。
func LoadItemCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read item config: %w", err)
	}

	var raw itemConfigFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal item config: %w", err)
	}
	return NewCatalog(raw.Items)
}

// NewCatalog 根据配置列表构建 Catalog。
//
// 这里集中做启动期校验，避免运行时遇到缺失字段、重复 ID 或不支持的存储类型。
func NewCatalog(items []ItemConfig) (*Catalog, error) {
	c := &Catalog{items: map[int64]ItemConfig{}}
	seenKeys := map[string]int64{}
	for _, item := range items {
		if item.ItemID <= 0 {
			return nil, fmt.Errorf("item_id must be positive: %+v", item)
		}
		if item.Key == "" {
			return nil, fmt.Errorf("item %d key is required", item.ItemID)
		}
		if _, ok := c.items[item.ItemID]; ok {
			return nil, fmt.Errorf("duplicate item_id: %d", item.ItemID)
		}
		if existingID, ok := seenKeys[item.Key]; ok {
			return nil, fmt.Errorf("duplicate item key %q: %d and %d", item.Key, existingID, item.ItemID)
		}
		if err := validateItemStorage(item); err != nil {
			return nil, err
		}
		c.items[item.ItemID] = item
		seenKeys[item.Key] = item.ItemID
	}
	if len(c.items) == 0 {
		return nil, fmt.Errorf("item config is empty")
	}
	return c, nil
}

// GetItem 根据 item_id 返回道具配置。
//
// 当 Catalog 未初始化或配置不存在时返回 false，由调用方决定报错还是降级。
func (c *Catalog) GetItem(itemID int64) (ItemConfig, bool) {
	if c == nil {
		return ItemConfig{}, false
	}
	item, ok := c.items[itemID]
	return item, ok
}

// validateItemStorage 校验道具的存储方式和必要字段是否匹配。
//
// 例如 player_field 必须声明 storage_key，inventory_stack 必须是可堆叠道具。
func validateItemStorage(item ItemConfig) error {
	switch item.StorageType {
	case StoragePlayerField:
		if item.StorageKey == "" {
			return fmt.Errorf("item %d player_field storage_key is required", item.ItemID)
		}
	case StorageInventoryStack:
		if !item.Stackable {
			return fmt.Errorf("item %d inventory_stack must be stackable", item.ItemID)
		}
	default:
		return fmt.Errorf("item %d unsupported storage_type: %s", item.ItemID, item.StorageType)
	}
	return nil
}
