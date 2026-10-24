package uci

import (
	"testing"
)

func TestParsePositionTracksRepetitionHistory(t *testing.T) {
	position, history, err := parsePosition([]string{
		"fen", "4k3/8/8/8/8/8/8/R3K3", "w", "-", "-", "0", "1", "moves",
		"e1d1", "e8d8", "d1e1", "d8e8",
		"e1d1", "e8d8", "d1e1", "d8e8",
	})
	if err != nil {
		t.Fatalf("parsePosition: %v", err)
	}
	if !position.IsThreefoldRepetition(history) {
		t.Fatal("parsePosition did not record a threefold repetition")
	}
}

func TestClientUsesPositionHistoryForDraws(t *testing.T) {
	client := startTestClient(t)

	const root = "4k3/8/8/8/8/8/8/R3K3 w - - 0 1"
	moves := []string{
		"e1d1", "e8d8", "d1e1", "d8e8",
		"e1d1", "e8d8", "d1e1", "d8e8",
	}
	if err := client.SetPosition(root, moves...); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}

	result, err := client.GoDepth(1)
	if err != nil {
		t.Fatalf("GoDepth: %v", err)
	}
	if result.BestMove == "" {
		t.Fatal("drawn position returned no best move")
	}
	if result.Score != 0 {
		t.Errorf("threefold position score = %d, want 0", result.Score)
	}
}
