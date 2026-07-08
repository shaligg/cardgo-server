package gamedata

import "fmt"

// NewGameData 根据已读取的配置构建运行时索引。
//
// 构建过程中会校验重复 ID、必要字段、奖励道具引用、关卡引用的卡牌和订单是否存在。
func NewGameData(cards []CardConfig, orders []OrderConfig, levels []LevelConfig, items ItemCatalog) (*GameData, error) {
	g := &GameData{
		Cards:  map[int64]CardConfig{},
		Orders: map[int64]OrderConfig{},
		Levels: map[int64]LevelConfig{},
	}
	for _, card := range cards {
		if err := validateCard(card, items); err != nil {
			return nil, err
		}
		if _, ok := g.Cards[card.CardID]; ok {
			return nil, fmt.Errorf("duplicate card_id: %d", card.CardID)
		}
		g.Cards[card.CardID] = card
	}
	for _, order := range orders {
		if err := validateOrder(order, items); err != nil {
			return nil, err
		}
		if _, ok := g.Orders[order.OrderID]; ok {
			return nil, fmt.Errorf("duplicate order_id: %d", order.OrderID)
		}
		g.Orders[order.OrderID] = order
	}
	for _, level := range levels {
		if err := validateLevel(level, g); err != nil {
			return nil, err
		}
		if _, ok := g.Levels[level.LevelID]; ok {
			return nil, fmt.Errorf("duplicate level_id: %d", level.LevelID)
		}
		g.Levels[level.LevelID] = level
	}
	if len(g.Cards) == 0 || len(g.Orders) == 0 || len(g.Levels) == 0 {
		return nil, fmt.Errorf("game data must include cards, orders and levels")
	}
	return g, nil
}

// validateCard 校验单张卡牌配置的基础字段。
func validateCard(card CardConfig, items ItemCatalog) error {
	if card.CardID <= 0 {
		return fmt.Errorf("card_id must be positive: %+v", card)
	}
	if card.Key == "" || card.Name == "" || card.Rarity == "" || card.CardType == "" {
		return fmt.Errorf("card %d missing required fields", card.CardID)
	}
	if card.Cost < 0 {
		return fmt.Errorf("card %d cost must be non-negative", card.CardID)
	}
	if len(card.Effects) == 0 {
		return fmt.Errorf("card %d effects are required", card.CardID)
	}
	for _, effect := range card.Effects {
		if effect.EffectType == "" {
			return fmt.Errorf("card %d effect_type is required", card.CardID)
		}
	}
	for _, upgrade := range card.UpgradeCosts {
		if upgrade.TargetLevel <= 1 {
			return fmt.Errorf("card %d upgrade target_level must be greater than 1", card.CardID)
		}
		if err := validateCosts(fmt.Sprintf("card %d upgrade to level %d", card.CardID, upgrade.TargetLevel), upgrade.Costs, items); err != nil {
			return err
		}
	}
	return nil
}

// validateOrder 校验订单配置，并确保订单奖励引用了合法道具。
func validateOrder(order OrderConfig, items ItemCatalog) error {
	if order.OrderID <= 0 {
		return fmt.Errorf("order_id must be positive: %+v", order)
	}
	if order.Key == "" || order.Name == "" || order.OrderType == "" {
		return fmt.Errorf("order %d missing required fields", order.OrderID)
	}
	if len(order.Requirements) == 0 {
		return fmt.Errorf("order %d requirements are required", order.OrderID)
	}
	for _, req := range order.Requirements {
		if req.Resource == "" || req.Count <= 0 {
			return fmt.Errorf("order %d has invalid requirement", order.OrderID)
		}
	}
	return validateRewards(fmt.Sprintf("order %d", order.OrderID), order.Rewards, items)
}

// validateLevel 校验关卡配置，并确保关卡引用的卡牌和订单都已经定义。
func validateLevel(level LevelConfig, data *GameData) error {
	if level.LevelID <= 0 {
		return fmt.Errorf("level_id must be positive: %+v", level)
	}
	if level.Name == "" || level.Chapter <= 0 {
		return fmt.Errorf("level %d missing required fields", level.LevelID)
	}
	if level.TurnLimit <= 0 || level.ActionPointPerTurn <= 0 || level.OrderSlots <= 0 || level.InitialOrders <= 0 {
		return fmt.Errorf("level %d has invalid battle parameters", level.LevelID)
	}
	if level.Goal.GoalType == "" || level.Goal.Target <= 0 {
		return fmt.Errorf("level %d goal is invalid", level.LevelID)
	}
	for _, cardID := range level.FixedCards {
		if _, ok := data.Cards[cardID]; !ok {
			return fmt.Errorf("level %d references missing card_id %d", level.LevelID, cardID)
		}
	}
	if len(level.OrderPool) == 0 {
		return fmt.Errorf("level %d order_pool is required", level.LevelID)
	}
	for _, entry := range level.OrderPool {
		if _, ok := data.Orders[entry.OrderID]; !ok {
			return fmt.Errorf("level %d references missing order_id %d", level.LevelID, entry.OrderID)
		}
		if entry.Weight <= 0 {
			return fmt.Errorf("level %d order %d weight must be positive", level.LevelID, entry.OrderID)
		}
	}
	return nil
}

// validateRewards 校验奖励列表。
//
// owner 用来拼接错误信息，方便定位是哪个订单或关卡奖励配置出错。
func validateRewards(owner string, rewards []RewardConfig, items ItemCatalog) error {
	if len(rewards) == 0 {
		return fmt.Errorf("%s rewards are required", owner)
	}
	for _, reward := range rewards {
		if reward.Count <= 0 {
			return fmt.Errorf("%s reward item %d count must be positive", owner, reward.ItemID)
		}
		if _, ok := items.GetItem(reward.ItemID); !ok {
			return fmt.Errorf("%s references missing item_id %d", owner, reward.ItemID)
		}
	}
	return nil
}

// validateCosts 校验消耗列表。
func validateCosts(owner string, costs []CostConfig, items ItemCatalog) error {
	if len(costs) == 0 {
		return fmt.Errorf("%s costs are required", owner)
	}
	for _, cost := range costs {
		if cost.Count <= 0 {
			return fmt.Errorf("%s cost item %d count must be positive", owner, cost.ItemID)
		}
		if _, ok := items.GetItem(cost.ItemID); !ok {
			return fmt.Errorf("%s references missing item_id %d", owner, cost.ItemID)
		}
	}
	return nil
}
