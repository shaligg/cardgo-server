// Package battle 提供 MVP 关卡局内运行时。
//
// 当前实现把局内状态放在 GameServer 内存中，结算奖励统一通过 asset.Service 落库。
package battle

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bigfish/go_orm_1/internal/domain/asset"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	// ErrInvalidReqID 表示需要幂等请求 ID 的操作没有传入 req_id。
	ErrInvalidReqID = errors.New("invalid req_id")
	// ErrGameDataMissing 表示关卡运行依赖的策划配置缺失。
	ErrGameDataMissing = errors.New("game data is missing")
	// ErrLevelNotFound 表示请求的关卡不存在。
	ErrLevelNotFound = errors.New("level not found")
	// ErrSessionNotFound 表示局内会话不存在或不属于当前玩家。
	ErrSessionNotFound = errors.New("level session not found")
	// ErrCardNotFound 表示请求打出的卡牌不存在。
	ErrCardNotFound = errors.New("card not found")
	// ErrCardNotInSession 表示玩家当前关卡不可使用该卡牌。
	ErrCardNotInSession = errors.New("card not in level session")
	// ErrInsufficientResource 表示资源转换时局内资源不足。
	ErrInsufficientResource = errors.New("insufficient battle resource")
	// ErrInsufficientActionPoint 表示当前回合行动点不足，不能打出卡牌。
	ErrInsufficientActionPoint = errors.New("insufficient action point")
	// ErrLevelNotComplete 表示关卡目标尚未完成，不能结算。
	ErrLevelNotComplete = errors.New("level goal not complete")
	// ErrLevelAlreadyComplete 表示关卡目标已经完成，应直接发起结算。
	ErrLevelAlreadyComplete = errors.New("level goal already complete")
	// ErrLevelFailed 表示关卡已经失败，不能继续操作或成功结算。
	ErrLevelFailed = errors.New("level failed")
	// ErrBattleInProgress 表示玩家已有一局尚未结算的关卡。
	ErrBattleInProgress = errors.New("battle already in progress")
)

// OrderState 是局内展示的订单状态。
type OrderState struct {
	OrderID      int64                     `json:"order_id"`
	Name         string                    `json:"name"`
	OrderType    string                    `json:"order_type"`
	Requirements []gamedata.ResourceAmount `json:"requirements"`
	Completed    bool                      `json:"completed"`
}

// LevelSession 是客户端可见的关卡运行时快照。
type LevelSession struct {
	SessionID       string           `json:"level_session_id"`
	UID             string           `json:"uid"`
	LevelID         int64            `json:"level_id"`
	Turn            int              `json:"turn"`
	TurnLimit       int              `json:"turn_limit"`
	ActionPoint     int              `json:"action_point"`
	GoalType        string           `json:"goal_type"`
	GoalTarget      int64            `json:"goal_target"`
	CompletedOrders int64            `json:"completed_orders"`
	Resources       map[string]int64 `json:"resources"`
	HandCards       []int64          `json:"hand_cards"`
	ActiveOrders    []OrderState     `json:"active_orders"`
	Settled         bool             `json:"settled"`
	Failed          bool             `json:"failed"`
}

// PlayCardResult 是打出卡牌后的局内状态。
type PlayCardResult struct {
	Session LevelSession `json:"session"`
	CardID  int64        `json:"card_id"`
}

// LevelSettleResult 是关卡结算结果。
type LevelSettleResult struct {
	OK              bool                     `json:"ok"`
	SessionID       string                   `json:"level_session_id"`
	LevelID         int64                    `json:"level_id"`
	CompletedOrders int64                    `json:"completed_orders"`
	FirstClear      bool                     `json:"first_clear"`
	Rewards         []asset.RewardItem       `json:"rewards"`
	Progress        repo.PlayerLevelProgress `json:"progress"`
	Player          *repo.Player             `json:"-"`
}

// Service 是关卡运行时服务。
type Service struct {
	Data     *gamedata.GameData
	Assets   asset.Service
	Tx       idb.TxManager
	Progress repo.LevelProgressRepository

	mu       sync.RWMutex
	sessions map[string]*runtimeSession
}

type runtimeSession struct {
	mu             sync.Mutex
	deleted        bool
	state          LevelSession
	level          gamedata.LevelConfig
	nextOrderIndex int
	pendingRewards []asset.RewardItem
	settleResult   *LevelSettleResult
}

