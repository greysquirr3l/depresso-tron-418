package main

import (
	"context"
	"database/sql"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestMain initialises shared test infrastructure (templates, server state)
// before any test in this package runs.
func TestMain(m *testing.M) {
	// Parse embedded templates so handler tests can render them.
	var err error
	tmpl, err = template.New("").Funcs(template.FuncMap{
		"not": func(b bool) bool { return !b },
	}).ParseFS(embedFS, "templates/*.html")
	if err != nil {
		panic("TestMain: template parse failed: " + err.Error())
	}

	// Initialise a shared in-memory DB so tests that call handlers work.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic("TestMain: open :memory: DB: " + err.Error())
	}
	if _, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS sessions (
			id               TEXT    PRIMARY KEY,
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			permit_approved  INTEGER  NOT NULL DEFAULT 0,
			beans_approved   INTEGER  NOT NULL DEFAULT 0,
			pow_solved       INTEGER  NOT NULL DEFAULT 0,
			brew_started     INTEGER  NOT NULL DEFAULT 0,
			rejection_count  INTEGER  NOT NULL DEFAULT 0,
			when_window_at   DATETIME,
			pow_challenge    TEXT     NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		);
	`); err != nil {
		panic("TestMain: schema: " + err.Error())
	}
	database = db

	serverState = newAppState()

	os.Exit(m.Run())
}

func TestHandleTeapotState_ReturnsHTMLWithMood(t *testing.T) {
	serverState.SetMood(MoodIdle)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/teapot-state", nil)
	w := httptest.NewRecorder()

	handleTeapotState(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "mood-badge") {
		t.Errorf("response missing mood-badge: %q", body)
	}
	if !strings.Contains(body, "IDLE") {
		t.Errorf("response missing IDLE mood text: %q", body)
	}
}

func TestHandleGeminiStatus_OnlineWhenKeySet(t *testing.T) {
	oldKey := geminiKey
	geminiKey = "fake-key-for-test"
	defer func() { geminiKey = oldKey }()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/gemini-status", nil)
	w := httptest.NewRecorder()

	handleGeminiStatus(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "BARISTA ONLINE") {
		t.Errorf("expected BARISTA ONLINE, got: %q", body)
	}
}

func TestHandleGeminiStatus_OfflineWhenNoKey(t *testing.T) {
	oldKey := geminiKey
	geminiKey = ""
	defer func() { geminiKey = oldKey }()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/gemini-status", nil)
	w := httptest.NewRecorder()

	handleGeminiStatus(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "OFFLINE SNARK") {
		t.Errorf("expected OFFLINE SNARK MODE, got: %q", body)
	}
}

func TestHandleIndex_NotFoundForNonRoot(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/unknown-path", nil)
	w := httptest.NewRecorder()

	handleIndex(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleIndex_RendersRoot(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handleIndex(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestHandleManager_SetsAllFlagsAndRedirects(t *testing.T) {
	freshDB(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/manager", nil)
	w := httptest.NewRecorder()

	handleManager(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if loc := w.Header().Get("HX-Redirect"); loc != "/" {
		t.Errorf("HX-Redirect = %q, want /", loc)
	}

	// Extract the session cookie and verify DB flags.
	var sid string
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName {
			sid = c.Value
		}
	}
	if sid == "" {
		t.Fatal("no session cookie in response")
	}
	s, err := getSession(context.Background(), sid)
	if err != nil {
		t.Fatalf("getSession: %v", err)
	}
	if !s.PermitApproved || !s.BeansApproved || !s.PowSolved {
		t.Errorf("manager override incomplete: permit=%v beans=%v pow=%v",
			s.PermitApproved, s.BeansApproved, s.PowSolved)
	}
}

func TestHandleSetGeminiKey_RejectsEmptyKey(t *testing.T) {
	freshDB(t)
	form := url.Values{"gemini_key": {""}}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/set-gemini-key",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handleSetGeminiKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "empty") {
		t.Errorf("expected 'empty' in rejection message, got: %q", body)
	}
}

func TestHandleSetGeminiKey_RejectsTooShortKey(t *testing.T) {
	freshDB(t)
	form := url.Values{"gemini_key": {"tooshort"}}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/set-gemini-key",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handleSetGeminiKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "short") {
		t.Errorf("expected 'short' in rejection message, got: %q", body)
	}
}

func TestHandleCheckBeans_RequiresPermit(t *testing.T) {
	freshDB(t)
	form := url.Values{"description": {"ethically-sourced micro-lot notes of stone fruit"}}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/check-beans",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handleCheckBeans(w, req)

	// Without a permit, the handler must refuse (403 content w/ 200 or explicit 403).
	body := w.Body.String()
	if !strings.Contains(body, "403") && !strings.Contains(strings.ToLower(body), "permit") {
		t.Errorf("expected permit-required error, got: %q", body)
	}
}

func TestHandlePowChallenge_RequiresBeansApproved(t *testing.T) {
	freshDB(t)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/pow-challenge", nil)
	w := httptest.NewRecorder()

	handlePowChallenge(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "403") && !strings.Contains(strings.ToLower(body), "bean") {
		t.Errorf("expected bean-auth error for unapproved session, got: %q", body)
	}
}
