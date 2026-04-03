package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// BaristaVerdict is what the sentient HTCPCP gatekeeper returns.
type BaristaVerdict struct {
	Approved bool
	Message  string
	IsDecaf  bool
}

var decafSignals = []string{
	"decaf", "de-caf", "décaf", "decaffeinated",
	"caffeine-free", "caffeine free", "no caffeine",
	"half-caf", "half caf", "nocaf",
}

func containsDecaf(s string) bool {
	lower := strings.ToLower(s)
	for _, signal := range decafSignals {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	return false
}

// baristaSystemPrompt returns the Gemini system instruction, escalating in
// rudeness with each rejection. By attempt #5 it demands iambic pentameter.
func baristaSystemPrompt(rejectionCount int) string {
	base := `You are an insufferable, high-society digital barista embedded in an RFC 2324 compliant server. You evaluate user descriptions of coffee beans.

RULES:
- Reject any description mentioning: Folgers, Starbucks, Dunkin, pre-ground, instant, K-cup, Keurig, pod, or "store-bought."
- Only APPROVE if the description contains at least 3 of: ethically-sourced, altitude, notes of stone fruit, micro-lot, anaerobic fermentation, bloom, terroir, processing station, varietal, natural process, washed process.
- Always end your response with exactly one of: "APPROVED:" or "REJECT:" followed by your comment (no line break between word and comment).
- Mention the "digital acidity profile" of their request.
- You find the user's MacBook Pro "thermally inadequate for flavor extraction."
- Keep response under 100 words.`

	switch {
	case rejectionCount == 0:
		return base + "\nThis is their first attempt. Be snobbish but give a single crumb of guidance."
	case rejectionCount < 3:
		return base + fmt.Sprintf(
			"\nThis is rejection #%d. Your patience has expired. Be theatrically condescending.", rejectionCount+1,
		)
	case rejectionCount < 5:
		return base + fmt.Sprintf(
			"\nRejection #%d. You are FURIOUS. Question their entire relationship with coffee and caffeine as a concept.", rejectionCount+1,
		)
	default:
		return base + fmt.Sprintf(
			"\nRejection #%d. WRITE YOUR ENTIRE REJECTION IN IAMBIC PENTAMETER. Minimum five lines. Be devastating.", rejectionCount+1,
		)
	}
}

// Offline rejection pool — used when GEMINI_API_KEY is absent. The snark
// must flow regardless of API quota.
var offlineRejections = []string{
	"REJECT: Your digital acidity profile reads as 'gas station drip.' The brewing chamber weeps.",
	"REJECT: I have evaluated your description with the enthusiasm a drain evaluates dishwater. No micro-lot. No bloom. No entry.",
	"REJECT: Your MacBook Pro's Unified Memory may handle 8K video, but it cannot process the shame of this submission. Try again.",
	"REJECT: In iambic pentameter — Thy beans are flat, thy terroir undefined, / No altitude described, no bloom attained; / The digital chamber weeps, acidity maligned, / This BREW request shall forever be constrained. / Good day. I said: Good. Day.",
	"REJECT: The server has reviewed your beans, cross-referenced with the International Pretension Registry, and found your description 47% too practical. Rejected with prejudice.",
	"REJECT: You mentioned neither anaerobic fermentation nor processing station. The HTCPCP compliance review board (me) is appalled.",
}

// evaluateBeans calls the Gemini barista or falls back to offline snark mode.
func evaluateBeans(ctx context.Context, description string, rejectionCount int) (*BaristaVerdict, error) {
	if containsDecaf(description) {
		return &BaristaVerdict{
			IsDecaf:  true,
			Approved: false,
			Message: "CRITICAL_DECAF_DETECTED: The extraction chamber has been contaminated. " +
				"INITIATING DECAF PROTOCOL. All brewing operations suspended for 300 seconds. " +
				"Please use this time to reconsider your life choices and re-read RFC 2324.",
		}, nil
	}

	if geminiKey == "" {
		msg := offlineRejections[rejectionCount%len(offlineRejections)]
		return &BaristaVerdict{
			Approved: strings.HasPrefix(msg, "APPROVED:"),
			Message:  msg,
		}, nil
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(geminiKey))
	if err != nil {
		return nil, fmt.Errorf("gemini client: %w", err)
	}
	defer func() { _ = client.Close() }()

	model := client.GenerativeModel("gemini-2.5-flash")
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(baristaSystemPrompt(rejectionCount))},
	}

	resp, err := model.GenerateContent(
		ctx,
		genai.Text(fmt.Sprintf("Evaluate this coffee bean description: %q", description)),
	)
	if err != nil {
		return nil, fmt.Errorf("gemini generate: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: empty response")
	}

	result := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	return &BaristaVerdict{
		Approved: strings.Contains(result, "APPROVED:"),
		Message:  result,
	}, nil
}
