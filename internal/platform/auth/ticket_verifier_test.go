package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

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
