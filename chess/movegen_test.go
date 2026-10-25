package chess

import (
	"slices"
	"testing"
)

func moveNames(moves []Move) []string {
	names := make([]string, len(moves))
	for i, m := range moves {
		names[i] = m.String()
	}
	slices.Sort(names)
	return names
}

func moveSet(moves []Move) map[string]Move {
	set := make(map[string]Move, len(moves))
	for _, m := range moves {
		set[m.String()] = m
	}
	return set
}

func requireMoves(t *testing.T, moves []Move, want ...string) {
	t.Helper()
	set := moveSet(moves)
	for _, uci := range want {
		if _, ok := set[uci]; !ok {
			t.Errorf("missing move %s; got %v", uci, moveNames(moves))
		}
	}
}

func requireNoMoves(t *testing.T, moves []Move, notWant ...string) {
	t.Helper()
	set := moveSet(moves)
	for _, uci := range notWant {
		if _, ok := set[uci]; ok {
			t.Errorf("unexpected move %s; got %v", uci, moveNames(moves))
		}
	}
}

func findMove(moves []Move, uci string) (Move, bool) {
	for _, m := range moves {
		if m.String() == uci {
			return m, true
		}
	}
	return Move{}, false
}

func movesFrom(moves []Move, from Square) []Move {
	var result []Move
	for _, m := range moves {
		if m.From() == from {
			result = append(result, m)
		}
	}
	return result
}

func TestIsSquareAttacked(t *testing.T) {
	cases := []struct {
		name string
		fen  string
		by   Colour
		want map[string]bool
	}{
		{
			name: "white pawn",
			fen:  "8/8/8/8/4P3/8/8/4K2k w - - 0 1",
			by:   White,
			want: map[string]bool{"d5": true, "f5": true, "e5": false},
		},
		{
			name: "black pawn",
			fen:  "8/8/8/3p4/4P3/8/8/4K2k w - - 0 1",
			by:   Black,
			want: map[string]bool{"c4": true, "e4": true, "c5": false},
		},
		{
			name: "knight",
			fen:  "8/8/8/8/8/5N2/8/4K2k w - - 0 1",
			by:   White,
			want: map[string]bool{"d4": true, "e5": true, "g5": true, "e4": false},
		},
		{
			name: "bishop blocked",
			fen:  "6k1/8/8/8/3P4/8/8/B3K3 w - - 0 1",
			by:   White,
			want: map[string]bool{"b2": true, "c3": true, "d4": true, "f6": false},
		},
		{
			name: "rook",
			fen:  "7k/8/8/8/8/8/8/R3K3 w - - 0 1",
			by:   White,
			want: map[string]bool{"a8": true, "d1": true, "a4": true, "h1": false},
		},
		{
			name: "queen",
			fen:  "7k/8/8/8/8/8/8/K2Q4 w - - 0 1",
			by:   White,
			want: map[string]bool{"d8": true, "h5": true, "a4": true, "b3": true, "b4": false},
		},
		{
			name: "king",
			fen:  "7k/8/8/8/8/8/8/4K3 w - - 0 1",
			by:   White,
			want: map[string]bool{"d1": true, "f2": true, "e3": false},
		},
		{
			name: "white pawn attacks back rank",
			fen:  "rnbq1k1r/pp1Pbppp/2p5/8/2B5/P7/1PP1NnPP/RNBQK2R b KQ - 0 8",
			by:   White,
			want: map[string]bool{"c8": true, "e8": true, "e7": false},
		},
		{
			name: "black pawn attacks first rank",
			fen:  "k7/8/8/8/8/8/4p3/7K b - - 0 1",
			by:   Black,
			want: map[string]bool{"d1": true, "f1": true, "e1": false},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := mustFEN(t, tc.fen)
			for square, want := range tc.want {
				got := b.isSquareAttacked(mustSquare(t, square), tc.by)
				if got != want {
					t.Errorf("isSquareAttacked(%s, %v) = %v, want %v", square, tc.by, got, want)
				}
			}
		})
	}
}

