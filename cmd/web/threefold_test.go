package main

import (
	"strings"
	"testing"
)

// TestReportedGameThreefoldIsClaimable replays the computer-vs-computer game
// from the report and checks that the third occurrence of the repeated position
// is claimable but does not end the game by itself.
func TestReportedGameThreefoldIsClaimable(t *testing.T) {
	g := newGame()
	t.Cleanup(g.Close)

	moves := strings.Fields(`e2e4 e7e5 g1f3 b8c6 f1c4 f8c5 b1c3 g8f6 d2d3 d7d6 c1g5 h7h6 g5f6 g7f6 c3a4 c8g4 h2h3 g4f3 g2f3 a8b8 c4b5 a7a6 b5c6 b7c6 a2a3 c5d4 c2c3 d4a7 d1c2 d8d7 e1c1 d7e6 c1b1 e6b3 c2b3 b8b3 d1d2 e8g8 b1a2 f8b8 h1g1 g8h7 g1g4 b3b5 b2b4 b8g8 g4h4 b5b7 f3f4 e5f4 h4f4 g8g6 f4f5 b7b5 h3h4 b5f5 e4f5 g6g4 d2e2 g4f4 e2e7 f4f5 e7f7 h7h8 f7c7 a7f2 c7c8 h8h7 c8c7 h7h8 c7c6 f2h4 c6d6 f5f3 a4c5 f3f2 a2b3 f2f1 d6a6 h8h7 a6a7 h7g8 a3a4 f1b1 b3c2 b1e1 a7a8 g8g7 a8a7 g7h8 a7a8 h8g7 c5e4 e1e2 c2c1 f6f5 a8a7 g7h8 a7a8 h8h7 a8a7 h7h8 a7a8 h8h7 e4d6 h4g5 c1b1 f5f4 a8a7 h7h8 d6f7 h8g8 f7g5 e2e1 b1c2 h6g5 a7a5 e1g1 a5a8 g8g7 a8a7 g7g8 a7a8 g8g7 a4a5 f4f3 a8a7 g7g8 a7a8 g8g7 a8a7 g7f6 a7a6 f6g7 a6a7`)
	for i, uci := range moves {
		if err := g.applyUCILocked(uci); err != nil {
			t.Fatalf("move %d %q: %v", i+1, uci, err)
		}
	}

	state := g.snapshotLocked()
	if state.Status != "playing" {
		t.Fatalf("status = %q, want playing (threefold is claimable)", state.Status)
	}
	if !state.ThreefoldClaim {
		t.Fatal("threefold claim is not available")
	}
	if err := g.claimDrawLocked(); err != nil {
		t.Fatalf("claimDrawLocked: %v", err)
	}
	if status := g.statusLocked(); status != "threefold" {
		t.Fatalf("status after claim = %q, want threefold", status)
	}
}
