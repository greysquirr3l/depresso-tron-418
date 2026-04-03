package main

import (
	"math/rand"
	"sync"
	"time"
)

// Mood represents the server's current existential state.
type Mood int

const (
	MoodIdle Mood = iota
	MoodPreheating
	MoodBrewing
	MoodTeapot
	MoodIdentityCrisis
)

var moodLabels = map[Mood]string{
	MoodIdle:           "IDLE ∷ Awaiting caffeine directive",
	MoodPreheating:     "PREHEATING ∷ Thermal equilibration in progress",
	MoodBrewing:        "BREWING ∷ Do not jostle the apparatus",
	MoodTeapot:         "TEAPOT MODE ∷ I am short and stout (RFC 2324 §2.3) — brew attempts return 418",
	MoodIdentityCrisis: "IDENTITY CRISIS ∷ If I cannot brew coffee, am I still a teapot? Philosophical downtime estimated 1–3 min.",
}

var moodClasses = map[Mood]string{
	MoodIdle:           "mood-idle",
	MoodPreheating:     "mood-preheating",
	MoodBrewing:        "mood-brewing",
	MoodTeapot:         "mood-teapot",
	MoodIdentityCrisis: "mood-crisis",
}

// AppState holds the server's current teapot identity.
type AppState struct {
	mu   sync.RWMutex
	mood Mood
}

func newAppState() *AppState {
	return &AppState{mood: MoodIdle}
}

func (s *AppState) GetMood() Mood {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mood
}

func (s *AppState) SetMood(m Mood) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mood = m
}

func (s *AppState) GetLabel() string {
	return moodLabels[s.GetMood()]
}

func (s *AppState) GetClass() string {
	return moodClasses[s.GetMood()]
}

// runMoodCycle runs a background goroutine that randomly transitions
// the server through its five existential states. This is the core
// innovation distinguishing us from lesser 418 implementations.
func (s *AppState) runMoodCycle() {
	for {
		wait := time.Duration(30+rand.Intn(150)) * time.Second //nolint:gosec // theatrical delay, not security-sensitive
		time.Sleep(wait)

		current := s.GetMood()
		switch current {
		case MoodIdle:
			// 15% chance of spontaneous teapot identity capture
			if rand.Intn(100) < 15 { //nolint:gosec // theatrical mood transition probability
				s.SetMood(MoodTeapot)
			}
		case MoodTeapot:
			// Prolonged teapot mode induces philosophical spiral
			if rand.Intn(100) < 40 { //nolint:gosec // theatrical mood transition probability
				s.SetMood(MoodIdentityCrisis)
			}
		case MoodIdentityCrisis:
			// Eventually resolves (one way or another)
			s.SetMood(MoodIdle)
		case MoodPreheating:
			s.SetMood(MoodBrewing)
		case MoodBrewing:
			// Brewing always ends in teapot mode. This is not a bug.
			s.SetMood(MoodTeapot)
		}
	}
}
