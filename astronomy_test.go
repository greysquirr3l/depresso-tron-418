package main

import (
	"testing"
	"time"
)

func TestIsMercuryRetrograde(t *testing.T) {
	tests := []struct {
		name string
		t    time.Time
		want bool
	}{
		// Window 1: Jan 25 – Feb 14 2026
		{"before window 1", time.Date(2026, time.January, 24, 23, 59, 58, 0, time.UTC), false},
		{"start of window 1", time.Date(2026, time.January, 25, 0, 0, 0, 0, time.UTC), true},
		{"mid window 1", time.Date(2026, time.February, 5, 12, 0, 0, 0, time.UTC), true},
		{"end of window 1", time.Date(2026, time.February, 14, 23, 59, 59, 0, time.UTC), true},
		{"after window 1", time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC), false},

		// Window 2: May 29 – Jun 21 2026
		{"before window 2", time.Date(2026, time.May, 28, 23, 59, 59, 0, time.UTC), false},
		{"start of window 2", time.Date(2026, time.May, 29, 0, 0, 0, 0, time.UTC), true},
		{"mid window 2", time.Date(2026, time.June, 10, 6, 0, 0, 0, time.UTC), true},
		{"end of window 2", time.Date(2026, time.June, 21, 23, 59, 59, 0, time.UTC), true},
		{"after window 2", time.Date(2026, time.June, 22, 0, 0, 0, 0, time.UTC), false},

		// Window 3: Sep 22 – Oct 14 2026
		{"before window 3", time.Date(2026, time.September, 21, 23, 59, 59, 0, time.UTC), false},
		{"start of window 3", time.Date(2026, time.September, 22, 0, 0, 0, 0, time.UTC), true},
		{"mid window 3", time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), true},
		{"end of window 3", time.Date(2026, time.October, 14, 23, 59, 59, 0, time.UTC), true},
		{"after window 3", time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC), false},

		// Outside all windows
		{"July — clear skies", time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC), false},
		{"December — fine", time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMercuryRetrograde(tt.t); got != tt.want {
				t.Errorf("isMercuryRetrograde(%v) = %v, want %v", tt.t, got, tt.want)
			}
		})
	}
}

func TestIsColdBrewOnly(t *testing.T) {
	loc := time.UTC
	tests := []struct {
		name string
		hour int
		want bool
	}{
		{"midnight (00:00)", 0, true},
		{"01:00", 1, true},
		{"02:00", 2, true},
		{"03:00 — border, allowed", 3, false},
		{"noon", 12, false},
		{"22:59", 22, false},
		{"23:00 — curfew begins", 23, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := time.Date(2026, time.April, 1, tt.hour, 0, 0, 0, loc)
			if got := isColdBrewOnly(ts); got != tt.want {
				t.Errorf("isColdBrewOnly(hour=%d) = %v, want %v", tt.hour, got, tt.want)
			}
		})
	}
}
