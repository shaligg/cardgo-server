package login

import "context"

type LoginRequest struct {
	Account   string
	Password  string
	ClientIP  string
	ClientVer string
}

type LoginResult struct {
	UID         string `json:"uid"`
	ServerID    string `json:"server_id"`
	WSAddr      string `json:"ws_addr"`
	EnterTicket string `json:"enter_ticket"`
	ExpireAt    int64  `json:"expire_at"`
}

type Provider interface {
	LoginAndIssueTicket(ctx context.Context, req LoginRequest) (LoginResult, error)
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

// LoginAndIssueTicket 完成登录分配和 ticket 签发。
func (s Service) LoginAndIssueTicket(ctx context.Context, req LoginRequest) (LoginResult, error) {
	uid := req.Account
	serverID, wsAddr, err := s.Allocator.Allocate(ctx, uid, req.ClientIP)
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