func TestLegalMovesStartPosition(t *testing.T) {
	got := NewStartingPosition().LegalMoves()
	want := []string{
		"a2a3", "a2a4", "b1a3", "b1c3", "b2b3", "b2b4", "c2c3", "c2c4",
		"d2d3", "d2d4", "e2e3", "e2e4", "f2f3", "f2f4", "g1f3", "g1h3",
		"g2g3", "g2g4", "h2h3", "h2h4",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d legal moves, want %d: %v", len(got), len(want), moveNames(got))
	}
	requireMoves(t, got, want...)

	seen := make(map[string]bool, len(got))
	for _, m := range got {
		if seen[m.String()] {
			t.Errorf("duplicate move %s", m)
		}
		seen[m.String()] = true
	}
}

func TestCastlingMoves(t *testing.T) {
	cases := []struct {
		name    string
		fen     string
		want    []string
		notWant []string
	}{
		{
			name: "both white castles",
			fen:  "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
			want: []string{"e1g1", "e1c1"},
		},
		{
			name:    "kingside blocked by attacked f1",
			fen:     "r3kr2/8/8/8/8/8/8/R3K2R w KQ - 0 1",
			want:    []string{"e1c1"},
			notWant: []string{"e1g1"},
		},
		{
			name:    "queenside blocked by attacked d1",
			fen:     "r2rk2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
			want:    []string{"e1g1"},
			notWant: []string{"e1c1"},
		},
		{
			name: "b1 attacked does not prevent queenside",
			fen:  "1r2k2r/8/8/8/8/8/8/R3K2R w KQk - 0 1",
			want: []string{"e1g1", "e1c1"},
		},
		{
			name:    "cannot castle out of check",
			fen:     "4r1k1/8/8/8/8/8/8/R3K2R w KQ - 0 1",
			notWant: []string{"e1g1", "e1c1"},
		},
		{
			name:    "kingside needs rook",
			fen:     "4k3/8/8/8/8/8/8/R3K3 w KQ - 0 1",
			want:    []string{"e1c1"},
			notWant: []string{"e1g1"},
		},
		{
			name:    "bishop blocks kingside",
			fen:     "r3k2r/8/8/8/8/8/8/R3KB1R w KQkq - 0 1",
			want:    []string{"e1c1"},
			notWant: []string{"e1g1"},
		},
		{
			name:    "knight blocks queenside",
			fen:     "r3k2r/8/8/8/8/8/8/RN2K2R w KQkq - 0 1",
			want:    []string{"e1g1"},
			notWant: []string{"e1c1"},
		},
		{
			name: "black castles",
			fen:  "r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1",
			want: []string{"e8g8", "e8c8"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			moves := mustFEN(t, tc.fen).LegalMoves()
			requireMoves(t, moves, tc.want...)
			requireNoMoves(t, moves, tc.notWant...)
		})
	}
}

func TestEnPassantMoves(t *testing.T) {
	t.Run("available", func(t *testing.T) {
		b := mustFEN(t, "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1")
		moves := b.LegalMoves()
		m, ok := findMove(moves, "e5d6")
		if !ok {
			t.Fatalf("missing e5d6; got %v", moveNames(moves))
		}
		if !m.IsCapture() || !m.IsEnPassant() {
			t.Errorf("e5d6 flags = %016b, want capture and en passant", m.flags)
		}
	})

	t.Run("pinned en passant is illegal", func(t *testing.T) {
		b := mustFEN(t, "4r1k1/8/8/3pP3/8/8/8/4K3 w - d6 0 1")
		moves := b.LegalMoves()
		requireNoMoves(t, moves, "e5d6")
	})

	t.Run("no en passant square", func(t *testing.T) {
		b := mustFEN(t, "4k3/8/8/3pP3/8/8/8/4K3 w - - 0 1")
		moves := b.LegalMoves()
		requireNoMoves(t, moves, "e5d6")
	})

	t.Run("black en passant", func(t *testing.T) {
		b := mustFEN(t, "4k3/8/8/8/3pP3/8/8/4K3 b - e3 0 1")
		moves := b.LegalMoves()
		m, ok := findMove(moves, "d4e3")
		if !ok {
			t.Fatalf("missing d4e3; got %v", moveNames(moves))
		}
		if !m.IsEnPassant() {
			t.Error("d4e3 is not flagged as en passant")
		}
	})

	t.Run("en passant gives check and is legal", func(t *testing.T) {
		b := mustFEN(t, "8/8/1k6/2b5/2pP4/8/5K2/8 b - d3 0 1")
		moves := b.LegalMoves()
		m, ok := findMove(moves, "c4d3")
		if !ok {
			t.Fatalf("missing c4d3; got %v", moveNames(moves))
		}
		if !m.IsEnPassant() {
			t.Error("c4d3 is not flagged as en passant")
		}
	})
}

