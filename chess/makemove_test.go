package chess

import "testing"

// mustFEN parses a FEN or fails the test.
func mustFEN(t *testing.T, fen string) *Position {
	t.Helper()
	b, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN(%q): %v", fen, err)
	}
	return b
}

// mustTestMove builds a move from UCI notation plus any flags the notation
// cannot express.
func mustTestMove(t *testing.T, uci string, extraFlags uint16) Move {
	t.Helper()
	m, err := ParseUCIMove(uci)
	if err != nil {
		t.Fatalf("ParseUCIMove(%q): %v", uci, err)
	}
	return Move{from: m.from, to: m.to, flags: m.flags | extraFlags}
}

// mustSquare parses an algebraic square or fails the test.
func mustSquare(t *testing.T, name string) Square {
	t.Helper()
	square, err := ParseSquare(name)
	if err != nil {
		t.Fatalf("ParseSquare(%q): %v", name, err)
	}
	return square
}

func TestMoveFlags(t *testing.T) {
	base := mustTestMove(t, "e2e4", 0)

	cases := []struct {
		name  string
		move  Move
		check func(Move) bool
		want  bool
	}{
		{"plain", base, Move.IsCapture, false},
		{"capture", Move{from: mustSquare(t, "e4"), to: mustSquare(t, "d5"), flags: captureFlag}, Move.IsCapture, true},
		{"en passant", Move{from: mustSquare(t, "e5"), to: mustSquare(t, "d6"), flags: captureFlag | enPassantFlag}, Move.IsEnPassant, true},
		{"castling", Move{from: e1, to: g1, flags: castlingFlag}, Move.IsCastling, true},
		{"double push", mustTestMove(t, "e2e4", doublePushFlag), Move.IsDoublePush, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.check(tc.move); got != tc.want {
				t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestMovePromotionFlags(t *testing.T) {
	m := mustTestMove(t, "a7a8q", 0)
	promotion, ok := m.Promotion()
	if !ok || promotion != Queen {
		t.Fatalf("Promotion() = %v, %v; want Queen, true", promotion, ok)
	}
	if m.IsCapture() {
		t.Error("plain promotion reported as a capture")
	}

	m = mustTestMove(t, "a7b8n", captureFlag)
	promotion, ok = m.Promotion()
	if !ok || promotion != Knight {
		t.Fatalf("Promotion() = %v, %v; want Knight, true", promotion, ok)
	}
	if !m.IsCapture() {
		t.Error("promotion capture was not reported as a capture")
	}
	if got := m.String(); got != "a7b8n" {
		t.Errorf("String() = %q, want a7b8n", got)
	}
}

func TestOpposite(t *testing.T) {
	if got := White.Opposite(); got != Black {
		t.Errorf("White.Opposite() = %v, want Black", got)
	}
	if got := Black.Opposite(); got != White {
		t.Errorf("Black.Opposite() = %v, want White", got)
	}
	if got := Both.Opposite(); got != Both {
		t.Errorf("Both.Opposite() = %v, want Both", got)
	}
}

func TestPieceAtAndKingSquare(t *testing.T) {
	b := NewStartingPosition()

	cases := []struct {
		square Square
		colour Colour
		piece  Piece
		ok     bool
	}{
		{a1, White, Rook, true},
		{e1, White, King, true},
		{a8, Black, Rook, true},
		{e8, Black, King, true},
		{mustSquare(t, "d4"), Both, NoPiece, false},
	}
	for _, tc := range cases {
		colour, piece, ok := b.pieceAt(tc.square)
		if ok != tc.ok || colour != tc.colour || piece != tc.piece {
			t.Errorf("pieceAt(%s) = (%v, %v, %v), want (%v, %v, %v)",
				SquareName(tc.square), colour, piece, ok, tc.colour, tc.piece, tc.ok)
		}
	}

	if got := b.kingSquare(White); got != e1 {
		t.Errorf("white kingSquare = %s, want e1", SquareName(got))
	}
	if got := b.kingSquare(Black); got != e8 {
		t.Errorf("black kingSquare = %s, want e8", SquareName(got))
	}
}

func TestCastlingRightsToClear(t *testing.T) {
	cases := []struct {
		square Square
		want   uint8
	}{
		{e1, whiteKingsideCastling | whiteQueensideCastling},
		{a1, whiteQueensideCastling},
		{h1, whiteKingsideCastling},
		{e8, blackKingsideCastling | blackQueensideCastling},
		{a8, blackQueensideCastling},
		{h8, blackKingsideCastling},
		{mustSquare(t, "d4"), 0},
	}
	for _, tc := range cases {
		if got := castlingRightsToClear(tc.square); got != tc.want {
			t.Errorf("castlingRightsToClear(%s) = %d, want %d", SquareName(tc.square), got, tc.want)
		}
	}
}

func TestMakeMove(t *testing.T) {
	cases := []struct {
		name       string
		fen        string
		uci        string
		extraFlags uint16
		wantFEN    string
	}{
		{
			name:       "double pawn push",
			fen:        "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			uci:        "e2e4",
			extraFlags: doublePushFlag,
			wantFEN:    "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		},
		{
			name:    "knight move increments halfmove clock",
			fen:     "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			uci:     "g1f3",
			wantFEN: "rnbqkbnr/pppppppp/8/8/8/5N2/PPPPPPPP/RNBQKB1R b KQkq - 1 1",
		},
		{
			name:       "kingside castling",
			fen:        "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
			uci:        "e1g1",
			extraFlags: castlingFlag,
			wantFEN:    "r3k2r/8/8/8/8/8/8/R4RK1 b kq - 1 1",
		},
		{
			name:       "queenside castling",
			fen:        "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
			uci:        "e1c1",
			extraFlags: castlingFlag,
			wantFEN:    "r3k2r/8/8/8/8/8/8/2KR3R b kq - 1 1",
		},
		{
			name:       "black kingside castling",
			fen:        "r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1",
			uci:        "e8g8",
			extraFlags: castlingFlag,
			wantFEN:    "r4rk1/8/8/8/8/8/8/R3K2R w KQ - 1 2",
		},
		{
			name:       "en passant",
			fen:        "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
			uci:        "e5d6",
			extraFlags: captureFlag | enPassantFlag,
			wantFEN:    "4k3/8/3P4/8/8/8/8/4K3 b - - 0 1",
		},
		{
			name:    "promotion",
			fen:     "8/P7/8/8/8/8/8/K6k w - - 0 1",
			uci:     "a7a8q",
			wantFEN: "Q7/8/8/8/8/8/8/K6k b - - 0 1",
		},
		{
			name:    "promotion capture",
			fen:     "1n6/P7/8/8/8/8/8/K6k w - - 0 1",
			uci:     "a7b8n",
			wantFEN: "1N6/8/8/8/8/8/8/K6k b - - 0 1",
		},
		{
			name:       "rook capture clears castling rights",
			fen:        "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
			uci:        "a1a8",
			extraFlags: captureFlag,
			wantFEN:    "R3k2r/8/8/8/8/8/8/4K2R b Kk - 0 1",
		},
		{
			name:    "king move clears both rights",
			fen:     "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
			uci:     "e1e2",
			wantFEN: "r3k2r/8/8/8/8/8/4K3/R6R b kq - 1 1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := mustFEN(t, tc.fen)
			m := mustTestMove(t, tc.uci, tc.extraFlags)
			next := b.makeMove(m)
			if got := next.FEN(); got != tc.wantFEN {
				t.Errorf("FEN after %s = %q, want %q", tc.uci, got, tc.wantFEN)
			}
			if got := b.FEN(); got != tc.fen {
				t.Errorf("makeMove mutated the original: got %q, want %q", got, tc.fen)
			}
		})
	}
}

func TestMakeMoveSequence(t *testing.T) {
	b := NewStartingPosition()
	sequence := []struct {
		uci   string
		flags uint16
	}{
		{"e2e4", doublePushFlag},
		{"e7e5", doublePushFlag},
		{"g1f3", 0},
		{"b8c6", 0},
		{"f1b5", 0},
		{"a7a6", 0},
		{"b5a4", 0},
		{"g8f6", 0},
		{"e1g1", castlingFlag},
	}

	for _, step := range sequence {
		m := mustTestMove(t, step.uci, step.flags)
		b = ptr(b.makeMove(m))
	}

	const want = "r1bqkb1r/1ppp1ppp/p1n2n2/4p3/B3P3/5N2/PPPP1PPP/RNBQ1RK1 b kq - 3 5"
	if got := b.FEN(); got != want {
		t.Errorf("FEN after sequence = %q, want %q", got, want)
	}
}

func ptr(p Position) *Position { return &p }

func TestPublicPositionHelpers(t *testing.T) {
	b := NewStartingPosition()

	if got := b.SideToMove(); got != White {
		t.Errorf("SideToMove() = %v, want White", got)
	}
	colour, piece, ok := b.PieceAt(a1)
	if !ok || colour != White || piece != Rook {
		t.Errorf("PieceAt(a1) = (%v, %v, %v), want (White, Rook, true)", colour, piece, ok)
	}
	if _, _, ok := b.PieceAt(mustSquare(t, "d4")); ok {
		t.Error("PieceAt(d4) reported an occupied square")
	}

	next := b.makeMove(mustTestMove(t, "e2e4", doublePushFlag))
	if got := next.SideToMove(); got != Black {
		t.Errorf("SideToMove() after e2e4 = %v, want Black", got)
	}
}
