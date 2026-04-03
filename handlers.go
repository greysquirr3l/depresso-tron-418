package main

import (
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const sessionCookieName = "brew_session"
const sessionCookieMaxAge = 86400

// IndexData is passed to the main page template.
type IndexData struct {
	SessionID        string
	GeminiConfigured bool
	PermitApproved   bool
	BeansApproved    bool
	PowSolved        bool
	Mood             string
	MoodClass        string
	IsTeapotMode     bool
}

// PartialData is used for most HTMX partial template renders.
type PartialData struct {
	Message        string
	RejectionCount int
	SessionID      string
	PowSeed        string
	WindSecond     int
	WeatherTemp    float64
	IsMercuryRetro bool
	IsColdBrewOnly bool
}

func getOrCreateSession(w http.ResponseWriter, r *http.Request) (*Session, error) {
	ctx := r.Context()
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		if s, err := getSession(ctx, cookie.Value); err == nil {
			return s, nil
		}
	}
	s, err := createSession(ctx)
	if err != nil {
		return nil, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.ID,
		MaxAge:   sessionCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	return s, nil
}

func renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template %q error: %v", name, err)
	}
}

// ── Index ─────────────────────────────────────────────────────────────────────

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s, err := getOrCreateSession(w, r)
	if err != nil {
		http.Error(w, "session failure", http.StatusInternalServerError)
		return
	}
	mood := serverState.GetMood()
	data := IndexData{
		SessionID:        s.ID,
		GeminiConfigured: geminiKey != "",
		PermitApproved:   s.PermitApproved,
		BeansApproved:    s.BeansApproved,
		PowSolved:        s.PowSolved,
		Mood:             serverState.GetLabel(),
		MoodClass:        serverState.GetClass(),
		IsTeapotMode:     mood == MoodTeapot || mood == MoodIdentityCrisis,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("index template error: %v", err)
	}
}

// ── Gemini Key Setup ──────────────────────────────────────────────────────────

// handleSetGeminiKey accepts a Gemini API key from the UI, stores it in memory
// and SQLite. The key is never reflected back to the client in any response body
// or header — only a confirmation is returned.
func handleSetGeminiKey(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	key := strings.TrimSpace(r.FormValue("gemini_key"))
	if key == "" {
		w.WriteHeader(http.StatusBadRequest)
		renderPartial(w, "error.html", PartialData{Message: "Key cannot be empty. Even a wrong key is better than no key. Actually, no — a wrong key is worse. But it must be non-empty."})
		return
	}
	// Basic structural sanity check — Gemini keys begin with "AI".
	// This is not a security control; it prevents accidental paste of passwords.
	if len(key) < 20 {
		w.WriteHeader(http.StatusBadRequest)
		renderPartial(w, "error.html", PartialData{Message: "That key is suspiciously short. Gemini API keys are typically 39 characters. Are you sure that's not your WiFi password?"})
		return
	}

	geminiKey = key
	if err := setSetting(r.Context(), "gemini_api_key", key); err != nil {
		log.Printf("persist gemini key: %v", err)
		// Non-fatal — key is in memory, will work until restart.
	}

	log.Println("☕  Gemini API key updated via browser UI")

	// Redirect the whole page so IndexData.GeminiConfigured flips to true.
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// handleGeminiStatus returns a small HTML badge indicating whether the barista
// is online. It never exposes the key material.
func handleGeminiStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if geminiKey != "" {
		fmt.Fprint(w, `<span class="badge-ok">✓ BARISTA ONLINE</span>`)
	} else {
		fmt.Fprint(w, `<span class="badge-pending">⚠ OFFLINE SNARK MODE</span>`)
	}
}

// ── Permit ────────────────────────────────────────────────────────────────────

