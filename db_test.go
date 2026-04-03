package main

import (
	"context"
	"database/sql"
	"testing"
)

// freshDB opens an in-memory SQLite database, runs the schema, replaces the
// package-level `database` variable, and registers cleanup to restore it.
func freshDB(t *testing.T) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open :memory: DB: %v", err)
	}
	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS sessions (
			id               TEXT    PRIMARY KEY,
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			permit_approved  INTEGER  NOT NULL DEFAULT 0,
			beans_approved   INTEGER  NOT NULL DEFAULT 0,
			pow_solved       INTEGER  NOT NULL DEFAULT 0,
			brew_started     INTEGER  NOT NULL DEFAULT 0,
			rejection_count  INTEGER  NOT NULL DEFAULT 0,
			when_window_at   DATETIME,
			pow_challenge    TEXT     NOT NULL DEFAULT '',
			gemini_key       TEXT     NOT NULL DEFAULT ''
		);
	`)
	if err != nil {
		t.Fatalf("DB schema: %v", err)
	}
	prev := database
	database = db
	t.Cleanup(func() {
		_ = db.Close()
		database = prev
	})
}

func TestCreateAndGetSession(t *testing.T) {
	freshDB(t)
	s, err := createSession(context.Background())
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	if s.ID == "" {
		t.Error("session ID is empty")
	}

	got, err := getSession(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("getSession: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("ID = %q, want %q", got.ID, s.ID)
	}
	if got.PermitApproved || got.BeansApproved || got.PowSolved || got.BrewStarted {
		t.Error("new session should have all flags false")
	}
	if got.RejectionCount != 0 {
		t.Errorf("RejectionCount = %d, want 0", got.RejectionCount)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	freshDB(t)
	_, err := getSession(context.Background(), "does-not-exist")
	if err == nil {
		t.Error("expected error for missing session, got nil")
	}
}

func TestApprovePermit(t *testing.T) {
	freshDB(t)
	s, _ := createSession(context.Background())
	if err := approvePermit(context.Background(), s.ID); err != nil {
		t.Fatalf("approvePermit: %v", err)
	}
	got, _ := getSession(context.Background(), s.ID)
	if !got.PermitApproved {
		t.Error("PermitApproved = false after approvePermit")
	}
	if got.BeansApproved || got.PowSolved {
		t.Error("unexpected flags set by approvePermit")
	}
}

func TestApproveBeans(t *testing.T) {
	freshDB(t)
	s, _ := createSession(context.Background())
	if err := approveBeans(context.Background(), s.ID); err != nil {
		t.Fatalf("approveBeans: %v", err)
	}
	got, _ := getSession(context.Background(), s.ID)
	if !got.BeansApproved {
		t.Error("BeansApproved = false after approveBeans")
	}
}

func TestIncrementRejection(t *testing.T) {
	freshDB(t)
	s, _ := createSession(context.Background())

	count, err := incrementRejection(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("incrementRejection: %v", err)
	}
	if count != 1 {
		t.Errorf("first increment = %d, want 1", count)
	}

	count, _ = incrementRejection(context.Background(), s.ID)
	if count != 2 {
		t.Errorf("second increment = %d, want 2", count)
	}
}

func TestSetAndGetPowChallenge(t *testing.T) {
	freshDB(t)
	s, _ := createSession(context.Background())

	if err := setPowChallenge(context.Background(), s.ID, "deadbeef"); err != nil {
		t.Fatalf("setPowChallenge: %v", err)
	}
	got, _ := getSession(context.Background(), s.ID)
	if got.PowChallenge != "deadbeef" {
		t.Errorf("PowChallenge = %q, want %q", got.PowChallenge, "deadbeef")
	}
}

func TestSetPowSolved(t *testing.T) {
	freshDB(t)
	s, _ := createSession(context.Background())

	if err := setPowSolved(context.Background(), s.ID); err != nil {
		t.Fatalf("setPowSolved: %v", err)
	}
	got, _ := getSession(context.Background(), s.ID)
	if !got.PowSolved {
		t.Error("PowSolved = false after setPowSolved")
	}
}

func TestSetBrewStarted(t *testing.T) {
	freshDB(t)
	s, _ := createSession(context.Background())

	if err := setBrewStarted(context.Background(), s.ID); err != nil {
		t.Fatalf("setBrewStarted: %v", err)
	}
	got, _ := getSession(context.Background(), s.ID)
	if !got.BrewStarted {
		t.Error("BrewStarted = false after setBrewStarted")
	}
}

func TestManagerOverride(t *testing.T) {
	freshDB(t)
	s, _ := createSession(context.Background())

	if err := managerOverride(context.Background(), s.ID); err != nil {
		t.Fatalf("managerOverride: %v", err)
	}
	got, _ := getSession(context.Background(), s.ID)
	if !got.PermitApproved {
		t.Error("PermitApproved = false after managerOverride")
	}
	if !got.BeansApproved {
		t.Error("BeansApproved = false after managerOverride")
	}
	if !got.PowSolved {
		t.Error("PowSolved = false after managerOverride")
	}
}

func TestSetSessionGeminiKey(t *testing.T) {
	freshDB(t)
	s, _ := createSession(context.Background())

	if err := setSessionGeminiKey(context.Background(), s.ID, "test-api-key"); err != nil {
		t.Fatalf("setSessionGeminiKey: %v", err)
	}
	got, _ := getSession(context.Background(), s.ID)
	if got.GeminiKey != "test-api-key" {
		t.Errorf("GeminiKey = %q, want %q", got.GeminiKey, "test-api-key")
	}

	// Overwrite with a new key.
	if err := setSessionGeminiKey(context.Background(), s.ID, "updated-key"); err != nil {
		t.Fatalf("setSessionGeminiKey update: %v", err)
	}
	got, _ = getSession(context.Background(), s.ID)
	if got.GeminiKey != "updated-key" {
		t.Errorf("GeminiKey after update = %q, want %q", got.GeminiKey, "updated-key")
	}
}
