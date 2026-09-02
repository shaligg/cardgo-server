package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	ilog "github.com/bigfish/go_orm_1/internal/infra/log"
	goredis "github.com/redis/go-redis/v9"
)

const (
	// PlayerKickTargetConnection 只关闭指定连接，用于跨节点顶号。
	PlayerKickTargetConnection = "connection"
	// PlayerKickTargetUID 关闭该 UID 在目标节点的当前连接，用于主动踢人。
	PlayerKickTargetUID = "uid"
	// PlayerKickTargetAll 关闭所有 GameServer 的全部当前连接，用于全服广播踢人。
	PlayerKickTargetAll = "all"
)

// PlayerKickNotice 要求目标 GameServer 关闭指定连接或玩家当前连接。
type PlayerKickNotice struct {
	Target string `json:"target"`
	UID    string `json:"uid"`
	ConnID string `json:"conn_id,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// PlayerKickBus 使用 Redis Pub/Sub 发送跨节点顶号通知。
// Redis 玩家归属仍是权威状态，定时归属扫描负责兜底丢失的通知。
type PlayerKickBus struct {
	client *Client
	prefix string

	mu     sync.Mutex
	cancel context.CancelFunc
	pubsub *goredis.PubSub
	wg     sync.WaitGroup
}

// NewPlayerKickBus 创建玩家顶号通知总线。
func NewPlayerKickBus(client *Client, prefix string) *PlayerKickBus {
	return &PlayerKickBus{client: client, prefix: prefix}
}

// Publish 向旧玩家所在节点发送精确到连接 ID 的顶号通知。
func (b *PlayerKickBus) Publish(ctx context.Context, serverID string, notice PlayerKickNotice) error {
	if err := b.validate(serverID, notice); err != nil {
		return err
	}
	return b.publish(ctx, b.channel(serverID), notice)
}

// Broadcast 向所有 GameServer 广播踢出全部当前连接的通知。
func (b *PlayerKickBus) Broadcast(ctx context.Context, notice PlayerKickNotice) error {
	if err := b.ready(); err != nil {
		return err
	}
	if notice.Target != PlayerKickTargetAll {
		return fmt.Errorf("broadcast player kick target must be %s", PlayerKickTargetAll)
	}
	if err := validatePlayerKickNotice(notice); err != nil {
		return err
	}
	return b.publish(ctx, b.broadcastChannel(), notice)
}

func (b *PlayerKickBus) publish(ctx context.Context, channel string, notice PlayerKickNotice) error {
	payload, err := json.Marshal(notice)
	if err != nil {
		return fmt.Errorf("marshal player kick notice: %w", err)
	}
	if err := b.client.raw.Publish(ctx, channel, payload).Err(); err != nil {
		return fmt.Errorf("publish player kick channel=%s uid=%s: %w", channel, notice.UID, err)
	}
	return nil
}

// Start 订阅当前 GameServer 的顶号频道；同一个实例只能启动一次。
func (b *PlayerKickBus) Start(ctx context.Context, serverID string, handler func(PlayerKickNotice)) error {
	if err := b.ready(); err != nil {
		return err
	}
	if serverID == "" || handler == nil {
		return fmt.Errorf("invalid player kick subscriber server=%s", serverID)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pubsub != nil {
		return fmt.Errorf("player kick subscriber already started")
	}
	subCtx, cancel := context.WithCancel(ctx)
	pubsub := b.client.raw.Subscribe(subCtx, b.channel(serverID), b.broadcastChannel())
	if _, err := pubsub.Receive(subCtx); err != nil {
		cancel()
		_ = pubsub.Close()
		return fmt.Errorf("subscribe player kick server=%s: %w", serverID, err)
	}
	b.cancel = cancel
	b.pubsub = pubsub
	b.wg.Add(1)
	go b.consume(subCtx, pubsub.Channel(), handler)
	return nil
}

// Stop 停止接收顶号通知。
func (b *PlayerKickBus) Stop() error {
	b.mu.Lock()
	cancel := b.cancel
	pubsub := b.pubsub
	b.cancel = nil
	b.pubsub = nil
	b.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	var err error
	if pubsub != nil {
		err = pubsub.Close()
	}
	b.wg.Wait()
	return err
}

func (b *PlayerKickBus) consume(ctx context.Context, messages <-chan *goredis.Message, handler func(PlayerKickNotice)) {
	defer b.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messages:
			if !ok {
				return
			}
			var notice PlayerKickNotice
			if err := json.Unmarshal([]byte(message.Payload), &notice); err != nil || validatePlayerKickNotice(notice) != nil {
				ilog.Errorf("ignore invalid player kick notice channel=%s", message.Channel)
				continue
			}
			handler(notice)
		}
	}
}

func (b *PlayerKickBus) validate(serverID string, notice PlayerKickNotice) error {
	if err := b.ready(); err != nil {
		return err
	}
	if serverID == "" {
		return fmt.Errorf("invalid player kick notice server=%s uid=%s conn=%s", serverID, notice.UID, notice.ConnID)
	}
	return validatePlayerKickNotice(notice)
}

func validatePlayerKickNotice(notice PlayerKickNotice) error {
	if notice.Target == PlayerKickTargetAll {
		return nil
	}
	if notice.UID == "" {
		return fmt.Errorf("player kick uid is empty")
	}
	switch notice.Target {
	case PlayerKickTargetConnection:
		if notice.ConnID == "" {
			return fmt.Errorf("player kick conn_id is empty")
		}
	case PlayerKickTargetUID:
		// 主动踢人只按 UID 查找目标节点的当前连接，不使用 conn_id。
	default:
		return fmt.Errorf("invalid player kick target: %s", notice.Target)
	}
	return nil
}

func (b *PlayerKickBus) ready() error {
	if b == nil || b.client == nil || b.client.raw == nil || b.prefix == "" {
		return fmt.Errorf("redis player kick bus is not initialized")
	}
	return nil
}

func (b *PlayerKickBus) channel(serverID string) string {
	return b.prefix + ":" + serverID
}

func (b *PlayerKickBus) broadcastChannel() string {
	return b.prefix + ":all"
}
