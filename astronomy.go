// Package main — see main.go for the entry point.
package main

import "time"

// Mercury retrograde periods for 2026 (UTC, approximate).
// We take these seriously. The server does not.
var mercuryRetrogradePeriods2026 = [][2]time.Time{
	{
		time.Date(2026, time.January, 25, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.February, 14, 23, 59, 59, 0, time.UTC),
	},
	{
		time.Date(2026, time.May, 29, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.June, 21, 23, 59, 59, 0, time.UTC),
	},
	{
		time.Date(2026, time.September, 22, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.October, 14, 23, 59, 59, 0, time.UTC),
	},
}

// isMercuryRetrograde returns true if the given time falls within a known
// retrograde window. Attempting to BREW during retrograde is inadvisable;
// the server will refuse with a 503 and a sympathetic but firm message
// about celestial interference with digital extraction.
func isMercuryRetrograde(t time.Time) bool {
	tUTC := t.UTC()
	for _, period := range mercuryRetrogradePeriods2026 {
		if !tUTC.Before(period[0]) && !tUTC.After(period[1]) {
			return true
		}
	}
	return false
}

// isColdBrewOnly returns true between 23:00 and 03:00 local time.
// Per RFC 9999 §Circadian (self-ratified), the server's heating element
// observes mandatory rest hours. Only cold brew is available during this window,
// and cold brew takes 12 hours, so you won't be seeing any coffee tonight.
func isColdBrewOnly(t time.Time) bool {
	h := t.Hour()
	return h >= 23 || h < 3
}
