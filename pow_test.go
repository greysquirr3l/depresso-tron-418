package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"
)

// findValidNonce searches for a nonce in [0, maxAttempts] where sha256(seed+nonce)
// starts with the powPrefix. Returns "" if none found.
func findValidNonce(seed string, maxAttempts int) string {
	for i := 0; i <= maxAttempts; i++ {
		n := strconv.Itoa(i)
		h := sha256.Sum256([]byte(seed + n))
		if strings.HasPrefix(hex.EncodeToString(h[:]), powPrefix) {
			return n
		}
	}
	return ""
}

// seedChallenge inserts a challenge directly into the in-memory map.
func seedChallenge(sessionID, seed string, expires time.Time) {
	powMu.Lock()
	activeChallenges[sessionID] = &powChallenge{seed: seed, expiresAt: expires}
	powMu.Unlock()
}

// clearChallenge removes a session's challenge from the map.
func clearChallenge(sessionID string) {
	powMu.Lock()
	delete(activeChallenges, sessionID)
	powMu.Unlock()
}

func TestVerifyPow_ValidNonce(t *testing.T) {
	sid := "test-valid"
	seed := "aabbccdd11223344"
	nonce := findValidNonce(seed, 10_000_000)
	if nonce == "" {
		t.Fatal("could not find a valid nonce within range — test setup broken")
	}
	seedChallenge(sid, seed, time.Now().Add(10*time.Minute))
	defer clearChallenge(sid)

	if !verifyPow(sid, nonce) {
		h := sha256.Sum256([]byte(seed + nonce))
		t.Errorf("verifyPow returned false for valid nonce %s (hash %s)", nonce, hex.EncodeToString(h[:]))
	}
}

func TestVerifyPow_WrongNonce(t *testing.T) {
	sid := "test-wrong"
	seed := "deadbeefdeadbeef"
	// Find a valid nonce then use nonce+1 (which is almost certainly invalid).
	valid := findValidNonce(seed, 10_000_000)
	if valid == "" {
		t.Fatal("could not find valid nonce for setup")
	}
	n, _ := strconv.ParseInt(valid, 10, 64)
	bad := strconv.FormatInt(n+1, 10)

	seedChallenge(sid, seed, time.Now().Add(10*time.Minute))
	defer clearChallenge(sid)

	h := sha256.Sum256([]byte(seed + bad))
	if strings.HasPrefix(hex.EncodeToString(h[:]), powPrefix) {
		t.Skip("nonce+1 happens to also be valid — skip")
	}
	if verifyPow(sid, bad) {
		t.Error("verifyPow returned true for wrong nonce")
	}
}

func TestVerifyPow_NonceTooLarge(t *testing.T) {
	sid := "test-toolarge"
	seedChallenge(sid, "anyseed", time.Now().Add(10*time.Minute))
	defer clearChallenge(sid)

	if verifyPow(sid, "10000001") {
		t.Error("verifyPow returned true for nonce > 10_000_000")
	}
}

func TestVerifyPow_NegativeNonce(t *testing.T) {
	sid := "test-negative"
	seedChallenge(sid, "anyseed", time.Now().Add(10*time.Minute))
	defer clearChallenge(sid)

	if verifyPow(sid, "-1") {
		t.Error("verifyPow returned true for negative nonce")
	}
}

func TestVerifyPow_NonNumericNonce(t *testing.T) {
	sid := "test-nonnumeric"
	seedChallenge(sid, "anyseed", time.Now().Add(10*time.Minute))
	defer clearChallenge(sid)

	if verifyPow(sid, "cafe") {
		t.Error("verifyPow returned true for non-numeric nonce")
	}
	if verifyPow(sid, "") {
		t.Error("verifyPow returned true for empty nonce")
	}
}

func TestVerifyPow_ExpiredChallenge(t *testing.T) {
	sid := "test-expired"
	seed := "expiredseed00000"
	nonce := findValidNonce(seed, 10_000_000)
	if nonce == "" {
		t.Fatal("could not find valid nonce for setup")
	}
	// Set expiry in the past.
	seedChallenge(sid, seed, time.Now().Add(-1*time.Second))
	defer clearChallenge(sid)

	if verifyPow(sid, nonce) {
		t.Error("verifyPow returned true for expired challenge")
	}
}

func TestVerifyPow_MissingSession(t *testing.T) {
	clearChallenge("no-such-session")
	if verifyPow("no-such-session", "12345") {
		t.Error("verifyPow returned true for non-existent session")
	}
}

func TestPowPrefixIsCorrect(t *testing.T) {
	if powPrefix != "cafe" {
		t.Errorf("powPrefix = %q, want %q", powPrefix, "cafe")
	}
}