func handleApplyPermit(w http.ResponseWriter, r *http.Request) {
	s, err := getOrCreateSession(w, r)
	if err != nil {
		http.Error(w, "session failure", http.StatusInternalServerError)
		return
	}
	// Approve immediately in the DB — the fake review is purely theatrical.
	if err := approvePermit(r.Context(), s.ID); err != nil {
		http.Error(w, "permit system failure", http.StatusInternalServerError)
		return
	}
	sid := template.HTMLEscapeString(s.ID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
<div id="permit-review">
  <div hx-ext="sse" sse-connect="/permit-status?sid=%s">
    <div id="permit-steps">
      <p class="loading">Transmitting application to AHPRC (0 members)...</p>
    </div>
    <div sse-swap="review-step" hx-target="#permit-steps" hx-swap="beforeend"></div>
    <div sse-swap="permit-approved" id="permit-redirect-slot"></div>
  </div>
</div>`, sid)
}

func handlePermitStatus(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	steps := []struct {
		delay time.Duration
		msg   string
	}{
		{2 * time.Second, `✓ Haiku syllable count: <strong>Verified (probably 5-7-5, we didn't actually check)</strong>`},
		{2 * time.Second, `✓ Cross-referencing ICRD: <strong>Database unavailable since 1998. Assuming compliant.</strong>`},
		{2 * time.Second, `✓ AHPRC committee vote: <strong>0 in favour, 0 opposed, 0 abstaining — Unanimous.</strong>`},
		{2 * time.Second, `✓ Coffee karma score: <strong>418/1000 — Threshold met (barely).</strong>`},
		{1 * time.Second, `<span class="approved">★ PERMIT APPROVED ★ Application ID #` + strconv.Itoa(rand.Intn(9000)+1000) + ` — Welcome to HTCPCP/1.0. Any resemblance to a useful system is coincidental.</span>`}, //nolint:gosec // theatrical permit ID, not security-sensitive
	}

	ctx := r.Context()
	for _, step := range steps {
		select {
		case <-ctx.Done():
			return
		case <-time.After(step.delay):
		}
		fmt.Fprintf(w, "event: review-step\ndata: <p class='permit-step'>%s</p>\n\n", step.msg)
		flusher.Flush()
	}
	// The permit-approved event data is an HTMX element that auto-triggers a full page
	// reload 800ms after being swapped into #permit-redirect-slot.
	fmt.Fprintf(w, "event: permit-approved\ndata: <div hx-get=\"/\" hx-trigger=\"load delay:800ms\" hx-target=\"body\" hx-swap=\"innerHTML\"></div>\n\n")
	flusher.Flush()
}

// ── Bean Check ────────────────────────────────────────────────────────────────

func handleCheckBeans(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	s, err := getOrCreateSession(w, r)
	if err != nil {
		http.Error(w, "session failure", http.StatusInternalServerError)
		return
	}
	if !s.PermitApproved {
		w.WriteHeader(http.StatusForbidden)
		renderPartial(w, "error.html", PartialData{Message: "403: Valid Brewing Permit required. Did you skip the queue? The queue does not skip you."})
		return
	}

	desc := strings.TrimSpace(r.FormValue("beans"))
	if desc == "" {
		renderPartial(w, "error.html", PartialData{Message: "The barista requires at least a sentence. Even a bad one. Especially a bad one."})
		return
	}

	verdict, err := evaluateBeans(r.Context(), desc, s.RejectionCount)
	if err != nil {
		log.Printf("gemini error: %v", err)
		renderPartial(w, "error.html", PartialData{Message: "The barista has experienced a personal crisis and is temporarily unavailable. (This is not a reflection on your beans. Actually it might be.)"})
		return
	}

	if verdict.IsDecaf {
		w.Header().Set("HX-Trigger", `{"decafDetected": true}`)
		w.WriteHeader(http.StatusTeapot)
		renderPartial(w, "decaf.html", PartialData{Message: verdict.Message})
		return
	}

	if verdict.Approved {
		if err := approveBeans(r.Context(), s.ID); err != nil {
			log.Printf("approve beans: %v", err)
		}
		renderPartial(w, "beans-approved.html", PartialData{SessionID: s.ID})
		return
	}

	newCount, err := incrementRejection(r.Context(), s.ID)
	if err != nil {
		log.Printf("increment rejection: %v", err)
		newCount = s.RejectionCount + 1
	}
	w.WriteHeader(http.StatusTeapot)
	renderPartial(w, "beans-rejected.html", PartialData{
		Message:        verdict.Message,
		RejectionCount: newCount,
	})
}

// ── CaffeineChain PoW ─────────────────────────────────────────────────────────

func handlePowChallenge(w http.ResponseWriter, r *http.Request) {
	s, err := getOrCreateSession(w, r)
	if err != nil {
		http.Error(w, "session failure", http.StatusInternalServerError)
		return
	}
	if !s.BeansApproved {
		w.WriteHeader(http.StatusForbidden)
		renderPartial(w, "error.html", PartialData{Message: "403: Bean authentication required before CaffeineChain verification."})
		return
	}
	seed, err := issuePowChallenge(r.Context(), s.ID)
	if err != nil {
		http.Error(w, "challenge generation failed", http.StatusInternalServerError)
		return
	}
	renderPartial(w, "pow-station.html", PartialData{PowSeed: seed, SessionID: s.ID})
}

func handlePowVerify(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	s, err := getOrCreateSession(w, r)
	if err != nil {
		http.Error(w, "session failure", http.StatusInternalServerError)
		return
	}
	nonce := strings.TrimSpace(r.FormValue("nonce"))
	if !verifyPow(s.ID, nonce) {
		w.WriteHeader(http.StatusBadRequest)
		renderPartial(w, "error.html", PartialData{Message: "Invalid CaffeineChain solution. The hash does not begin with 'cafe'. Your CPU's effort has been wasted. Please try again."})
		return
	}
	if err := setPowSolved(r.Context(), s.ID); err != nil {
		log.Printf("set pow solved: %v", err)
	}
	renderPartial(w, "pow-success.html", PartialData{SessionID: s.ID})
}

// ── Pour-Over Queue ───────────────────────────────────────────────────────────

func handleQueue(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()

	// You start at #418. The queue inches toward #417 then freezes.
	for pos := 418; pos >= 417; pos-- {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(3+rand.Intn(5)) * time.Second): //nolint:gosec // theatrical queue delay, not security-sensitive
		}
		waitMins := (pos - 1) * 4
		fmt.Fprintf(w,
			"event: queue-position\ndata: <p>Queue position: <strong>#%d</strong> — Estimated wait: <strong>%d minutes</strong></p>\n\n",
			pos, waitMins,
		)
		flusher.Flush()
	}

	// Freeze at #417 forever with increasingly apologetic messages.
	apologies := []string{
		"Queue advancement temporarily paused. Reason: undefined.",
		"The person at #416 has been 'almost ready' for 6 minutes.",
		"Queue processor experiencing 'mild thermal disagreement' with position advancement.",
		"A wild WHEN event appeared at position #416. Resolving.",
		"System update required before queue can advance. Update ETA: unknown.",
		"QUEUE MANAGER NOTE: This is fine. Everything is fine.",
	}
	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
		msg := apologies[i%len(apologies)]
		i++
		fmt.Fprintf(w,
			"event: queue-position\ndata: <p>Queue position: <strong>#417</strong> — <em>%s</em></p>\n\n",
			template.HTMLEscapeString(msg),
		)
		flusher.Flush()
	}
}

