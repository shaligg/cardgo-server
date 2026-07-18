package globalcore

import "context"

// RewardItem 是公共领域邮件附件使用的奖励 DTO。
//
// 这里不复用 game/asset 的结构，避免公共领域接口被 GameServer 具体资产实现绑死。
type RewardItem struct {
	Type  string
	ID    string
	Count int64
}

// MailItem 是邮件列表返回的轻量 DTO。
type MailItem struct {
	MailID  string
	Title   string
	Content string
	TS      int64
}

// MailService 定义邮件公共领域能力。
//
// 邮件常用于补偿、排行榜奖励和离线通知，发送与领取都必须由调用方显式传入 reqID，
// 以便 LocalService 和未来 RemoteClient 使用同一套幂等语义。
type MailService interface {
	SendSystemMail(ctx context.Context, uid string, title string, content string, rewards []RewardItem, reqID string) error
	List(ctx context.Context, uid string, cursor string, limit int) ([]MailItem, string, error)
	Claim(ctx context.Context, uid string, mailID string, reqID string) error
}