// StartLevel 创建一个新的内存关卡会话。
func (s *Service) StartLevel(ctx context.Context, uid string, levelID int64, reqID string) (LevelSession, error) {
	_ = ctx
	if reqID == "" {
		return LevelSession{}, ErrInvalidReqID
	}
	if s.Data == nil {
		return LevelSession{}, ErrGameDataMissing
	}

	level, ok := s.Data.Levels[levelID]
	if !ok {
		return LevelSession{}, fmt.Errorf("%w: %d", ErrLevelNotFound, levelID)
	}

	rs := &runtimeSession{
		level:          level,
		nextOrderIndex: 0,
		state: LevelSession{
			SessionID:   uuid.NewString(),
			UID:         uid,
			LevelID:     levelID,
			Turn:        1,
			TurnLimit:   level.TurnLimit,
			ActionPoint: level.ActionPointPerTurn,
			GoalType:    level.Goal.GoalType,
			GoalTarget:  level.Goal.Target,
			Resources:   map[string]int64{},
			HandCards:   append([]int64(nil), level.FixedCards...),
		},
	}
	for i := 0; i < level.InitialOrders; i++ {
		rs.state.ActiveOrders = append(rs.state.ActiveOrders, s.nextOrder(rs))
	}

	if err := s.storeSession(uid, rs); err != nil {
		return LevelSession{}, err
	}
	return cloneSession(rs.state), nil
}

// PlayCard 执行一张卡牌的 MVP 效果，并尝试自动完成满足条件的订单。
func (s *Service) PlayCard(ctx context.Context, uid string, sessionID string, cardID int64, reqID string) (PlayCardResult, error) {
	_ = ctx
	if reqID == "" {
		return PlayCardResult{}, ErrInvalidReqID
	}
	rs, err := s.lockSession(uid, sessionID)
	if err != nil {
		return PlayCardResult{}, err
	}
	defer rs.mu.Unlock()
	if s.Data == nil {
		return PlayCardResult{}, ErrGameDataMissing
	}
	card, ok := s.Data.Cards[cardID]
	if !ok {
		return PlayCardResult{}, fmt.Errorf("%w: %d", ErrCardNotFound, cardID)
	}
	if !containsCard(rs.state.HandCards, cardID) {
		return PlayCardResult{}, fmt.Errorf("%w: %d", ErrCardNotInSession, cardID)
	}
	if rs.state.Settled {
		return PlayCardResult{Session: cloneSession(rs.state), CardID: cardID}, nil
	}
	if rs.state.Failed {
		return PlayCardResult{}, ErrLevelFailed
	}
	if card.Cost > rs.state.ActionPoint {
		return PlayCardResult{}, ErrInsufficientActionPoint
	}
	nextState := cloneSession(rs.state)
	nextState.ActionPoint -= card.Cost
	if err := applyCardEffects(&nextState, card); err != nil {
		return PlayCardResult{}, err
	}
	rs.state = nextState
	s.completeReadyOrders(rs)
	return PlayCardResult{Session: cloneSession(rs.state), CardID: cardID}, nil
}

// EndTurn 结束当前回合。普通回合进入下一回合并恢复行动点，最后一回合结束时判定失败。
func (s *Service) EndTurn(ctx context.Context, uid string, sessionID string, reqID string) (LevelSession, error) {
	_ = ctx
	if reqID == "" {
		return LevelSession{}, ErrInvalidReqID
	}
	rs, err := s.lockSession(uid, sessionID)
	if err != nil {
		return LevelSession{}, err
	}
	defer rs.mu.Unlock()

	if rs.state.Settled {
		return cloneSession(rs.state), nil
	}
	if rs.state.Failed {
		return cloneSession(rs.state), nil
	}
	if rs.state.CompletedOrders >= rs.level.Goal.Target {
		return LevelSession{}, ErrLevelAlreadyComplete
	}
	if rs.state.Turn >= rs.level.TurnLimit {
		rs.state.Failed = true
		rs.state.ActionPoint = 0
		return cloneSession(rs.state), nil
	}

	rs.state.Turn++
	rs.state.ActionPoint = rs.level.ActionPointPerTurn
	return cloneSession(rs.state), nil
}