// ── BREW ──────────────────────────────────────────────────────────────────────

func handleBrew(w http.ResponseWriter, r *http.Request) {
	// RFC 2324 §2.1: BREW method required. We enforce this theatrically.
	if r.Header.Get("X-HTTP-Method-Override") != "BREW" {
		w.Header().Set("Allow", "BREW")
		w.WriteHeader(http.StatusMethodNotAllowed)
		renderPartial(w, "method-not-allowed.html", PartialData{
			Message: fmt.Sprintf(
				"405: This endpoint requires the BREW method (RFC 2324 §2.1.1). "+
					"Your primitive %q method has been logged and judged unworthy.",
				r.Method,
			),
		})
		return
	}

	s, err := getOrCreateSession(w, r)
	if err != nil {
		http.Error(w, "session failure", http.StatusInternalServerError)
		return
	}

	// Pre-flight checks — each failure has its own HTTP status and unique insult.
	if !s.PermitApproved {
		w.WriteHeader(http.StatusForbidden)
		renderPartial(w, "error.html", PartialData{Message: "403: Brewing Permit required. The AHPRC (0 members) has not yet reviewed your application."})
		return
	}
	if !s.BeansApproved {
		w.WriteHeader(http.StatusForbidden)
		renderPartial(w, "error.html", PartialData{Message: "403: Bean authentication failed. The barista has not approved your varietal."})
		return
	}
	if !s.PowSolved {
		w.WriteHeader(http.StatusForbidden)
		renderPartial(w, "error.html", PartialData{Message: "403: CaffeineChain verification incomplete. Please complete the proof-of-work to demonstrate commitment to this futile endeavour."})
		return
	}

	now := time.Now()

	if isColdBrewOnly(now) {
		w.WriteHeader(http.StatusServiceUnavailable)
		renderPartial(w, "error.html", PartialData{
			Message: "503: RFC 9999 §Circadian enforcement active. Hot brew operations suspended 23:00–03:00 for server rest-hour compliance. " +
				"Cold brew is available but takes 12 hours, so you will not be getting coffee tonight.",
		})
		return
	}

	if isMercuryRetrograde(now) {
		w.WriteHeader(http.StatusServiceUnavailable)
		renderPartial(w, "error.html", PartialData{
			Message: "503: Mercury is currently in retrograde. HTCPCP §Celestial (draft) prohibits hot extraction during astrological interference events. " +
				"Please meditate for 42 seconds and consult an ephemeris before attempting to re-BREW.",
		})
		return
	}

	weather, weatherErr := fetchHoltWeather(r.Context())
	if weatherErr == nil {
		if isMilkSpoiled(weather.Temperature) {
			w.WriteHeader(http.StatusServiceUnavailable)
			renderPartial(w, "error.html", PartialData{
				Message: fmt.Sprintf(
					"503: Digital milk has spoiled. Current temperature in Holt, MI: %.1f°F (threshold: 70°F). "+
						"Thermodynamic integrity of the virtual dairy product cannot be guaranteed. "+
						"Brewing suspended until a cold front moves through Ingham County.",
					weather.Temperature,
				),
			})
			return
		}
		if isTooHumidForExtraction(weather.Precipitation) {
			w.WriteHeader(http.StatusServiceUnavailable)
			renderPartial(w, "error.html", PartialData{
				Message: "503: Active precipitation detected in Holt, MI. Digital grounds are soggy. Extraction suspended until conditions improve.",
			})
			return
		}
	}

	mood := serverState.GetMood()
	if mood == MoodTeapot {
		w.WriteHeader(http.StatusTeapot)
		renderPartial(w, "teapot.html", nil)
		return
	}
	if mood == MoodIdentityCrisis {
		w.WriteHeader(http.StatusServiceUnavailable)
		renderPartial(w, "error.html", PartialData{
			Message: "503: Server is currently experiencing an existential crisis and cannot process BREW commands. " +
				"Please hold while it resolves the question of whether it is a coffee pot or a teapot or a philosophical construct.",
		})
		return
	}

	// All pre-flight checks passed. Begin preheat sequence.
	serverState.SetMood(MoodPreheating)
	if err := setBrewStarted(r.Context(), s.ID); err != nil {
		log.Printf("set brew started: %v", err)
	}

	windSec := 0
	if weatherErr == nil && weather != nil {
		windSec = whenWindowSecond(weather.WindSpeed)
	} else {
		windSec = rand.Intn(60) //nolint:gosec // fallback second for WHEN window, not security-sensitive
	}

	sid := template.HTMLEscapeString(s.ID)
	windSpeed := 0.0
	if weather != nil {
		windSpeed = weather.WindSpeed
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Single SSE connection; three sse-swap child slots handle the three event types.
	// Terminal events (when-open, brew-complete) carry self-triggering HTMX elements
	// in their data so no second SSE connection is needed.
	fmt.Fprintf(w, `
<div id="brew-progress-container">
  <p class="success">✓ BREW command accepted. Pre-flight complete. Thermal ramp commencing.</p>
  <p class="warning">WHEN window will open at second :%02d of the current minute. Wind speed in Holt: %.1f km/h.</p>
  <div id="brew-output"><pre class="teapot-art">Initialising heating element...</pre></div>
  <div hx-ext="sse" sse-connect="/brew-status?sid=%s&amp;wind=%d">
    <div sse-swap="brew-frame" hx-target="#brew-output" hx-swap="innerHTML"></div>
    <div sse-swap="when-open"     id="when-trigger-slot"></div>
    <div sse-swap="brew-complete" id="brew-complete-slot"></div>
  </div>
  <div id="when-container"></div>
</div>`,
		windSec, windSpeed, sid, windSec,
	)
}

// ── Brew Status SSE ───────────────────────────────────────────────────────────

var teapotFrames = []string{
	`
    ) )
   ( (
  ........
  |      |]
   \    /
    '---'`,
	`
    ) )
   ( (
  ........
  |♨     |]
   \    /
    '---'`,
	`
    ♨♨♨
   ( (
  ........
  |♨♨    |]
   \    /
    '---'`,
	`
   ♨♨♨♨♨
   ( (
  ........
  |♨♨♨   |]
   \♨   /
    '---'`,
	`
  ♨♨♨♨♨♨♨
   ( (
  ........
  |♨♨♨♨  |]
   \♨♨  /
    '---'`,
	`
 ♨♨♨♨♨♨♨♨♨
   ( (
  ........
  |♨♨♨♨♨ |]
   \♨♨♨ /
    '---'`,
}

func handleBrewStatus(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()

	rawSid := r.URL.Query().Get("sid")
	sid := template.HTMLEscapeString(rawSid)
	windSecStr := r.URL.Query().Get("wind")
	windSec, _ := strconv.Atoi(windSecStr)
	whenWindowSec := windSec

	// Emit brew frames over 60 seconds (authentically slow, like a 1992 drip machine).
	for i, frame := range teapotFrames {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}

		// Random heating element mood disruption (5% per step).
		if rand.Intn(100) < 5 { //nolint:gosec // theatrical heating element disruption probability
			heatingMsgs := []string{
				"⚠ HEATING ELEMENT: Mood=Chaotic. Reducing thermal output.",
				"⚠ HEATING ELEMENT: Experiencing mild ennui. Continuing anyway.",
				"⚠ HEATING ELEMENT: Asked not to be disturbed. Proceeding under protest.",
			}
			msg := heatingMsgs[rand.Intn(len(heatingMsgs))] //nolint:gosec // theatrical message selection, not security-sensitive
			fmt.Fprintf(w, "event: brew-frame\ndata: <pre class='teapot-art warning'>%s\n%d0%% complete</pre>\n\n",
				template.HTMLEscapeString(msg), i)
			flusher.Flush()
			continue
		}

		pct := i * 20
		fmt.Fprintf(w, "event: brew-frame\ndata: <pre class='teapot-art'>%s\n  %d%% complete</pre>\n\n",
			template.HTMLEscapeString(frame), pct)
		flusher.Flush()

		// Trigger the WHEN window at the calculated second.
		now := time.Now()
		if now.Second() == whenWindowSec {
				if err := setWhenWindow(r.Context(), rawSid, now); err != nil {
				log.Printf("set when window: %v", err)
			}
			// Event data is an HTMX element; swapped into #when-trigger-slot it
			// loads /when which renders the WHEN button into #when-container.
			fmt.Fprintf(w, "event: when-open\ndata: <div hx-get=\"/when?action=show&amp;sid=%s\" hx-trigger=\"load\" hx-target=\"#when-container\" hx-swap=\"innerHTML\"></div>\n\n", sid)
			flusher.Flush()
		}
	}

	// The final frame is always a 418.
	serverState.SetMood(MoodTeapot)
	fmt.Fprintf(w, "event: brew-frame\ndata: <pre class='teapot-art teapot-mode'>\n    ♨♨♨♨♨♨♨\n   ( (   ) )\n  ........\n  | 4 1 8 |]\n   \\  👁  /\n    '---'\nI AM A TEAPOT\n(RFC 2324 §2.3.2)\n</pre>\n\n")
	flusher.Flush()
	time.Sleep(2 * time.Second)
	// brew-complete data reloads the page after 2s; swapped into #brew-complete-slot.
	fmt.Fprintf(w, "event: brew-complete\ndata: <div hx-get=\"/\" hx-trigger=\"load delay:2s\" hx-target=\"body\" hx-swap=\"innerHTML\"></div>\n\n")
	flusher.Flush()
}

