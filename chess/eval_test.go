package chess

import (
	"strings"
	"testing"
)

func TestEvaluateStartingPosition(t *testing.T) {
	if got := NewStartingPosition().Evaluate(); got != 0 {
		t.Fatalf("starting position evaluation = %d, want 0", got)
	}
}

func TestEvaluateTerms(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		want int // Evaluate(a) - Evaluate(b)
	}{
		{
			name: "knight centrality",
			a:    "8/8/8/8/3N4/8/8/K6k w - - 0 1",
			b:    "8/8/8/8/8/8/N7/K6k w - - 0 1",
			want: 25,
		},
		{
			name: "king corner",
			a:    "7k/8/8/8/8/8/8/K7 w - - 0 1",
			b:    "7k/8/8/8/4K3/8/8/8 w - - 0 1",
			want: 55,
		},
		{
			name: "pawn near promotion",
			a:    "7k/4P3/8/8/8/8/8/K7 w - - 0 1",
			b:    "7k/8/8/8/8/8/4P3/K7 w - - 0 1",
			want: 80,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diff := mustFEN(t, tc.a).Evaluate() - mustFEN(t, tc.b).Evaluate()
			if diff != tc.want {
				t.Errorf("evaluation difference = %d, want %d", diff, tc.want)
			}
		})
	}
}

func TestEvaluateMaterial(t *testing.T) {
	start := NewStartingPosition()

	cases := []struct {
		name string
		fen  string
		want int // difference from the starting position
	}{
		{
			name: "black queen removed",
			fen:  "rnb1kbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			want: pieceSquareScore(Queen, d1),
		},
		{
			name: "white knight removed",
			fen:  "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/R1BQKBNR w KQkq - 0 1",
			want: -pieceSquareScore(Knight, b1),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diff := mustFEN(t, tc.fen).Evaluate() - start.Evaluate()
			if diff != tc.want {
				t.Errorf("evaluation difference = %d, want %d", diff, tc.want)
			}
		})
	}
}

func TestEvaluateColourSymmetry(t *testing.T) {
	fens := []string{
		NewStartingPosition().FEN(),
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"rnbqkbnr/ppp1pppp/8/3pP3/8/8/PPPP1PPP/RNBQKBNR w KQkq d6 0 3",
		"8/8/8/4k3/8/8/3Q4/4K3 w - - 0 1",
	}

	for _, fen := range fens {
		t.Run(fen, func(t *testing.T) {
			original := mustFEN(t, fen)
			mirroredFEN := mirrorFEN(t, fen)
			mirrored := mustFEN(t, mirroredFEN)

			if got, want := mirrored.Evaluate(), -original.Evaluate(); got != want {
				t.Errorf("mirrored evaluation = %d, want %d (mirrored FEN %q)", got, want, mirroredFEN)
			}
		})
	}
}

func TestPieceSquareValuesMirror(t *testing.T) {
	for piece := Pawn; piece <= King; piece++ {
		for square := Square(0); square < squareCount; square++ {
			want := -pieceSquareValues[piece][square^56]
			got := pieceSquareValues[piece+blackPieceOffset][square]
			if got != want {
				t.Fatalf("black %v on %s = %d, want %d", piece, SquareName(square), got, want)
			}
		}
	}
}

// mirrorFEN returns the position with colours swapped and the board flipped
// vertically. It is used to test that evaluation is symmetric between colours.
func mirrorFEN(t *testing.T, fen string) string {
	t.Helper()

	fields := strings.Fields(fen)
	if len(fields) != 6 {
		t.Fatalf("mirrorFEN(%q): expected 6 fields, got %d", fen, len(fields))
	}

	ranks := strings.Split(fields[0], "/")
	if len(ranks) != boardSize {
		t.Fatalf("mirrorFEN(%q): expected 8 ranks, got %d", fen, len(ranks))
	}
	for i, j := 0, len(ranks)-1; i < j; i, j = i+1, j-1 {
		ranks[i], ranks[j] = ranks[j], ranks[i]
	}
	for i, rank := range ranks {
		var builder strings.Builder
		for _, v := range rank {
			builder.WriteByte(swapCase(byte(v)))
		}
		ranks[i] = builder.String()
	}

	side := "b"
	if fields[1] == "b" {
		side = "w"
	}

	var castling strings.Builder
	for _, symbol := range castlingSymbols {
		if strings.ContainsRune(fields[2], rune(swapCase(symbol[0]))) {
			castling.WriteString(symbol)
		}
	}
	castlingField := castling.String()
	if castlingField == "" {
		castlingField = "-"
	}

	enPassant := "-"
	if fields[3] != "-" {
		square, err := ParseSquare(fields[3])
		if err != nil {
			t.Fatalf("mirrorFEN(%q): en passant square: %v", fen, err)
		}
		fr := FileRankFromSquare(square)
		mirrored := SquareFromFileRank(FileRank{file: fr.File(), rank: 7 - fr.Rank()})
		enPassant = SquareName(mirrored)
	}

	return strings.Join([]string{
		strings.Join(ranks, "/"),
		side,
		castlingField,
		enPassant,
		fields[4],
		fields[5],
	}, " ")
}

// swapCase flips the case of an ASCII letter and leaves anything else alone.
func swapCase(v byte) byte {
	switch {
	case v >= 'a' && v <= 'z':
		return v - 'a' + 'A'
	case v >= 'A' && v <= 'Z':
		return v - 'A' + 'a'
	default:
		return v
	}
}
