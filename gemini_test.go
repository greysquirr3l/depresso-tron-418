package main

import (
	"context"
	"strings"
	"testing"
)

func TestContainsDecaf(t *testing.T) {
	positives := []string{
		"decaf",
		"DECAF",
		"Decaffeinated",
		"de-caf latte",
		"caffeine-free",
		"caffeine free blend",
		"no caffeine please",
		"half-caf",
		"half caf",
		"nocaf blend",
		"My amazing décaf single-origin",
	}
	for _, s := range positives {
		if !containsDecaf(s) {
			t.Errorf("containsDecaf(%q) = false, want true", s)
		}
	}

	negatives := []string{
		"ethically-sourced micro-lot with notes of stone fruit",
		"anaerobic fermentation, altitude 1800m, natural process",
		"single-origin washed Ethiopian varietal",
		"bloom technique, terroir-forward",
		"",
	}
	for _, s := range negatives {
		if containsDecaf(s) {
			t.Errorf("containsDecaf(%q) = true, want false", s)
		}
	}
}

func TestBaristaSystemPromptEscalates(t *testing.T) {
	tests := []struct {
		rejections int
		wantContains string
	}{
		{0, "first attempt"},
		{1, "rejection #2"},
		{2, "rejection #3"},
		{3, "FURIOUS"},
		{4, "FURIOUS"},
		{5, "IAMBIC PENTAMETER"},
		{10, "IAMBIC PENTAMETER"},
	}
	for _, tt := range tests {
		prompt := baristaSystemPrompt(tt.rejections)
		lower := strings.ToLower(prompt)
		if !strings.Contains(lower, strings.ToLower(tt.wantContains)) {
			t.Errorf("baristaSystemPrompt(%d) missing %q", tt.rejections, tt.wantContains)
		}
	}
}

func TestBaristaSystemPromptAlwaysHasBaseRules(t *testing.T) {
	for _, n := range []int{0, 1, 3, 5, 9} {
		prompt := baristaSystemPrompt(n)
		if !strings.Contains(prompt, "digital acidity profile") {
			t.Errorf("baristaSystemPrompt(%d): missing 'digital acidity profile'", n)
		}
		if !strings.Contains(prompt, "APPROVED:") {
			t.Errorf("baristaSystemPrompt(%d): missing APPROVED: instruction", n)
		}
		if !strings.Contains(prompt, "REJECT:") {
			t.Errorf("baristaSystemPrompt(%d): missing REJECT: instruction", n)
		}
	}
}

func TestEvaluateBeansDecafTriggersDeclination(t *testing.T) {
	// geminiKey is empty in tests — decaf is caught before the API call anyway.
	verdict, err := evaluateBeans(context.Background(), "I want a nice decaf please", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verdict.IsDecaf {
		t.Error("IsDecaf = false, want true")
	}
	if verdict.Approved {
		t.Error("Approved = true for decaf, want false")
	}
	if !strings.Contains(verdict.Message, "DECAF") {
		t.Errorf("Message %q does not mention DECAF", verdict.Message)
	}
}

func TestEvaluateBeansOfflineRotatesRejections(t *testing.T) {
	// With no gemini key, evaluateBeans cycles through offlineRejections.
	oldKey := geminiKey
	geminiKey = ""
	defer func() { geminiKey = oldKey }()

	seen := map[string]bool{}
	for i := range offlineRejections {
		v, err := evaluateBeans(context.Background(), "some normal beans", i)
		if err != nil {
			t.Fatalf("rejection %d: unexpected error: %v", i, err)
		}
		if seen[v.Message] {
			continue
		}
		seen[v.Message] = true
	}
	if len(seen) != len(offlineRejections) {
		t.Errorf("expected %d unique offline messages, got %d", len(offlineRejections), len(seen))
	}
}

func TestEvaluateBeansOfflineApprovedFlag(t *testing.T) {
	oldKey := geminiKey
	geminiKey = ""
	defer func() { geminiKey = oldKey }()

	for i, msg := range offlineRejections {
		v, _ := evaluateBeans(context.Background(), "beans", i)
		wantApproved := strings.HasPrefix(msg, "APPROVED:")
		if v.Approved != wantApproved {
			t.Errorf("offline rejection %d: Approved = %v, want %v", i, v.Approved, wantApproved)
		}
	}
}
