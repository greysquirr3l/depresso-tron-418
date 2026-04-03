package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

var database *sql.DB

// Session tracks a single user's brewing journey through the gauntlet.
type Session struct {
	ID             string
	CreatedAt      time.Time
	PermitApproved bool
	BeansApproved  bool
	PowSolved      bool
	BrewStarted    bool
	RejectionCount int
	WhenWindowAt   *time.Time
	PowChallenge   string
	GeminiKey      string
}

func initDB() error {
	db, err := sql.Open("sqlite", "depresso.db")
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	database = db

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
		return fmt.Errorf("init schema: %w", err)
	}
	// Idempotent migration for databases created before the gemini_key column
	// was added. SQLite returns an error on duplicate columns; we intentionally
	// ignore it.
	_, _ = db.ExecContext(context.Background(), `ALTER TABLE sessions ADD COLUMN gemini_key TEXT NOT NULL DEFAULT ''`)
	return nil
}

// getSetting and setSetting removed — Gemini API keys are per-session.
// See setSessionGeminiKey / Session.GeminiKey.

func closeDB() {
	if database != nil {
		_ = database.Close()
	}
}

func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func createSession(ctx context.Context) (*Session, error) {
	id, err := newSessionID()
	if err != nil {
		return nil, err
	}
	_, err = database.ExecContext(ctx, `INSERT INTO sessions (id) VALUES (?)`, id)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return &Session{ID: id, CreatedAt: time.Now()}, nil
}

func getSession(ctx context.Context, id string) (*Session, error) {
	row := database.QueryRowContext(ctx, `
		SELECT id, created_at, permit_approved, beans_approved,
		       pow_solved, brew_started, rejection_count, when_window_at, pow_challenge, gemini_key
		FROM sessions WHERE id = ?`, id)

	s := &Session{}
	var whenWindowAt sql.NullTime
	var permitApproved, beansApproved, powSolved, brewStarted int

	err := row.Scan(
		&s.ID, &s.CreatedAt,
		&permitApproved, &beansApproved,
		&powSolved, &brewStarted,
		&s.RejectionCount, &whenWindowAt, &s.PowChallenge, &s.GeminiKey,
	)
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}

	s.PermitApproved = permitApproved == 1
	s.BeansApproved = beansApproved == 1
	s.PowSolved = powSolved == 1
	s.BrewStarted = brewStarted == 1
	if whenWindowAt.Valid {
		s.WhenWindowAt = &whenWindowAt.Time
	}
	return s, nil
}

func approvePermit(ctx context.Context, id string) error {
	if _, err := database.ExecContext(ctx, `UPDATE sessions SET permit_approved = 1 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("approve permit: %w", err)
	}
	return nil
}

func approveBeans(ctx context.Context, id string) error {
	if _, err := database.ExecContext(ctx, `UPDATE sessions SET beans_approved = 1 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("approve beans: %w", err)
	}
	return nil
}

// incrementRejection increments and returns the new rejection count.
func incrementRejection(ctx context.Context, id string) (int, error) {
	_, err := database.ExecContext(ctx,
		`UPDATE sessions SET rejection_count = rejection_count + 1 WHERE id = ?`, id,
	)
	if err != nil {
		return 0, fmt.Errorf("increment rejection: %w", err)
	}
	s, err := getSession(ctx, id)
	if err != nil {
		return 0, err
	}
	return s.RejectionCount, nil
}

func setPowChallenge(ctx context.Context, id, challenge string) error {
	if _, err := database.ExecContext(ctx, `UPDATE sessions SET pow_challenge = ? WHERE id = ?`, challenge, id); err != nil {
		return fmt.Errorf("set pow challenge: %w", err)
	}
	return nil
}

func setPowSolved(ctx context.Context, id string) error {
	if _, err := database.ExecContext(ctx, `UPDATE sessions SET pow_solved = 1 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("set pow solved: %w", err)
	}
	return nil
}

// managerOverride fast-tracks the session through all gatekeeping phases.
func managerOverride(ctx context.Context, id string) error {
	_, err := database.ExecContext(ctx, `
		UPDATE sessions
		SET permit_approved = 1,
		    beans_approved  = 1,
		    pow_solved      = 1
		WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("manager override: %w", err)
	}
	return nil
}

func setBrewStarted(ctx context.Context, id string) error {
	if _, err := database.ExecContext(ctx, `UPDATE sessions SET brew_started = 1 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("set brew started: %w", err)
	}
	return nil
}

func setWhenWindow(ctx context.Context, id string, t time.Time) error {
	_, err := database.ExecContext(ctx,
		`UPDATE sessions SET when_window_at = ? WHERE id = ?`, t, id,
	)
	if err != nil {
		return fmt.Errorf("set when window: %w", err)
	}
	return nil
}

// setSessionGeminiKey stores the user's personal Gemini API key against their session.
// The key is never logged or reflected back to the client.
func setSessionGeminiKey(ctx context.Context, id, key string) error {
	if _, err := database.ExecContext(ctx, `UPDATE sessions SET gemini_key = ? WHERE id = ?`, key, id); err != nil {
		return fmt.Errorf("set session gemini key: %w", err)
	}
	return nil
}