func TestPromotionMoves(t *testing.T) {
	t.Run("white push promotions", func(t *testing.T) {
		b := mustFEN(t, "8/P7/8/8/8/8/8/K6k w - - 0 1")
		moves := b.LegalMoves()
		promotions := []struct {
			uci   string
			piece Piece
		}{
			{"a7a8q", Queen}, {"a7a8r", Rook}, {"a7a8b", Bishop}, {"a7a8n", Knight},
		}
		for _, tc := range promotions {
			m, ok := findMove(moves, tc.uci)
			if !ok {
				t.Errorf("missing %s; got %v", tc.uci, moveNames(moves))
				continue
			}
			promotion, ok := m.Promotion()
			if !ok || promotion != tc.piece {
				t.Errorf("%s Promotion() = %v, %v; want %v, true", tc.uci, promotion, ok, tc.piece)
			}
			if m.IsCapture() {
				t.Errorf("%s unexpectedly flagged as a capture", tc.uci)
			}
		}
		requireNoMoves(t, moves, "a7a8")
	})

	t.Run("white capture promotions", func(t *testing.T) {
		b := mustFEN(t, "1n6/P7/8/8/8/8/8/K6k w - - 0 1")
		moves := b.LegalMoves()
		for _, uci := range []string{"a7b8q", "a7b8r", "a7b8b", "a7b8n"} {
			m, ok := findMove(moves, uci)
			if !ok {
				t.Errorf("missing %s; got %v", uci, moveNames(moves))
				continue
			}
			if !m.IsCapture() {
				t.Errorf("%s not flagged as a capture", uci)
			}
			if _, ok := m.Promotion(); !ok {
				t.Errorf("%s not flagged as a promotion", uci)
			}
		}
		requireNoMoves(t, moves, "a7b8")
	})

	t.Run("black promotions", func(t *testing.T) {
		b := mustFEN(t, "k6K/8/8/8/8/8/p7/8 b - - 0 1")
		moves := b.LegalMoves()
		for _, uci := range []string{"a2a1q", "a2a1r", "a2a1b", "a2a1n"} {
			if _, ok := findMove(moves, uci); !ok {
				t.Errorf("missing %s; got %v", uci, moveNames(moves))
			}
		}
	})
}

func TestRayBetween(t *testing.T) {
	cases := []struct {
		to      string
		between []string
	}{
		{"h8", []string{"e5", "f6", "g7"}},
		{"a1", []string{"c3", "b2"}},
		{"h4", []string{"e4", "f4", "g4"}},
		{"a4", []string{"c4", "b4"}},
		{"d8", []string{"d5", "d6", "d7"}},
		{"d1", []string{"d3", "d2"}},
		{"g1", []string{"e3", "f2"}},
		{"a7", []string{"c5", "b6"}},
	}
	for _, tc := range cases {
		t.Run("d4_"+tc.to, func(t *testing.T) {
			got, ok := rayBetween(mustSquare(t, "d4"), mustSquare(t, tc.to))
			if !ok {
				t.Fatal("aligned squares were rejected")
			}
			want, err := SquaresToBitBoard(tc.between)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("between =\n%s\nwant =\n%s", got, want)
			}
		})
	}
	for _, to := range []string{"d4", "e6"} {
		if _, ok := rayBetween(mustSquare(t, "d4"), mustSquare(t, to)); ok {
			t.Errorf("d4 to %s unexpectedly produced a ray", to)
		}
	}
}

