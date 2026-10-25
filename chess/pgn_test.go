package chess

import (
	"strings"
	"testing"
)

func TestSAN(t *testing.T) {
	tests := []struct {
		name string
		fen  string
		uci  string
		want string
	}{
		{"pawn push", NewStartingPosition().FEN(), "e2e4", "e4"},
		{"knight", NewStartingPosition().FEN(), "g1f3", "Nf3"},
		{"capture", "4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1", "e4d5", "exd5"},
		{"en passant", "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1", "e5d6", "exd6"},
		{"kingside castling", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1g1", "O-O"},
		{"queenside castling", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1c1", "O-O-O"},
		{"promotion", "8/P7/8/8/8/8/7k/4K3 w - - 0 1", "a7a8q", "a8=Q"},
		{"underpromotion", "8/P7/8/8/8/8/7k/4K3 w - - 0 1", "a7a8n", "a8=N"},
		{"promotion capture", "1n6/P7/8/8/k7/8/8/4K3 w - - 0 1", "a7b8q", "axb8=Q"},
		{"check", "4k3/8/8/8/8/8/8/4K2R w - - 0 1", "h1h8", "Rh8+"},
		{"checkmate", "6k1/5ppp/8/8/8/8/8/R6K w - - 0 1", "a1a8", "Ra8#"},
		{"file disambiguation", "4k3/8/8/8/8/2N1N3/8/4K3 w - - 0 1", "c3d5", "Ncd5"},
		{"rank disambiguation", "4k3/8/8/8/8/R7/8/R3K3 w - - 0 1", "a1a2", "R1a2"},
		{"full disambiguation", "4k3/8/8/8/8/Q7/8/Q1Q4K w - - 0 1", "a1c3", "Qa1c3"},
		{"black piece", "4k3/8/8/8/8/2n1n3/8/4K3 b - - 0 1", "c3d5", "Ncd5"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			position := mustFEN(t, test.fen)
			move, err := ParseUCIMove(test.uci)
			if err != nil {
				t.Fatalf("ParseUCIMove(%q): %v", test.uci, err)
			}
			got, err := position.SAN(*move)
			if err != nil {
				t.Fatalf("SAN(%q): %v", test.uci, err)
			}
			if got != test.want {
				t.Errorf("SAN(%q) = %q, want %q", test.uci, got, test.want)
			}
		})
	}
}

func TestSANRejectsIllegalMove(t *testing.T) {
	position := NewStartingPosition()
	move, err := ParseUCIMove("e2e5")
	if err != nil {
		t.Fatalf("ParseUCIMove: %v", err)
	}
	if _, err := position.SAN(*move); err == nil {
		t.Fatal("SAN accepted an illegal move")
	}
}

func playMoves(t *testing.T, start *Position, uciMoves ...string) []Move {
	t.Helper()
	moves := make([]Move, 0, len(uciMoves))
	position := start
	for _, uci := range uciMoves {
		next, move, err := position.ApplyUCIMove(uci)
		if err != nil {
			t.Fatalf("ApplyUCIMove(%q): %v", uci, err)
		}
		moves = append(moves, move)
		position = next
	}
	return moves
}

func TestPGN(t *testing.T) {
	start := NewStartingPosition()
	moves := playMoves(t, start, "e2e4", "e7e5", "g1f3", "b8c6", "f1b5", "a7a6")

	got, err := PGN(start, moves, PGNHeader{
		Event:  "Test",
		Site:   "gochess web",
		Date:   "2026.10.24",
		Round:  "-",
		White:  "?",
		Black:  "?",
		Result: "*",
	})
	if err != nil {
		t.Fatalf("PGN: %v", err)
	}

	const want = `[Event "Test"]
[Site "gochess web"]
[Date "2026.10.24"]
[Round "-"]
[White "?"]
[Black "?"]
[Result "*"]

1. e4 e5 2. Nf3 Nc6 3. Bb5 a6 *
`
	if got != want {
		t.Errorf("PGN =\n%s\nwant\n%s", got, want)
	}
}

func TestPGNCheckmate(t *testing.T) {
	start := NewStartingPosition()
	moves := playMoves(t, start, "e2e4", "e7e5", "f1c4", "b8c6", "d1h5", "g8f6", "h5f7")

	got, err := PGN(start, moves, PGNHeader{Result: "1-0"})
	if err != nil {
		t.Fatalf("PGN: %v", err)
	}
	if !strings.Contains(got, "1. e4 e5 2. Bc4 Nc6 3. Qh5 Nf6 4. Qxf7# 1-0\n") {
		t.Errorf("PGN does not contain Scholar's Mate movetext:\n%s", got)
	}
	if !strings.Contains(got, `[Result "1-0"]`) {
		t.Errorf("PGN does not contain the result tag:\n%s", got)
	}
}

func TestPGNBlackToMove(t *testing.T) {
	start := mustFEN(t, "4k3/8/8/8/8/8/8/4K2R b - - 7 42")
	moves := playMoves(t, start, "e8d8")

	got, err := PGN(start, moves, PGNHeader{Event: "Black to move"})
	if err != nil {
		t.Fatalf("PGN: %v", err)
	}
	if !strings.Contains(got, `[SetUp "1"]`+"\n"+`[FEN "4k3/8/8/8/8/8/8/4K2R b - - 7 42"]`) {
		t.Errorf("PGN is missing SetUp/FEN tags:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n\n42... Kd8 *\n") {
		t.Errorf("PGN movetext = %q, want 42... Kd8 *", got)
	}
}

func TestPGNEmptyGameUsesDefaults(t *testing.T) {
	got, err := PGN(NewStartingPosition(), nil, PGNHeader{})
	if err != nil {
		t.Fatalf("PGN: %v", err)
	}
	for _, tag := range []string{
		`[Event "?"]`,
		`[Site "?"]`,
		`[Date "????.??.??"]`,
		`[Round "?"]`,
		`[White "?"]`,
		`[Black "?"]`,
		`[Result "*"]`,
	} {
		if !strings.Contains(got, tag) {
			t.Errorf("PGN is missing %s:\n%s", tag, got)
		}
	}
	if strings.Contains(got, `[SetUp "1"]`) {
		t.Errorf("standard starting position should not use SetUp:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n\n*\n") {
		t.Errorf("PGN movetext = %q, want *", got)
	}
}

func TestPGNEscapesTags(t *testing.T) {
	got, err := PGN(NewStartingPosition(), nil, PGNHeader{
		Event: `A "quoted" \ event`,
		Site:  "line\nbreak",
	})
	if err != nil {
		t.Fatalf("PGN: %v", err)
	}
	if !strings.Contains(got, `[Event "A \"quoted\" \\ event"]`) {
		t.Errorf("quoted Event tag was not escaped:\n%s", got)
	}
	if !strings.Contains(got, `[Site "line break"]`) {
		t.Errorf("line break in Site tag was not folded:\n%s", got)
	}
}

func TestPGNWrapsLongMovetext(t *testing.T) {
	start := NewStartingPosition()
	uciMoves := make([]string, 0, 40)
	for i := 0; i < 20; i++ {
		uciMoves = append(uciMoves, "g1f3", "g8f6", "f3g1", "f6g8")
	}
	moves := playMoves(t, start, uciMoves...)
	got, err := PGN(start, moves, PGNHeader{Result: "*"})
	if err != nil {
		t.Fatalf("PGN: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	movetextStarted := false
	movetextLines := 0
	for _, line := range lines {
		if line == "" {
			movetextStarted = true
			continue
		}
		if !movetextStarted {
			continue
		}
		movetextLines++
		if len(line) > 80 {
			t.Errorf("movetext line is %d columns, want at most 80: %q", len(line), line)
		}
	}
	if movetextLines < 2 {
		t.Fatalf("movetext was not wrapped: %d lines", movetextLines)
	}
}

func TestPGNErrors(t *testing.T) {
	move, err := ParseUCIMove("e2e5")
	if err != nil {
		t.Fatalf("ParseUCIMove: %v", err)
	}

	if _, err := PGN(nil, nil, PGNHeader{}); err == nil {
		t.Error("PGN accepted a nil start position")
	}
	if _, err := PGN(NewStartingPosition(), nil, PGNHeader{Result: "win"}); err == nil {
		t.Error("PGN accepted an invalid result")
	}
	if _, err := PGN(NewStartingPosition(), []Move{*move}, PGNHeader{}); err == nil {
		t.Error("PGN accepted an illegal move")
	}
}
