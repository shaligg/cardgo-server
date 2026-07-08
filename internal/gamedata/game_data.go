package gamedata

import (
	"encoding/json"
	"fmt"
	"os"
)

// ConfigPaths 描述本次启动需要加载的策划配置文件路径。
//
// 服务启动时统一加载并校验，避免业务流程中临时读取文件。
type ConfigPaths struct {
	CardConfigPath  string
	OrderConfigPath string
	LevelConfigPath string
}

// GameData 是战斗和关卡相关配置的运行时只读集合。
type GameData struct {
	Cards  map[int64]CardConfig
	Orders map[int64]OrderConfig
	Levels map[int64]LevelConfig
}

// CardConfig 定义一张卡牌的基础属性和效果。
type CardConfig struct {
	CardID       int64                   `json:"card_id"`
	Key          string                  `json:"key"`
	Name         string                  `json:"name"`
	Rarity       string                  `json:"rarity"`
	CardType     string                  `json:"card_type"`
	Cost         int                     `json:"cost"`
	Effects      []EffectConfig          `json:"effects"`
	UpgradeCosts []CardUpgradeCostConfig `json:"upgrade_costs,omitempty"`
}

// CardUpgradeCostConfig 表示升级到目标等级时需要消耗的道具列表。
type CardUpgradeCostConfig struct {
	TargetLevel int          `json:"target_level"`
	Costs       []CostConfig `json:"costs"`
}

// EffectConfig 定义卡牌效果。
//
// 当前保持通用字段，具体字段含义由 effect_type 决定，后续实现战斗引擎时再细化解释器。
type EffectConfig struct {
	EffectType   string `json:"effect_type"`
	Trigger      string `json:"trigger"`
	Resource     string `json:"resource"`
	Value        int64  `json:"value"`
	ToResource   string `json:"to_resource"`
	ToValue      int64  `json:"to_value"`
	LimitPerTurn int    `json:"limit_per_turn"`
}

// OrderConfig 定义关卡中的订单目标。
//
// 玩家完成 requirements 后获得 rewards，奖励中的 item_id 必须能在道具配置中找到。
type OrderConfig struct {
	OrderID      int64            `json:"order_id"`
	Key          string           `json:"key"`
	Name         string           `json:"name"`
	OrderType    string           `json:"order_type"`
	TurnLimit    int              `json:"turn_limit"`
	Requirements []ResourceAmount `json:"requirements"`
	Rewards      []RewardConfig   `json:"rewards"`
	Tags         []string         `json:"tags"`
}

// ResourceAmount 表示订单需求中的一种资源及数量。
type ResourceAmount struct {
	Resource string `json:"resource"`
	Count    int64  `json:"count"`
}

// RewardConfig 表示一次奖励中的道具和数量。
type RewardConfig struct {
	ItemID int64 `json:"item_id"`
	Count  int64 `json:"count"`
}

// CostConfig 表示一次消耗中的道具和数量。
type CostConfig struct {
	ItemID int64 `json:"item_id"`
	Count  int64 `json:"count"`
}

// LevelConfig 定义一个关卡的基础参数、可出现订单和通关奖励。
type LevelConfig struct {
	LevelID            int64            `json:"level_id"`
	Name               string           `json:"name"`
	Chapter            int              `json:"chapter"`
	TurnLimit          int              `json:"turn_limit"`
	ActionPointPerTurn int              `json:"action_point_per_turn"`
	OrderSlots         int              `json:"order_slots"`
	InitialOrders      int              `json:"initial_orders"`
	Goal               LevelGoal        `json:"goal"`
	FixedCards         []int64          `json:"fixed_cards"`
	OrderPool          []OrderPoolEntry `json:"order_pool"`
	FirstClearRewards  []RewardConfig   `json:"first_clear_rewards"`
	RepeatRewards      []RewardConfig   `json:"repeat_rewards"`
}

// LevelGoal 定义关卡胜利目标。
type LevelGoal struct {
	GoalType string `json:"goal_type"`
	Target   int64  `json:"target"`
	Resource string `json:"resource"`
}

// OrderPoolEntry 定义订单池中的一个订单及其抽取权重。
type OrderPoolEntry struct {
	OrderID int64 `json:"order_id"`
	Weight  int   `json:"weight"`
}

type cardsFile struct {
	Cards []CardConfig `json:"cards"`
}

type ordersFile struct {
	Orders []OrderConfig `json:"orders"`
}

type levelsFile struct {
	Levels []LevelConfig `json:"levels"`
}

// LoadGameData 加载卡牌、订单和关卡配置，并和道具配置一起做交叉校验。
//
// 这里是配置进入运行时的统一入口，后续新增配置也优先挂在这里完成启动期验证。
func LoadGameData(paths ConfigPaths, items ItemCatalog) (*GameData, error) {
	cards, err := loadCards(paths.CardConfigPath)
	if err != nil {
		return nil, err
	}
	orders, err := loadOrders(paths.OrderConfigPath)
	if err != nil {
		return nil, err
	}
	levels, err := loadLevels(paths.LevelConfigPath)
	if err != nil {
		return nil, err
	}
	return NewGameData(cards, orders, levels, items)
}

// loadCards 读取卡牌配置文件。
func loadCards(path string) ([]CardConfig, error) {
	var raw cardsFile
	if err := readJSON(path, &raw); err != nil {
		return nil, fmt.Errorf("load card config: %w", err)
	}
	return raw.Cards, nil
}

// loadOrders 读取订单配置文件。
func loadOrders(path string) ([]OrderConfig, error) {
	var raw ordersFile
	if err := readJSON(path, &raw); err != nil {
		return nil, fmt.Errorf("load order config: %w", err)
	}
	return raw.Orders, nil
}

// loadLevels 读取关卡配置文件。
func loadLevels(path string) ([]LevelConfig, error) {
	var raw levelsFile
	if err := readJSON(path, &raw); err != nil {
		return nil, fmt.Errorf("load level config: %w", err)
	}
	return raw.Levels, nil
}

// readJSON 读取并反序列化 JSON 配置文件。
//
// 所有配置加载都走这个函数，保证错误信息中带有具体文件路径。
func readJSON(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return nil
}