func TestLegalContextCheckers(t *testing.T) {
	cases := []struct {
		name     string
		fen      string
		checkers []string
		target   []string
	}{
		{"rook", "4r1k1/8/8/8/8/8/8/4K3 w - - 0 1", []string{"e8"}, []string{"e2", "e3", "e4", "e5", "e6", "e7", "e8"}},
		{"bishop", "6k1/8/8/8/1b6/8/8/4K3 w - - 0 1", []string{"b4"}, []string{"b4", "c3", "d2"}},
		{"knight", "6k1/8/8/8/8/5n2/8/4K3 w - - 0 1", []string{"f3"}, []string{"f3"}},
		{"pawn", "6k1/8/8/8/8/8/3p4/4K3 w - - 0 1", []string{"d2"}, []string{"d2"}},
		{"double", "4r1k1/8/8/8/1b6/8/8/4K3 w - - 0 1", []string{"b4", "e8"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := mustFEN(t, tc.fen).newLegalContext()
			wantCheckers, err := SquaresToBitBoard(tc.checkers)
			if err != nil {
				t.Fatal(err)
			}
			if ctx.checkers != wantCheckers || ctx.checkerCount != len(tc.checkers) {
				t.Errorf("checkers = %v/%d, want %v/%d", ctx.checkers, ctx.checkerCount, wantCheckers, len(tc.checkers))
			}
			if tc.target != nil {
				wantTarget, err := SquaresToBitBoard(tc.target)
				if err != nil {
					t.Fatal(err)
				}
				if ctx.checkTarget != wantTarget {
					t.Errorf("check target =\n%s\nwant =\n%s", ctx.checkTarget, wantTarget)
				}
			}
		})
	}
}

func TestLegalContextPins(t *testing.T) {
	t.Run("one rook blocker is pinned", func(t *testing.T) {
		ctx := mustFEN(t, "4r1k1/8/8/8/8/8/4R3/4K3 w - - 0 1").newLegalContext()
		want, err := SquaresToBitBoard([]string{"e2", "e3", "e4", "e5", "e6", "e7", "e8"})
		if err != nil {
			t.Fatal(err)
		}
		if got := ctx.pinMasks[mustSquare(t, "e2")]; got != want {
			t.Errorf("pin mask =\n%s\nwant =\n%s", got, want)
		}
	})
	t.Run("one diagonal blocker is pinned", func(t *testing.T) {
		ctx := mustFEN(t, "k6b/6P1/8/8/8/8/8/K7 w - - 0 1").newLegalContext()
		want, err := SquaresToBitBoard([]string{"b2", "c3", "d4", "e5", "f6", "g7", "h8"})
		if err != nil {
			t.Fatal(err)
		}
		if got := ctx.pinMasks[mustSquare(t, "g7")]; got != want {
			t.Errorf("pin mask = %v, want %v", got, want)
		}
	})
	t.Run("two blockers are not pinned", func(t *testing.T) {
		ctx := mustFEN(t, "4r1k1/8/8/8/8/4R3/4R3/4K3 w - - 0 1").newLegalContext()
		if got := ctx.pinMasks[mustSquare(t, "e2")]; got != allSquares {
			t.Errorf("pin mask with two blockers = %v, want all squares", got)
		}
	})
	t.Run("unpinned piece uses all squares", func(t *testing.T) {
		ctx := NewStartingPosition().newLegalContext()
		if ctx.checkerCount != 0 || ctx.checkTarget != allSquares {
			t.Fatalf("starting context has unexpected check state: %+v", ctx)
		}
		for square, mask := range ctx.pinMasks {
			if mask != allSquares {
				t.Fatalf("square %s has unexpected pin mask %v", SquareName(Square(square)), mask)
			}
		}
	})
}

func TestPinnedPieces(t *testing.T) {
	t.Run("pinned knight has no moves", func(t *testing.T) {
		b := mustFEN(t, "4r1k1/8/8/8/8/8/4N3/4K3 w - - 0 1")
		moves := b.LegalMoves()
		if pinned := movesFrom(moves, mustSquare(t, "e2")); len(pinned) != 0 {
			t.Errorf("pinned knight has moves: %v", moveNames(pinned))
		}
	})

	t.Run("pinned rook may move along the pin", func(t *testing.T) {
		b := mustFEN(t, "4r1k1/8/8/8/8/8/4R3/4K3 w - - 0 1")
		moves := b.LegalMoves()
		requireMoves(t, moves, "e2e8")
		requireNoMoves(t, moves, "e2a2")
	})

	t.Run("king cannot step into check", func(t *testing.T) {
		b := mustFEN(t, "4r1k1/8/8/8/8/8/8/4K3 w - - 0 1")
		moves := b.LegalMoves()
		requireNoMoves(t, moves, "e1e2")
		requireMoves(t, moves, "e1d1")
	})

	t.Run("double check allows only king moves", func(t *testing.T) {
		b := mustFEN(t, "4r1k1/8/8/8/1b6/8/8/4K3 w - - 0 1")
		moves := b.LegalMoves()
		if len(moves) == 0 {
			t.Fatal("expected at least one king move in double check")
		}
		for _, m := range moves {
			if m.From() != e1 {
				t.Errorf("non-king move %s generated in double check", m)
			}
		}
	})
}