// SettleLevel 结算关卡并发放奖励。
//
// 同一个内存会话只会发奖一次，重复调用会返回首次结算结果。
func (s *Service) SettleLevel(ctx context.Context, uid string, sessionID string, reqID string) (LevelSettleResult, error) {
	if reqID == "" {
		return LevelSettleResult{}, ErrInvalidReqID
	}

	rs, err := s.lockSession(uid, sessionID)
	if err != nil {
		return LevelSettleResult{}, err
	}
	defer rs.mu.Unlock()
	if rs.settleResult != nil {
		return *rs.settleResult, nil
	}
	if rs.state.Failed {
		return LevelSettleResult{}, ErrLevelFailed
	}
	if rs.state.CompletedOrders < rs.level.Goal.Target {
		return LevelSettleResult{}, ErrLevelNotComplete
	}

	result := LevelSettleResult{
		OK:              true,
		SessionID:       rs.state.SessionID,
		LevelID:         rs.state.LevelID,
		CompletedOrders: rs.state.CompletedOrders,
	}

	var changes []asset.ChangeResult
	if err := s.Tx.Do(ctx, func(tx *gorm.DB) error {
		if s.Progress == nil {
			return fmt.Errorf("level progress repository is nil")
		}
		progress, err := s.Progress.GetLevelProgressInTx(ctx, tx, uid, rs.state.LevelID)
		if err != nil && !errors.Is(err, repo.ErrLevelProgressNotFound) {
			return err
		}
		now := time.Now().Unix()
		result.FirstClear = errors.Is(err, repo.ErrLevelProgressNotFound)
		if result.FirstClear {
			progress = repo.PlayerLevelProgress{
				UID:            uid,
				LevelID:        rs.state.LevelID,
				FirstClearedAt: now,
			}
		}
		progress.ClearCount++
		progress.LastClearedAt = now
		if result.FirstClear {
			err = s.Progress.CreateLevelProgressInTx(ctx, tx, progress)
		} else {
			err = s.Progress.UpdateLevelProgressInTx(ctx, tx, progress)
		}
		if err != nil {
			return err
		}
		result.Progress = progress
		result.Rewards = buildSettleRewards(rs, result.FirstClear)
		if len(result.Rewards) == 0 {
			return nil
		}
		changes, err = s.Assets.ApplyRewardInTx(ctx, tx, uid, result.Rewards, "level.settle", reqID)
		return err
	}); err != nil {
		return LevelSettleResult{}, err
	}
	if len(changes) > 0 {
		for _, change := range changes {
			if change.Player != nil {
				player := *change.Player
				result.Player = &player
			}
		}
	}
	rs.state.Settled = true
	rs.settleResult = &result
	return result, nil
}

// buildSettleRewards 合并本局订单奖励，并按进度选择首通或重复通关奖励。
func buildSettleRewards(rs *runtimeSession, firstClear bool) []asset.RewardItem {
	rewards := append([]asset.RewardItem(nil), rs.pendingRewards...)
	configured := rs.level.RepeatRewards
	if firstClear {
		configured = rs.level.FirstClearRewards
	}
	for _, reward := range configured {
		rewards = append(rewards, asset.RewardItem{ItemID: reward.ItemID, Count: reward.Count})
	}
	return rewards
}

func (s *Service) initLocked() {
	if s.sessions == nil {
		s.sessions = map[string]*runtimeSession{}
	}
}

// storeSession 保存玩家的新关卡。未结算的旧关卡不能被覆盖，已结算关卡由新关卡替换。
func (s *Service) storeSession(uid string, next *runtimeSession) error {
	for {
		s.mu.RLock()
		current := s.sessions[uid]
		s.mu.RUnlock()
		if current == nil {
			s.mu.Lock()
			s.initLocked()
			if s.sessions[uid] == nil {
				s.sessions[uid] = next
				s.mu.Unlock()
				return nil
			}
			s.mu.Unlock()
			continue
		}

		current.mu.Lock()
		if current.deleted {
			current.mu.Unlock()
			continue
		}
		if !current.state.Settled && !current.state.Failed {
			current.mu.Unlock()
			return ErrBattleInProgress
		}

		s.mu.Lock()
		if s.sessions[uid] != current {
			s.mu.Unlock()
			current.mu.Unlock()
			continue
		}
		current.deleted = true
		s.sessions[uid] = next
		s.mu.Unlock()
		current.mu.Unlock()
		return nil
	}
}

