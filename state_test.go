package main

import (
	"sync"
	"testing"
)

func TestAppStateDefaultMoodIsIdle(t *testing.T) {
	s := newAppState()
	if s.GetMood() != MoodIdle {
		t.Errorf("default mood = %v, want MoodIdle", s.GetMood())
	}
}

func TestAppStateSetAndGetMood(t *testing.T) {
	moods := []Mood{MoodIdle, MoodPreheating, MoodBrewing, MoodTeapot, MoodIdentityCrisis}
	s := newAppState()
	for _, m := range moods {
		s.SetMood(m)
		if got := s.GetMood(); got != m {
			t.Errorf("SetMood(%v) then GetMood() = %v", m, got)
		}
	}
}

func TestAppStateGetLabel(t *testing.T) {
	s := newAppState()
	for mood, want := range moodLabels {
		s.SetMood(mood)
		if got := s.GetLabel(); got != want {
			t.Errorf("mood %v: GetLabel() = %q, want %q", mood, got, want)
		}
	}
}

func TestAppStateGetClass(t *testing.T) {
	s := newAppState()
	for mood, want := range moodClasses {
		s.SetMood(mood)
		if got := s.GetClass(); got != want {
			t.Errorf("mood %v: GetClass() = %q, want %q", mood, got, want)
		}
	}
}

func TestAppStateConcurrentReadWrite(_ *testing.T) {
	s := newAppState()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.SetMood(MoodBrewing)
		}()
		go func() {
			defer wg.Done()
			_ = s.GetMood()
		}()
	}
	wg.Wait()
}

func TestMoodLabelsAllPresent(t *testing.T) {
	allMoods := []Mood{MoodIdle, MoodPreheating, MoodBrewing, MoodTeapot, MoodIdentityCrisis}
	for _, m := range allMoods {
		if _, ok := moodLabels[m]; !ok {
			t.Errorf("moodLabels missing entry for Mood(%d)", m)
		}
		if _, ok := moodClasses[m]; !ok {
			t.Errorf("moodClasses missing entry for Mood(%d)", m)
		}
	}
}
