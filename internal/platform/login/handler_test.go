package login

import (
	"context"
	"errors"
	"testing"
)

type fakeAllocator struct {
	serverID string
	wsAddr   string
	err      error
}

func (a fakeAllocator) Allocate(ctx context.Context, uid string, clientIP string) (string, string, error) {
	_ = ctx
	_ = uid
	_ = clientIP
	if a.err != nil {
		return "", "", a.err
	}
	return a.serverID, a.wsAddr, nil
}

type fakeIssuer struct {
	token string
	expAt int64
	err   error
}

func (i fakeIssuer) Issue(ctx context.Context, uid string, serverID string) (string, int64, error) {
	_ = ctx
	_ = uid
	_ = serverID
	if i.err != nil {
		return "", 0, i.err
	}
	return i.token, i.expAt, nil
}

type fakeLastServerRecorder struct {
	uid      string
	serverID string
	err      error
}

func (r *fakeLastServerRecorder) SaveLastServerID(ctx context.Context, uid string, serverID string) error {
	_ = ctx
	r.uid = uid
	r.serverID = serverID
	return r.err
}

func TestLoginAndIssueTicketRecordsLastServer(t *testing.T) {
	recorder := &fakeLastServerRecorder{}
	svc := Service{
		Allocator:  fakeAllocator{serverID: "gs-a", wsAddr: "ws://gs-a/ws"},
		Issuer:     fakeIssuer{token: "ticket-a", expAt: 123},
		LastServer: recorder,
	}

	result, err := svc.LoginAndIssueTicket(context.Background(), LoginRequest{Account: "u1"})
	if err != nil {
		t.Fatalf("LoginAndIssueTicket returned error: %v", err)
	}
	if result.UID != "u1" || result.ServerID != "gs-a" || result.WSAddr != "ws://gs-a/ws" || result.EnterTicket != "ticket-a" {
		t.Fatalf("unexpected login result: %+v", result)
	}
	if recorder.uid != "u1" || recorder.serverID != "gs-a" {
		t.Fatalf("expected recorder to save u1/gs-a, got %s/%s", recorder.uid, recorder.serverID)
	}
}

func TestLoginAndIssueTicketIgnoresLastServerRecordError(t *testing.T) {
	svc := Service{
		Allocator:  fakeAllocator{serverID: "gs-a", wsAddr: "ws://gs-a/ws"},
		Issuer:     fakeIssuer{token: "ticket-a", expAt: 123},
		LastServer: &fakeLastServerRecorder{err: errors.New("record failed")},
	}

	result, err := svc.LoginAndIssueTicket(context.Background(), LoginRequest{Account: "u1"})
	if err != nil {
		t.Fatalf("LoginAndIssueTicket should ignore recorder error, got %v", err)
	}
	if result.ServerID != "gs-a" {
		t.Fatalf("expected login result even when recorder fails, got %+v", result)
	}
}

func TestLoginAndIssueTicketReturnsAllocatorError(t *testing.T) {
	wantErr := errors.New("allocator failed")
	svc := Service{
		Allocator: fakeAllocator{err: wantErr},
		Issuer:    fakeIssuer{token: "ticket-a", expAt: 123},
	}

	_, err := svc.LoginAndIssueTicket(context.Background(), LoginRequest{Account: "u1"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected allocator error, got %v", err)
	}
}

func TestLoginAndIssueTicketReturnsIssuerError(t *testing.T) {
	wantErr := errors.New("issuer failed")
	svc := Service{
		Allocator: fakeAllocator{serverID: "gs-a", wsAddr: "ws://gs-a/ws"},
		Issuer:    fakeIssuer{err: wantErr},
	}

	_, err := svc.LoginAndIssueTicket(context.Background(), LoginRequest{Account: "u1"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected issuer error, got %v", err)
	}
}
