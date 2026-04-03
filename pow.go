package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CaffeineChain™ — Proof-of-Work verification layer.
//
// Before the barista will accept a bean description, the client browser must
// demonstrate commitment by finding a nonce N such that:
//
//	hex(sha256(seed + N)) has prefix "cafe"
//
// This burns approximately 5–30 seconds of CPU. This is intentional.
// The name "CaffeineChain" does not imply any blockchain technology, financial
// instrument, or useful computation. It implies bureaucracy.
const powPrefix = "cafe"

type powChallenge struct {
	seed      string
	expiresAt time.Time
}

var (
	powMu            sync.Mutex
	activeChallenges = map[string]*powChallenge{}
)

// issuePowChallenge generates a fresh challenge for the given session.
func issuePowChallenge(ctx context.Context, sessionID string) (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	seed := hex.EncodeToString(b)

	powMu.Lock()
	activeChallenges[sessionID] = &powChallenge{
		seed:      seed,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	powMu.Unlock()

	if err := setPowChallenge(ctx, sessionID, seed); err != nil {
		return "", err
	}
	return seed, nil
}

// verifyPow checks that the client's nonce solves the CaffeineChain challenge.
func verifyPow(sessionID, nonce string) bool {
	powMu.Lock()
	challenge, ok := activeChallenges[sessionID]
	powMu.Unlock()

	if !ok || time.Now().After(challenge.expiresAt) {
		return false
	}

	// Guard against absurdly large nonces used as a DoS vector.
	n, err := strconv.ParseInt(nonce, 10, 64)
	if err != nil || n < 0 || n > 10_000_000 {
		return false
	}

	h := sha256.Sum256([]byte(challenge.seed + nonce))
	return strings.HasPrefix(hex.EncodeToString(h[:]), powPrefix)
}
