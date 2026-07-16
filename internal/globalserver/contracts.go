// Package globalserver 定义公共服/job 编排层的最小接口契约。
//
// MVP 阶段这里不放具体结算实现，也不启动独立进程；GameServer 可以同进程调用这些接口。
// 未来需要独立 GlobalServer 时，在这些接口外层增加 transport adapter，接口语义保持稳定。
// 本包禁止依赖连接、session、在线热状态等 GameServer 私有运行时对象。
package globalserver

import "context"

// RewardItem 是公共服任务使用的奖励 DTO。
//
// 这里不直接引用 game/asset 的内部结构，避免未来 globalserver 独立部署时
// 被 GameServer 的具体实现绑死。真正发奖仍由资产服务负责执行。
type RewardItem struct {
	Type  string
	ID    string
	Count int64
}

// RankSettlementService 定义排行榜赛季结算这类公共服编排入口。
//
// MVP 阶段可以同进程调用；未来拆成独立 GlobalServer 时，在接口外层增加
// RPC/HTTP/MQ adapter，业务调用方不直接依赖传输层。
type RankSettlementService interface {
	SettleSeason(ctx context.Context, req SettleRankSeasonRequest) (SettleRankSeasonResult, error)
	GetSettlement(ctx context.Context, boardID string, seasonID string) (RankSettlement, error)
}

// BatchMailService 定义批量邮件、补偿邮件等公共批处理入口。
type BatchMailService interface {
	SendBatchMail(ctx context.Context, req SendBatchMailRequest) (SendBatchMailResult, error)
}

// GlobalJobService 定义公共服通用任务入口，用于手动触发、定时触发和失败重试。
type GlobalJobService interface {
	StartJob(ctx context.Context, req StartJobRequest) (GlobalJobResult, error)
	GetJob(ctx context.Context, jobID string) (GlobalJobResult, error)
}

// SettleRankSeasonRequest 是排行榜赛季结算请求。
type SettleRankSeasonRequest struct {
	BoardID      string
	SeasonID     string
	SettlementID string
	Operator     string
	Reason       string
}

// SettleRankSeasonResult 是排行榜赛季结算结果。
type SettleRankSeasonResult struct {
	SettlementID string
	BoardID      string
	SeasonID     string
	Status       string
	RewardCount  int64
}

// RankSettlement 描述一次排行榜结算任务的持久化状态。
type RankSettlement struct {
	BoardID      string
	SeasonID     string
	SettlementID string
	Status       string
	StartedAt    int64
	FinishedAt   int64
}

// SendBatchMailRequest 是批量邮件发送请求。
type SendBatchMailRequest struct {
	JobID   string
	UIDs    []string
	Title   string
	Content string
	Rewards []RewardItem
}

// SendBatchMailResult 是批量邮件发送结果。
type SendBatchMailResult struct {
	JobID     string
	Status    string
	SentCount int64
}

// StartJobRequest 是公共服通用任务启动请求。
type StartJobRequest struct {
	JobID   string
	JobType string
	Payload []byte
}

// GlobalJobResult 是公共服通用任务状态。
type GlobalJobResult struct {
	JobID      string
	JobType    string
	Status     string
	RetryCount int
	ErrorMsg   string
}
