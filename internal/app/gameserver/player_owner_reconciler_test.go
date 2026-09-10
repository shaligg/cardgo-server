package gameserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/platform/session"
)

type fakePlayerOwnerStore struct {
	owners        map[string]session.PlayerOwner
	getErr        error
	refreshedUIDs []string
}

func (s *fakePlayerOwnerStore) Claim(ctx context.Context, uid string, serverID string, connID string, ttl time.Duration) (session.PlayerOwner, error) {
	return session.PlayerOwner{}, nil
}

func (s *fakePlayerOwnerStore) MarkOffline(ctx context.Context, uid string, serverID string, connID string, ttl time.Duration) error {
	return nil
}

func (s *fakePlayerOwnerStore) GetLastServerID(ctx context.Context, uid string) (string, bool, error) {
	owner, ok := s.owners[uid]
	return owner.ServerID, ok, nil
}

func (s *fakePlayerOwnerStore) GetOwners(ctx context.Context, uids []string) (map[string]session.PlayerOwner, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	out := make(map[string]session.PlayerOwner, len(uids))
	for _, uid := range uids {
		if owner, ok := s.owners[uid]; ok {
			out[uid] = owner
		}
	}
	return out, nil
}

func (s *fakePlayerOwnerStore) RefreshOwned(ctx context.Context, serverID string, uids []string, ttl time.Duration) error {
	s.refreshedUIDs = append([]string(nil), uids...)
	return nil
}

func TestPlayerOwnerReconcilerRefreshesActiveOwner(t *testing.T) {
	sessions := session.NewMemoryManager()
	_, accepted, err := sessions.BindWithinLimit(context.Background(), session.Session{UID: "u1", ConnID: "conn-a"}, 10)
	if err != nil || !accepted {
		t.Fatalf("bind session: accepted=%v err=%v", accepted, err)
	}
	owners := &fakePlayerOwnerStore{owners: map[string]session.PlayerOwner{
		"u1": {UID: "u1", ServerID: "gs-a", ConnID: "conn-a"},
	}}
	reconciler := &playerOwnerReconciler{nodeID: "gs-a", ownerTTL: time.Minute, owners: owners, sessions: sessions}

	reconciler.ReconcileOwners(context.Background())

	if len(owners.refreshedUIDs) != 1 || owners.refreshedUIDs[0] != "u1" {
		t.Fatalf("refreshed uids = %v, want [u1]", owners.refreshedUIDs)
	}
}

func TestPlayerOwnerReconcilerKeepsActiveSessionWhenRedisFails(t *testing.T) {
	sessions := session.NewMemoryManager()
	_, accepted, err := sessions.BindWithinLimit(context.Background(), session.Session{UID: "u1", ConnID: "conn-a"}, 10)
	if err != nil || !accepted {
		t.Fatalf("bind session: accepted=%v err=%v", accepted, err)
	}
	reconciler := &playerOwnerReconciler{
		nodeID:   "gs-a",
		owners:   &fakePlayerOwnerStore{getErr: errors.New("redis unavailable")},
		sessions: sessions,
	}

	reconciler.ReconcileOwners(context.Background())

	if current, ok, getErr := sessions.GetByUID(context.Background(), "u1"); getErr != nil || !ok || current.ConnID != "conn-a" {
		t.Fatalf("active session changed after Redis failure: session=%+v ok=%v err=%v", current, ok, getErr)
	}
}

func TestPlayerOwnerReconcilerKeepsActiveSessionWhenOwnerMissing(t *testing.T) {
	sessions := session.NewMemoryManager()
	_, accepted, err := sessions.BindWithinLimit(context.Background(), session.Session{UID: "u1", ConnID: "conn-a"}, 10)
	if err != nil || !accepted {
		t.Fatalf("bind session: accepted=%v err=%v", accepted, err)
	}
	reconciler := &playerOwnerReconciler{
		nodeID:   "gs-a",
		owners:   &fakePlayerOwnerStore{owners: map[string]session.PlayerOwner{}},
		sessions: sessions,
	}

	reconciler.ReconcileOwners(context.Background())

	if current, ok, getErr := sessions.GetByUID(context.Background(), "u1"); getErr != nil || !ok || current.ConnID != "conn-a" {
		t.Fatalf("active session changed when owner was missing: session=%+v ok=%v err=%v", current, ok, getErr)
	}
}
