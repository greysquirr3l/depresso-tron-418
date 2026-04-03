package main

import "testing"

func TestIsMilkSpoiled(t *testing.T) {
	tests := []struct {
		temp float64
		want bool
	}{
		{0.0, false},
		{69.9, false},
		{70.0, false},  // boundary — exactly 70 is still safe
		{70.1, true},
		{100.0, true},
	}
	for _, tt := range tests {
		if got := isMilkSpoiled(tt.temp); got != tt.want {
			t.Errorf("isMilkSpoiled(%.1f) = %v, want %v", tt.temp, got, tt.want)
		}
	}
}

func TestIsTooHumidForExtraction(t *testing.T) {
	tests := []struct {
		precip float64
		want   bool
	}{
		{0.0, false},
		{0.001, true},
		{1.0, true},
		{25.0, true},
	}
	for _, tt := range tests {
		if got := isTooHumidForExtraction(tt.precip); got != tt.want {
			t.Errorf("isTooHumidForExtraction(%.3f) = %v, want %v", tt.precip, got, tt.want)
		}
	}
}

func TestWhenWindowSecond(t *testing.T) {
	tests := []struct {
		wind float64
		want int
	}{
		{0.0, 0},
		{1.0, 1},
		{59.0, 59},
		{60.0, 0},
		{61.0, 1},
		{75.5, 15},  // int(75.5) = 75, 75%60 = 15
		{120.0, 0},
		{121.9, 1},
	}
	for _, tt := range tests {
		if got := whenWindowSecond(tt.wind); got != tt.want {
			t.Errorf("whenWindowSecond(%.1f) = %d, want %d", tt.wind, got, tt.want)
		}
	}
}
