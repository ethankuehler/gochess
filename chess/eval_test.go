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

func TestEvaluateAttackedPieces(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		want int // Evaluate(a) - Evaluate(b)
	}{
		{
			// The e2 knight attacks the white queen on f4; the c3 knight does
			// not. Both knight squares have the same centrality and every other
			// piece is identical, so the difference is the queen attack term.
			name: "white queen attacked",
			a:    "7k/8/8/8/5Q2/8/4n3/K7 w - - 0 1",
			b:    "7k/8/8/8/5Q2/2n5/8/K7 w - - 0 1",
			want: -attackPenalty[Queen],
		},
		{
			// The white knight attacks the black queen in the first position
			// and not in the second, which is good for White.
			name: "black queen attacked",
			a:    "7k/8/8/8/5q2/8/4N3/K7 w - - 0 1",
			b:    "7k/8/8/8/5q2/2N5/8/K7 w - - 0 1",
			want: attackPenalty[Queen],
		},
		{
			// The e8 rook checks the white king on e1 in the first position; on
			// d1 the king is safe. Both king squares have the same piece-square
			// value, so the difference is the king attack term.
			name: "white king attacked",
			a:    "k3r3/8/8/8/8/8/8/4K3 w - - 0 1",
			b:    "k3r3/8/8/8/8/8/8/3K4 w - - 0 1",
			want: -attackPenalty[King],
		},
		{
			// The e1 rook checks the black king on e8 in the first position;
			// the d1 rook does not.
			name: "black king attacked",
			a:    "4k3/8/8/8/8/8/8/K3R3 b - - 0 1",
			b:    "4k3/8/8/8/8/8/8/K2R4 b - - 0 1",
			want: attackPenalty[King],
		},
		{
			// The c4 bishop attacks the white knight on e2; the f5 bishop does
			// not. Both bishop squares have the same centrality.
			name: "white knight attacked",
			a:    "7k/8/8/8/2b5/8/4N3/K7 w - - 0 1",
			b:    "7k/8/8/5b2/8/8/4N3/K7 w - - 0 1",
			want: -attackPenalty[Knight],
		},
		{
			name: "black knight attacked",
			a:    "7k/8/8/8/2B5/8/4n3/K7 w - - 0 1",
			b:    "7k/8/8/5B2/8/8/4n3/K7 w - - 0 1",
			want: attackPenalty[Knight],
		},
		{
			// The f6 knight attacks the white pawn on e4; the c6 knight does
			// not. Both knight squares have the same centrality.
			name: "white pawn attacked",
			a:    "7k/8/5n2/8/4P3/8/8/K7 w - - 0 1",
			b:    "7k/8/2n5/8/4P3/8/8/K7 w - - 0 1",
			want: -attackPenalty[Pawn],
		},
		{
			name: "black pawn attacked",
			a:    "7k/8/5N2/8/4p3/8/8/K7 w - - 0 1",
			b:    "7k/8/2N5/8/4p3/8/8/K7 w - - 0 1",
			want: attackPenalty[Pawn],
		},
		{
			// The c3 pawn defends the d4 knight, but a pawn is a cheaper
			// attacker: e5 pawn takes d4 and the c3 pawn only wins a pawn back.
			// The a5 pawn is on the same rank and does not attack d4.
			name: "defended knight attacked by pawn",
			a:    "7k/8/8/4p3/3N4/2P5/8/K7 w - - 0 1",
			b:    "7k/8/8/p7/3N4/2P5/8/K7 w - - 0 1",
			want: -attackPenalty[Knight],
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

func TestAttackedPieceScoreAllTypes(t *testing.T) {
	// Every white piece is attacked: the a8 rook checks the king on a1, the b8
	// queen attacks the b2 pawn, the e1 bishop attacks the c3 knight, the d8
	// rook attacks the d3 bishop, the e7 rook attacks the e3 rook, and the f8
	// queen attacks the f3 queen.
	position := mustFEN(t, "rq1r1q1k/4r3/8/8/8/2NBRQ2/1P6/K3b3 w - - 0 1")

	// Only the king counts for White: the king cannot be defended out of check,
	// while every other attacked white piece is defended by a comrade and is
	// not attacked by a cheaper black piece.
	if got, want := position.attackedPieceScore(position.attackInfo(Black), position.attackInfo(White).all, White), attackPenalty[King]; got != want {
		t.Errorf("white attacked-piece score = %d, want %d", got, want)
	}

	// Of White's attacks, only the undefended bishop on e1 is hanging; the e7
	// rook, f8 queen, and a8 rook are defended and are attacked by equal or more
	// valuable pieces.
	if got, want := position.attackedPieceScore(position.attackInfo(White), position.attackInfo(Black).all, Black), attackPenalty[Bishop]; got != want {
		t.Errorf("black attacked-piece score = %d, want %d", got, want)
	}
}

func TestAttackedPieceScoreCheaperAttacker(t *testing.T) {
	// Every target is defended by the c3 pawn. A piece is only counted when it
	// is undefended or attacked by a piece from a cheaper material class.
	cases := []struct {
		name string
		fen  string
		want int
	}{
		{
			name: "pawn attacks defended knight",
			fen:  "7k/8/8/4p3/3N4/2P5/8/K7 w - - 0 1",
			want: attackPenalty[Knight],
		},
		{
			name: "bishop attacks defended knight",
			fen:  "7k/8/5b2/8/3N4/2P5/8/K7 w - - 0 1",
			want: 0,
		},
		{
			name: "rook attacks defended queen",
			fen:  "3r3k/8/8/8/3Q4/2P5/8/K7 w - - 0 1",
			want: attackPenalty[Queen],
		},
		{
			name: "pawn attacks defended pawn",
			fen:  "7k/8/8/4p3/3P4/2P5/8/K7 w - - 0 1",
			want: 0,
		},
		{
			name: "bishop attacks defended rook",
			fen:  "7k/8/5b2/8/3R4/2P5/8/K7 w - - 0 1",
			want: attackPenalty[Rook],
		},
		{
			name: "queen attacks defended queen",
			fen:  "3q3k/8/8/8/3Q4/2P5/8/K7 w - - 0 1",
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			position := mustFEN(t, tc.fen)
			got := position.attackedPieceScore(position.attackInfo(Black), position.attackInfo(White).all, White)
			if got != tc.want {
				t.Errorf("attacked-piece score = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestAttackInfoMatchesIsSquareAttacked(t *testing.T) {
	fens := []string{
		NewStartingPosition().FEN(),
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"rq1r1q1k/4r3/8/8/8/2NBRQ2/1P6/K3b3 w - - 0 1",
	}

	for _, fen := range fens {
		position := mustFEN(t, fen)
		for colour := White; colour <= Black; colour++ {
			info := position.attackInfo(colour)
			for square := Square(0); square < squareCount; square++ {
				want := position.isSquareAttacked(square, colour)
				got := info.all&(BitBoard(1)<<square) != 0
				if got != want {
					t.Errorf("%q: %v attack map on %s = %v, isSquareAttacked = %v",
						fen, colour, SquareName(square), got, want)
				}
			}
		}
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
		// Attack terms must flip with the board: a white piece under attack
		// mirrors to a black piece under attack, for every piece type.
		"7k/8/8/8/5Q2/8/4n3/K7 w - - 0 1",
		"k3r3/8/8/8/8/8/8/4K3 w - - 0 1",
		"4k3/8/8/8/8/8/8/K3R3 b - - 0 1",
		"rq1r1q1k/4r3/8/8/8/2NBRQ2/1P6/K3b3 w - - 0 1",
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
