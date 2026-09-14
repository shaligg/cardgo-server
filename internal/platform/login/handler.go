package login

import "context"

type LoginResult struct {
	UID         string `json:"uid"`
	ServerID    string `json:"server_id"`
	WSAddr      string `json:"ws_addr"`
	EnterTicket string `json:"enter_ticket"`
	ExpireAt    int64  `json:"expire_at"`
}

type Provider interface {
	Enter(ctx context.Context, uid string, clientIP string) (LoginResult, error)
}

// LastServerRecorder 记录玩家最近一次被分配到的 GameServer。
//
// 这个记录用于重连优先回原服；记录失败不应该阻断登录主链路。
type LastServerRecorder interface {
	SaveLastServerID(ctx context.Context, uid string, serverID string) error
}

// Service 编排登录流程。
//
// 它负责调用节点分配器选择 GameServer，再签发带 server_id 的 enter_ticket。
type Service struct {
	Allocator  NodeAllocator
	Issuer     TicketIssuer
	LastServer LastServerRecorder
}

// Enter 只为已经验证的 UID 分配节点并签发入场票。
func (s Service) Enter(ctx context.Context, uid string, clientIP string) (LoginResult, error) {
	serverID, wsAddr, err := s.Allocator.Allocate(ctx, uid, clientIP)
	if err != nil {
		return LoginResult{}, err
	}
	token, expAt, err := s.Issuer.Issue(ctx, uid, serverID)
	if err != nil {
		return LoginResult{}, err
	}
	if s.LastServer != nil {
		_ = s.LastServer.SaveLastServerID(ctx, uid, serverID)
	}
	return LoginResult{
		UID:         uid,
		ServerID:    serverID,
		WSAddr:      wsAddr,
		EnterTicket: token,
		ExpireAt:    expAt,
	}, nil
}
