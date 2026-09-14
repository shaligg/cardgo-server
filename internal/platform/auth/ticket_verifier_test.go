package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingNonceStore struct {
	calls int
	ttl   time.Duration
}

func (s *recordingNonceStore) ConsumeOnce(_ context.Context, _ string, ttl time.Duration) error {
	s.calls++
	s.ttl = ttl
	return nil
}

func TestVerifierAcceptsSignedTicketOnce(t *testing.T) {
	now := time.Now().Unix()
	secret := []byte("test-ticket-secret")
	token, err := SignTicket(TicketClaims{
		UID:      "u:1",
		ServerID: "node-a",
		ExpUnix:  now + 60,
		Nonce:    "nonce-1",
		Issuer:   "login-module",
	}, secret)
	if err != nil {
		t.Fatalf("SignTicket returned error: %v", err)
	}

	verifier := Verifier{NonceStore: NewMemoryNonceStore(), Secret: secret, Issuer: "login-module"}
	claims, err := verifier.Verify(context.Background(), token, "node-a", now)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if claims.UID != "u:1" || claims.ServerID != "node-a" || claims.Nonce != "nonce-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if _, err := verifier.Verify(context.Background(), token, "node-a", now); !errors.Is(err, ErrReplay) {
		t.Fatalf("second Verify error = %v, want ErrReplay", err)
	}
}

func TestVerifierRejectsTicketSignedWithDifferentSecret(t *testing.T) {
	now := time.Now().Unix()
	token, err := SignTicket(TicketClaims{
		UID:      "u1",
		ServerID: "node-a",
		ExpUnix:  now + 60,
		Nonce:    "nonce-1",
		Issuer:   "login-module",
	}, []byte("wrong-secret"))
	if err != nil {
		t.Fatalf("SignTicket returned error: %v", err)
	}

	verifier := Verifier{Secret: []byte("expected-secret"), Issuer: "login-module"}
	if _, err := verifier.Verify(context.Background(), token, "node-a", now); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify error = %v, want ErrInvalidToken", err)
	}
}

func TestVerifierRejectsExpiredTicketBeforeConsumingNonce(t *testing.T) {
	const now int64 = 2_000_000_000
	secret := []byte("test-ticket-secret")
	for _, exp := range []int64{now, now - 1} {
		t.Run(time.Unix(exp, 0).String(), func(t *testing.T) {
			token, err := SignTicket(TicketClaims{
				UID:      "u1",
				ServerID: "node-a",
				ExpUnix:  exp,
				Nonce:    "expired-nonce",
				Issuer:   "login-module",
			}, secret)
			if err != nil {
				t.Fatal(err)
			}

			store := &recordingNonceStore{}
			verifier := Verifier{NonceStore: store, Secret: secret, Issuer: "login-module"}
			if _, err := verifier.Verify(context.Background(), token, "node-a", now); !errors.Is(err, ErrExpiredToken) {
				t.Fatalf("Verify error = %v, want ErrExpiredToken", err)
			}
			if store.calls != 0 {
				t.Fatalf("nonce store calls = %d, want 0", store.calls)
			}
		})
	}
}

func TestVerifierUsesTicketRemainingLifetimeForNonce(t *testing.T) {
	const now int64 = 2_000_000_000
	secret := []byte("test-ticket-secret")
	token, err := SignTicket(TicketClaims{
		UID:      "u1",
		ServerID: "node-a",
		ExpUnix:  now + 45,
		Nonce:    "nonce-ttl",
		Issuer:   "login-module",
	}, secret)
	if err != nil {
		t.Fatal(err)
	}

	store := &recordingNonceStore{}
	verifier := Verifier{NonceStore: store, Secret: secret, Issuer: "login-module"}
	if _, err := verifier.Verify(context.Background(), token, "node-a", now); err != nil {
		t.Fatal(err)
	}
	if store.calls != 1 || store.ttl != 45*time.Second {
		t.Fatalf("nonce store calls/ttl = %d/%s, want 1/%s", store.calls, store.ttl, 45*time.Second)
	}
}
