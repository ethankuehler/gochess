package chess

import "testing"

func TestApplyUCIMoveDoublePush(t *testing.T) {
	position := mustFEN(t, NewStartingPosition().FEN())

	next, move, err := position.ApplyUCIMove("e2e4")
	if err != nil {
		t.Fatalf("ApplyUCIMove: %v", err)
	}
	if !move.IsDoublePush() {
		t.Fatalf("e2e4 did not carry the double-push flag: %+v", move)
	}
	if got, want := next.FEN(), "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"; got != want {
		t.Fatalf("FEN after e2e4 = %q, want %q", got, want)
	}
	if position.FEN() != NewStartingPosition().FEN() {
		t.Fatal("ApplyUCIMove mutated the original position")
	}
}

func TestApplyUCIMoveCapture(t *testing.T) {
	position := mustFEN(t, "4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1")

	next, move, err := position.ApplyUCIMove("e4d5")
	if err != nil {
		t.Fatalf("ApplyUCIMove: %v", err)
	}
	if !move.IsCapture() {
		t.Fatalf("e4d5 did not carry the capture flag: %+v", move)
	}
	if got, want := next.FEN(), "4k3/8/8/3P4/8/8/8/4K3 b - - 0 1"; got != want {
		t.Fatalf("FEN after e4d5 = %q, want %q", got, want)
	}
}

func TestApplyUCIMoveEnPassant(t *testing.T) {
	position := mustFEN(t, "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1")

	next, move, err := position.ApplyUCIMove("e5d6")
	if err != nil {
		t.Fatalf("ApplyUCIMove: %v", err)
	}
	if !move.IsEnPassant() {
		t.Fatalf("e5d6 did not carry the en passant flag: %+v", move)
	}
	if got, want := next.FEN(), "4k3/8/3P4/8/8/8/8/4K3 b - - 0 1"; got != want {
		t.Fatalf("FEN after e5d6 = %q, want %q", got, want)
	}
}

func TestApplyUCIMoveCastling(t *testing.T) {
	position := mustFEN(t, "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")

	next, move, err := position.ApplyUCIMove("e1g1")
	if err != nil {
		t.Fatalf("ApplyUCIMove: %v", err)
	}
	if !move.IsCastling() {
		t.Fatalf("e1g1 did not carry the castling flag: %+v", move)
	}
	if got, want := next.FEN(), "r3k2r/8/8/8/8/8/8/R4RK1 b kq - 1 1"; got != want {
		t.Fatalf("FEN after e1g1 = %q, want %q", got, want)
	}

	next, move, err = position.ApplyUCIMove("e1c1")
	if err != nil {
		t.Fatalf("ApplyUCIMove queenside: %v", err)
	}
	if !move.IsCastling() {
		t.Fatalf("e1c1 did not carry the castling flag: %+v", move)
	}
	if got, want := next.FEN(), "r3k2r/8/8/8/8/8/8/2KR3R b kq - 1 1"; got != want {
		t.Fatalf("FEN after e1c1 = %q, want %q", got, want)
	}
}

func TestApplyUCIMovePromotions(t *testing.T) {
	position := mustFEN(t, "8/P7/8/8/8/8/7k/4K3 w - - 0 1")

	cases := []struct {
		uci  string
		want Piece
	}{
		{"a7a8q", Queen},
		{"a7a8r", Rook},
		{"a7a8b", Bishop},
		{"a7a8n", Knight},
	}

	for _, tc := range cases {
		next, move, err := position.ApplyUCIMove(tc.uci)
		if err != nil {
			t.Fatalf("ApplyUCIMove(%q): %v", tc.uci, err)
		}
		promotion, ok := move.Promotion()
		if !ok || promotion != tc.want {
			t.Fatalf("%s promotion = %v, %v; want %v, true", tc.uci, promotion, ok, tc.want)
		}
		if got := next.PieceBitboard(White, tc.want); got&(BitBoard(1)<<SquareFromFileRank(FileRank{file: 0, rank: 7})) == 0 {
			t.Fatalf("%s did not place a %v on a8", tc.uci, tc.want)
		}
	}
}

func TestApplyUCIMovePromotionCapture(t *testing.T) {
	position := mustFEN(t, "1r6/P7/8/8/8/8/7k/4K3 w - - 0 1")

	_, move, err := position.ApplyUCIMove("a7b8n")
	if err != nil {
		t.Fatalf("ApplyUCIMove: %v", err)
	}
	promotion, ok := move.Promotion()
	if !ok || promotion != Knight {
		t.Fatalf("promotion = %v, %v; want Knight, true", promotion, ok)
	}
	if !move.IsCapture() {
		t.Fatal("promotion capture did not carry the capture flag")
	}
}

func TestApplyUCIMoveRejectsIllegalAndMalformed(t *testing.T) {
	position := mustFEN(t, NewStartingPosition().FEN())

	for _, uci := range []string{"e2e5", "e7e5", "e2e4q", "not-a-move", "e2e2"} {
		if _, _, err := position.ApplyUCIMove(uci); err == nil {
			t.Errorf("ApplyUCIMove(%q) succeeded, want an error", uci)
		}
	}
}

func TestFindLegalMoveIgnoresNotationFlags(t *testing.T) {
	position := mustFEN(t, NewStartingPosition().FEN())

	parsed, err := ParseUCIMove("e2e4")
	if err != nil {
		t.Fatalf("ParseUCIMove: %v", err)
	}
	legal, ok := position.FindLegalMove(*parsed)
	if !ok {
		t.Fatal("FindLegalMove did not find e2e4")
	}
	if !legal.IsDoublePush() {
		t.Fatalf("FindLegalMove returned a move without the double-push flag: %+v", legal)
	}
}