func TestEnPassantKingSafety(t *testing.T) {
	cases := []struct {
		name string
		fen  string
		move string
	}{
		{"white horizontal exposure", "4k3/8/8/K2pP2r/8/8/8/8 w - d6 0 1", "e5d6"},
		{"black horizontal exposure", "8/8/8/8/k2Pp2R/8/8/4K3 b - d3 0 1", "e4d3"},
		{"white diagonal exposure", "b6k/8/8/3pP3/8/8/8/7K w - d6 0 1", "e5d6"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireNoMoves(t, mustFEN(t, tc.fen).LegalMoves(), tc.move)
		})
	}
}

func TestEnPassantCheckEvasions(t *testing.T) {
	cases := []struct {
		name string
		fen  string
		move string
	}{
		{"capture checking pawn", "4k3/8/8/3pP3/4K3/8/8/8 w - d6 0 1", "e5d6"},
		{"white blocks sliding check", "4k3/8/r6K/3pP3/8/8/8/8 w - d6 0 1", "e5d6"},
		{"black blocks sliding check", "8/8/8/8/3Pp3/R6k/8/4K3 b - d3 0 1", "e4d3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := mustFEN(t, tc.fen)
			if !b.InCheck() {
				t.Fatal("test position is not in check")
			}
			moves := b.LegalMoves()
			requireMoves(t, moves, tc.move)
			m, _ := findMove(moves, tc.move)
			if !m.IsEnPassant() || !m.IsCapture() {
				t.Errorf("%s is not flagged as an en-passant capture", m)
			}
		})
	}
}

func TestKingCaptures(t *testing.T) {
	t.Run("undefended capture is legal", func(t *testing.T) {
		requireMoves(t, mustFEN(t, "4k3/8/8/3r4/4K3/8/8/8 w - - 0 1").LegalMoves(), "e4d5")
	})
	t.Run("defended capture is illegal", func(t *testing.T) {
		requireNoMoves(t, mustFEN(t, "4k3/8/2p5/3r4/4K3/8/8/8 w - - 0 1").LegalMoves(), "e4d5")
	})
}

func TestPinnedPromotion(t *testing.T) {
	moves := mustFEN(t, "k6b/6P1/8/8/8/8/8/K7 w - - 0 1").LegalMoves()
	requireMoves(t, moves, "g7h8q", "g7h8r", "g7h8b", "g7h8n")
	requireNoMoves(t, moves, "g7g8q", "g7g8r", "g7g8b", "g7g8n")
}

func TestBlackQueensideCastlingWithAttackedB8(t *testing.T) {
	requireMoves(t, mustFEN(t, "r3k2r/8/8/8/8/8/8/1R2K3 b KQkq - 0 1").LegalMoves(), "e8c8")
}