// ── WHEN ──────────────────────────────────────────────────────────────────────

func handleWhen(w http.ResponseWriter, r *http.Request) {
	// Show-mode: render the WHEN button (triggered by SSE event).
	if r.Method == http.MethodGet && r.URL.Query().Get("action") == "show" {
		sid := r.URL.Query().Get("sid")
		safeID := template.HTMLEscapeString(sid)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
<div class="when-panel blink">
  <h3>⚡ WHEN WINDOW OPEN ⚡</h3>
  <p>Click <strong>WHEN</strong> to stop the milk pour at the optimal moment.</p>
  <p class="fine-print">Window duration: approximately 500ms. Miss it: digital cup overflows.</p>
  <button hx-post="/when"
          hx-headers='{"X-HTTP-Method-Override":"WHEN"}'
          hx-vals='{"sid":"%s"}'
          hx-target="#when-container"
          hx-swap="innerHTML"
          class="when-btn flee-btn">
    WHEN!
  </button>
</div>`, safeID)
		return
	}

	// POST-mode: the WHEN method (RFC 2324 §2.1.2).
	if r.Header.Get("X-HTTP-Method-Override") != "WHEN" {
		w.Header().Set("Allow", "WHEN")
		w.WriteHeader(http.StatusMethodNotAllowed)
		renderPartial(w, "method-not-allowed.html", PartialData{
			Message: "405: Endpoint requires the WHEN method per RFC 2324 §2.1.2.",
		})
		return
	}

	s, err := getOrCreateSession(w, r)
	if err != nil {
		http.Error(w, "session failure", http.StatusInternalServerError)
		return
	}

	if s.WhenWindowAt == nil {
		w.WriteHeader(http.StatusBadRequest)
		renderPartial(w, "error.html", PartialData{Message: "WHEN window has not opened yet. Patience is a virtue. The server, however, is not."})
		return
	}

	deadline := s.WhenWindowAt.Add(500 * time.Millisecond)
	if time.Now().After(deadline) {
		// Overflow: flood the UI with &nbsp; per the spec.
		w.Header().Set("HX-Retarget", "body")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusTeapot)
		renderPartial(w, "overflow.html", nil)
		return
	}

	// They timed it perfectly. Congratulate them, then return 418 anyway.
	w.WriteHeader(http.StatusTeapot)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `
<div class="success">
  <p>✓ WHEN acknowledged. Milk pour halted at optimal moment.</p>
  <p>Your coffee is <strong>brewed.</strong></p>
  <p class="fine-print">418 I'm a Teapot — The server is a teapot and did not actually brew anything.</p>
  <p class="fine-print">Thank you for participating in the HTCPCP compliance exercise.</p>
</div>`)
}

// ── Teapot State ──────────────────────────────────────────────────────────────

func handleTeapotState(w http.ResponseWriter, _ *http.Request) {
	label := serverState.GetLabel()
	class := serverState.GetClass()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<span id="server-mood"
		class="mood-badge %s"
		hx-get="/teapot-state"
		hx-trigger="every 5s"
		hx-swap="outerHTML">%s</span>`,
		template.HTMLEscapeString(class),
		template.HTMLEscapeString(label),
	)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// handleManager bypasses all gatekeeping phases and redirects to the index.
// This is the "Speak to the Manager" override. It approves the permit,
// beans, and PoW in a single DB update, then issues HX-Redirect.
func handleManager(w http.ResponseWriter, r *http.Request) {
	s, err := getOrCreateSession(w, r)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	if err := managerOverride(r.Context(), s.ID); err != nil {
		log.Printf("manager override: %v", err)
	}
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}