// lockSession 获取并锁定单个关卡会话；调用方必须在返回成功后解锁 rs.mu。
func (s *Service) lockSession(uid string, sessionID string) (*runtimeSession, error) {
	s.mu.RLock()
	rs := s.sessions[uid]
	s.mu.RUnlock()
	if rs == nil {
		return nil, ErrSessionNotFound
	}
	rs.mu.Lock()
	if rs.deleted || rs.state.SessionID != sessionID {
		rs.mu.Unlock()
		return nil, ErrSessionNotFound
	}
	return rs, nil
}

// PlayerUIDs 返回当前节点仍保存局内状态的玩家 UID。
func (s *Service) PlayerUIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.sessions))
	for uid := range s.sessions {
		if uid == "" {
			continue
		}
		out = append(out, uid)
	}
	return out
}

// DeletePlayerRuntime 删除指定玩家在当前节点的关卡运行时。
func (s *Service) DeletePlayerRuntime(uid string) int {
	s.mu.RLock()
	runtime := s.sessions[uid]
	s.mu.RUnlock()
	if runtime == nil {
		return 0
	}

	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[uid] != runtime {
		return 0
	}
	runtime.deleted = true
	delete(s.sessions, uid)
	return 1
}

func (s *Service) nextOrder(rs *runtimeSession) OrderState {
	if len(rs.level.OrderPool) == 0 || s.Data == nil {
		return OrderState{}
	}
	entry := rs.level.OrderPool[rs.nextOrderIndex%len(rs.level.OrderPool)]
	rs.nextOrderIndex++
	order := s.Data.Orders[entry.OrderID]
	return OrderState{
		OrderID:      order.OrderID,
		Name:         order.Name,
		OrderType:    order.OrderType,
		Requirements: append([]gamedata.ResourceAmount(nil), order.Requirements...),
	}
}

func (s *Service) completeReadyOrders(rs *runtimeSession) {
	for {
		completedAny := false
		for i := range rs.state.ActiveOrders {
			order := &rs.state.ActiveOrders[i]
			if order.Completed || !canComplete(rs.state.Resources, order.Requirements) {
				continue
			}
			consumeRequirements(rs.state.Resources, order.Requirements)
			order.Completed = true
			rs.state.CompletedOrders++
			if cfg, ok := s.Data.Orders[order.OrderID]; ok {
				for _, reward := range cfg.Rewards {
					rs.pendingRewards = append(rs.pendingRewards, asset.RewardItem{ItemID: reward.ItemID, Count: reward.Count})
				}
			}
			if rs.state.CompletedOrders < rs.level.Goal.Target {
				rs.state.ActiveOrders = append(rs.state.ActiveOrders, s.nextOrder(rs))
			}
			completedAny = true
		}
		if !completedAny {
			return
		}
	}
}

func applyCardEffects(state *LevelSession, card gamedata.CardConfig) error {
	for _, effect := range card.Effects {
		switch effect.EffectType {
		case "gain_resource", "periodic_gain":
			state.Resources[effect.Resource] += effect.Value
		case "convert_resource":
			if state.Resources[effect.Resource] < effect.Value {
				return fmt.Errorf("%w: %s", ErrInsufficientResource, effect.Resource)
			}
			state.Resources[effect.Resource] -= effect.Value
			state.Resources[effect.ToResource] += effect.ToValue
		case "copy_resource", "cost_reduce", "draw_card", "order_reward_bonus", "refresh_order", "resource_bonus":
			// 这些效果先在 MVP 中接受配置但不展开复杂行为，后续随关卡规则逐步实现。
		default:
			// 未知效果在配置校验阶段暂不拦截，运行时先保持兼容，避免阻塞策划试配。
		}
	}
	return nil
}

func canComplete(resources map[string]int64, requirements []gamedata.ResourceAmount) bool {
	for _, req := range requirements {
		if resources[req.Resource] < req.Count {
			return false
		}
	}
	return true
}

func consumeRequirements(resources map[string]int64, requirements []gamedata.ResourceAmount) {
	for _, req := range requirements {
		resources[req.Resource] -= req.Count
	}
}

func containsCard(cards []int64, cardID int64) bool {
	for _, id := range cards {
		if id == cardID {
			return true
		}
	}
	return false
}

func cloneSession(in LevelSession) LevelSession {
	out := in
	out.Resources = map[string]int64{}
	for k, v := range in.Resources {
		out.Resources[k] = v
	}
	out.HandCards = append([]int64(nil), in.HandCards...)
	out.ActiveOrders = append([]OrderState(nil), in.ActiveOrders...)
	return out
}