func TestMoveFlagsConsistent(t *testing.T) {
	positions := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
		"4r1k1/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
		"8/P7/8/8/8/8/8/K6k w - - 0 1",
		"1n6/P7/8/8/8/8/8/K6k w - - 0 1",
		"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
		"4r1k1/8/8/8/1b6/8/8/4K3 w - - 0 1",
	}

	for _, fen := range positions {
		t.Run(fen, func(t *testing.T) {
			b := mustFEN(t, fen)
			us := b.sideToMove()
			them := us.Opposite()

			for _, m := range b.LegalMoves() {
				colour, piece, ok := b.pieceAt(m.From())
				if !ok || colour != us {
					t.Errorf("%s: from square does not hold a %v piece", m, us)
					continue
				}

				toColour, _, toOccupied := b.pieceAt(m.To())
				ep := m.IsEnPassant()
				if ep {
					if piece != Pawn || !m.IsCapture() || toOccupied {
						t.Errorf("%s: invalid en passant flags or piece", m)
					}
					captured := Square(int(m.To()) - 8)
					if us == Black {
						captured = Square(int(m.To()) + 8)
					}
					cc, cp, cok := b.pieceAt(captured)
					if !cok || cc != them || cp != Pawn {
						t.Errorf("%s: en passant captured square %s has no enemy pawn", m, SquareName(captured))
					}
				} else if toOccupied {
					if toColour != them || !m.IsCapture() {
						t.Errorf("%s: capture flag does not match the board", m)
					}
				} else if m.IsCapture() {
					t.Errorf("%s: capture flag set but target is empty", m)
				}

				if m.IsCastling() {
					fromFR := FileRankFromSquare(m.From())
					toFR := FileRankFromSquare(m.To())
					if piece != King || absInt(int(toFR.File())-int(fromFR.File())) != 2 || toOccupied {
						t.Errorf("%s: invalid castling move", m)
					}
				}

				if m.IsDoublePush() {
					if piece != Pawn || absInt(int(m.To())-int(m.From())) != 16 {
						t.Errorf("%s: invalid double push", m)
					}
					middle := Square((int(m.From()) + int(m.To())) / 2)
					if _, _, occupied := b.pieceAt(middle); occupied {
						t.Errorf("%s: double push crosses an occupied square", m)
					}
				}

				promotion, isPromotion := m.Promotion()
				if isPromotion {
					if piece != Pawn || !isPromotionRank(us, m.To()) {
						t.Errorf("%s: promotion flag on a non-promotion", m)
					}
					if promotion < Knight || promotion > Queen {
						t.Errorf("%s: invalid promotion piece %v", m, promotion)
					}
				} else if piece == Pawn && isPromotionRank(us, m.To()) {
					t.Errorf("%s: pawn reaches the last rank without a promotion flag", m)
				}
			}
		})
	}
}

func TestLegalMovesDoNotMutate(t *testing.T) {
	b := mustFEN(t, "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1")
	before := b.FEN()
	_ = b.LegalMoves()
	if got := b.FEN(); got != before {
		t.Errorf("LegalMoves mutated the position: got %q, want %q", got, before)
	}
}

func TestLegalMovesNoDuplicates(t *testing.T) {
	for _, fen := range []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
		"8/P7/8/8/8/8/8/K6k w - - 0 1",
	} {
		moves := mustFEN(t, fen).LegalMoves()
		seen := make(map[string]bool, len(moves))
		for _, m := range moves {
			uci := m.String()
			if seen[uci] {
				t.Errorf("%s: duplicate move %s", fen, uci)
			}
			seen[uci] = true
		}
	}
}

func TestInCheck(t *testing.T) {
	cases := []struct {
		fen  string
		want bool
	}{
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", false},
		{"4r1k1/8/8/8/8/8/8/4K3 w - - 0 1", true},
		{"4k3/8/8/8/8/8/8/4K3 w - - 0 1", false},
		{"4k3/8/8/8/8/8/8/4RK2 b - - 0 1", true},
	}
	for _, tc := range cases {
		if got := mustFEN(t, tc.fen).InCheck(); got != tc.want {
			t.Errorf("InCheck(%q) = %v, want %v", tc.fen, got, tc.want)
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestApplyMove(t *testing.T) {
	b := NewStartingPosition()
	m, ok := findMove(b.LegalMoves(), "e2e4")
	if !ok {
		t.Fatal("missing e2e4 in the starting position")
	}

	next, err := b.ApplyMove(m)
	if err != nil {
		t.Fatalf("ApplyMove(e2e4): %v", err)
	}
	const want = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"
	if got := next.FEN(); got != want {
		t.Errorf("FEN after ApplyMove = %q, want %q", got, want)
	}
	if got := b.FEN(); got != NewStartingPosition().FEN() {
		t.Errorf("ApplyMove mutated the original: %q", got)
	}

	if _, err := b.ApplyMove(mustTestMove(t, "e2e5", 0)); err == nil {
		t.Error("ApplyMove accepted an illegal move")
	}
}
