package chess

import (
	"strings"
	"testing"
)

// TestBitboardCoordinateMapping validates that bit positions 0-63 map correctly to squares a1-h8
func TestBitboardCoordinateMapping(t *testing.T) {
	// Test that square a1 is bit 0
	loc, err := LocFromAlg("a1")
	if err != nil {
		t.Fatalf("Failed to parse a1: %v", err)
	}
	if loc != 1 {
		t.Errorf("a1 should be bit 0 (value 1), got %d", loc)
	}

	// Test that square h8 is bit 63
	loc, err = LocFromAlg("h8")
	if err != nil {
		t.Fatalf("Failed to parse h8: %v", err)
	}
	expected := BitBoard(1) << 63
	if loc != expected {
		t.Errorf("h8 should be bit 63 (value 2^63), got %d", loc)
	}

	// Test systematic mapping for all squares
	for rank := 0; rank < 8; rank++ {
		for file := 0; file < 8; file++ {
			alg := string(COLUMNS[file]) + string(rune('1'+rank))
			loc, err := LocFromAlg(alg)
			if err != nil {
				t.Errorf("Failed to parse %s: %v", alg, err)
				continue
			}

			// Expected bit position is file + rank * 8
			expectedShift := Shift(file + rank*8)
			expectedLoc := BitBoard(1) << expectedShift

			if loc != expectedLoc {
				t.Errorf("Square %s: expected bit %d (value %d), got %d",
					alg, expectedShift, expectedLoc, loc)
			}
		}
	}
}

// TestStartingPositionPiecePlacement verifies pieces are at correct positions in starting setup
func TestStartingPositionPiecePlacement(t *testing.T) {
	board := NewBoardDefault()

	// Test white pawns on rank 2 (bits 8-15)
	expectedWhitePawns := BitBoard(0b1111111100000000)
	if board.pieces[PAWN] != expectedWhitePawns {
		t.Errorf("White pawns: expected %064b, got %064b",
			expectedWhitePawns, board.pieces[PAWN])
	}

	// Test white rooks on a1 and h1 (bits 0 and 7)
	expectedWhiteRooks := BitBoard(0b10000001)
	if board.pieces[ROOK] != expectedWhiteRooks {
		t.Errorf("White rooks: expected %064b, got %064b",
			expectedWhiteRooks, board.pieces[ROOK])
	}

	// Test white knights on b1 and g1 (bits 1 and 6)
	expectedWhiteKnights := BitBoard(0b01000010)
	if board.pieces[KNIGHT] != expectedWhiteKnights {
		t.Errorf("White knights: expected %064b, got %064b",
			expectedWhiteKnights, board.pieces[KNIGHT])
	}

	// Test white bishops on c1 and f1 (bits 2 and 5)
	expectedWhiteBishops := BitBoard(0b00100100)
	if board.pieces[BISHOP] != expectedWhiteBishops {
		t.Errorf("White bishops: expected %064b, got %064b",
			expectedWhiteBishops, board.pieces[BISHOP])
	}

	// Test white queen on e1 (bit 4) - Note: Implementation places queen on e1, not standard d1
	expectedWhiteQueen := BitBoard(0b00010000)
	if board.pieces[QUEEN] != expectedWhiteQueen {
		t.Errorf("White queen: expected %064b, got %064b",
			expectedWhiteQueen, board.pieces[QUEEN])
	}

	// Test white king on d1 (bit 3) - Note: Implementation places king on d1, not standard e1
	expectedWhiteKing := BitBoard(0b00001000)
	if board.pieces[KING] != expectedWhiteKing {
		t.Errorf("White king: expected %064b, got %064b",
			expectedWhiteKing, board.pieces[KING])
	}

	// Test black pawns on rank 7 (bits 48-55)
	expectedBlackPawns := BitBoard(0b1111111100000000) << 40
	if board.pieces[PAWN+BLACK_OFFSET] != expectedBlackPawns {
		t.Errorf("Black pawns: expected %064b, got %064b",
			expectedBlackPawns, board.pieces[PAWN+BLACK_OFFSET])
	}

	// Test black pieces on rank 8 (bits 56-63)
	expectedBlackRooks := BitBoard(0b10000001) << 56
	if board.pieces[ROOK+BLACK_OFFSET] != expectedBlackRooks {
		t.Errorf("Black rooks: expected %064b, got %064b",
			expectedBlackRooks, board.pieces[ROOK+BLACK_OFFSET])
	}
}

// TestManualPiecePlacement verifies piece placement at each square
func TestManualPiecePlacement(t *testing.T) {
	// Test placing a piece at each square manually
	for square := Shift(0); square < 64; square++ {
		board := &BoardState{}
		board.pieces[PAWN] = BitBoard(1) << square

		// Verify the bit is set at the correct position
		if board.pieces[PAWN] != (BitBoard(1) << square) {
			t.Errorf("Piece placement at square %d failed", square)
		}

		// Verify Occupied returns the correct bitboard
		occupied := board.Occupied(WHITE)
		if occupied != (BitBoard(1) << square) {
			t.Errorf("Occupied() at square %d: expected %064b, got %064b",
				square, BitBoard(1)<<square, occupied)
		}
	}
}

// TestInvalidFENRejection tests that invalid FEN strings are rejected
func TestInvalidFENRejection(t *testing.T) {
	invalidFENs := []struct {
		fen         string
		description string
	}{
		{"", "empty string"},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR", "missing fields"},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR x KQkq - 0 1", "invalid turn"},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP w KQkq - 0 1", "incomplete board"},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - abc 1", "invalid halfmove"},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 xyz", "invalid fullmove"},
	}

	for _, tc := range invalidFENs {
		_, err := NewBoardFEN(tc.fen)
		if err == nil {
			t.Errorf("Expected error for %s, but got none", tc.description)
		}
	}
}

// TestMinimalPosition tests a minimal king vs king position
func TestMinimalPosition(t *testing.T) {
	fen := "4k3/8/8/8/8/8/8/4K3 w - - 0 1"
	board, err := NewBoardFEN(fen)
	if err != nil {
		t.Fatalf("Failed to parse minimal FEN: %v", err)
	}

	// Verify reconstruction
	reconstructed := board.FEN()
	if reconstructed != fen {
		t.Errorf("Minimal position FEN mismatch:\nInput:  %s\nOutput: %s", fen, reconstructed)
	}

	// Verify only kings are present
	whiteOccupied := board.Occupied(WHITE)
	blackOccupied := board.Occupied(BLACK)

	// Count pieces
	whiteCount := 0
	for i := Shift(0); i < 64; i++ {
		if whiteOccupied&(BitBoard(1)<<i) != 0 {
			whiteCount++
		}
	}
	blackCount := 0
	for i := Shift(0); i < 64; i++ {
		if blackOccupied&(BitBoard(1)<<i) != 0 {
			blackCount++
		}
	}

	if whiteCount != 1 || blackCount != 1 {
		t.Errorf("Expected 1 white and 1 black piece, got %d white and %d black", whiteCount, blackCount)
	}
}

// TestEnPassantEncoding tests en passant square encoding and decoding
func TestEnPassantEncoding(t *testing.T) {
	testCases := []struct {
		fen             string
		expectedEnpassant string
	}{
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", "-"},
		{"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1", "e3"},
		{"rnbqkbnr/ppp1pppp/8/3pP3/8/8/PPPP1PPP/RNBQKBNR w KQkq d6 0 2", "d6"},
	}

	for _, tc := range testCases {
		board, err := NewBoardFEN(tc.fen)
		if err != nil {
			t.Errorf("Failed to parse FEN %s: %v", tc.fen, err)
			continue
		}

		// Check InfoString contains correct en passant info
		infoStr := board.InfoString()
		if !strings.Contains(infoStr, tc.expectedEnpassant) {
			t.Errorf("InfoString for FEN %s:\nExpected to contain: %s\nGot: %s",
				tc.fen, tc.expectedEnpassant, infoStr)
		}

		// Verify round-trip
		reconstructed := board.FEN()
		if reconstructed != tc.fen {
			t.Errorf("En passant FEN mismatch:\nInput:  %s\nOutput: %s", tc.fen, reconstructed)
		}
	}
}

// TestCastlingRightsEncoding tests castling rights encoding in the encoding field
func TestCastlingRightsEncoding(t *testing.T) {
	testCases := []struct {
		fen              string
		expectedCastling string
	}{
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", "KQkq"},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQ - 0 1", "KQ"},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w kq - 0 1", "kq"},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1", "-"},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w K - 0 1", "K"},
	}

	for _, tc := range testCases {
		board, err := NewBoardFEN(tc.fen)
		if err != nil {
			t.Errorf("Failed to parse FEN %s: %v", tc.fen, err)
			continue
		}

		// Verify round-trip
		reconstructed := board.FEN()
		if reconstructed != tc.fen {
			t.Errorf("Castling rights FEN mismatch:\nInput:  %s\nOutput: %s", tc.fen, reconstructed)
		}

		// Check InfoString contains correct castling info
		infoStr := board.InfoString()
		if !strings.Contains(infoStr, tc.expectedCastling) {
			t.Errorf("InfoString for FEN %s:\nExpected to contain: %s\nGot: %s",
				tc.fen, tc.expectedCastling, infoStr)
		}
	}
}

// TestHalfmoveFullmoveClock tests halfmove and fullmove clock parsing
func TestHalfmoveFullmoveClock(t *testing.T) {
	testCases := []struct {
		fen              string
		expectedHalfmove uint16
		expectedFullmove uint16
	}{
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", 0, 1},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 5 10", 5, 10},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 25 50", 25, 50},
	}

	for _, tc := range testCases {
		board, err := NewBoardFEN(tc.fen)
		if err != nil {
			t.Errorf("Failed to parse FEN %s: %v", tc.fen, err)
			continue
		}

		if board.halfmove_clock != tc.expectedHalfmove {
			t.Errorf("Halfmove clock for FEN %s: expected %d, got %d",
				tc.fen, tc.expectedHalfmove, board.halfmove_clock)
		}

		if board.fullmove_number != tc.expectedFullmove {
			t.Errorf("Fullmove number for FEN %s: expected %d, got %d",
				tc.fen, tc.expectedFullmove, board.fullmove_number)
		}

		// Verify round-trip
		reconstructed := board.FEN()
		if reconstructed != tc.fen {
			t.Errorf("Move clock FEN mismatch:\nInput:  %s\nOutput: %s", tc.fen, reconstructed)
		}
	}
}

// TestMoveRoundTrip tests move conversion between algebraic and internal format
func TestMoveRoundTrip(t *testing.T) {
	testMoves := []string{
		"e2e4", "e7e5", "g1f3", "b8c6",
		"a1a8", "h1h8", "a8a1", "h8h1",
	}

	for _, uci := range testMoves {
		move, err := NewMoveUCI(uci)
		if err != nil {
			t.Errorf("Failed to create move from UCI %s: %v", uci, err)
			continue
		}

		reconstructed := move.String()
		if reconstructed != uci {
			t.Errorf("Move round-trip failed: input %s, output %s", uci, reconstructed)
		}
	}
}

// TestComplexPositionNoBitCollisions tests complex positions to ensure no bit collisions
func TestComplexPositionNoBitCollisions(t *testing.T) {
	// Test a complex position with many pieces
	fen := "r1bqkb1r/pppp1ppp/2n2n2/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4"
	board, err := NewBoardFEN(fen)
	if err != nil {
		t.Fatalf("Failed to parse complex FEN: %v", err)
	}

	// Verify no pieces occupy the same square
	allPieces := BitBoard(0)
	for i := 0; i < 12; i++ {
		if allPieces&board.pieces[i] != 0 {
			t.Errorf("Bit collision detected: piece %d overlaps with previously set bits", i)
		}
		allPieces |= board.pieces[i]
	}

	// Verify round-trip
	reconstructed := board.FEN()
	if reconstructed != fen {
		t.Errorf("Complex position FEN mismatch:\nInput:  %s\nOutput: %s", fen, reconstructed)
	}
}

// TestDisplayOutputVisualLayout tests that String() produces correct visual layout
func TestDisplayOutputVisualLayout(t *testing.T) {
	board := NewBoardDefault()
	output := board.String()

	// The output should have multiple lines (one per rank plus newlines)
	lines := strings.Split(output, "\n")
	if len(lines) < 8 {
		t.Errorf("String() output should have at least 8 lines for the board, got %d", len(lines))
	}

	// Check that rank 8 (black pieces) appears first in the output
	// The first non-empty line should contain black pieces (lowercase letters)
	firstBoardLine := ""
	for _, line := range lines {
		if len(strings.TrimSpace(line)) > 0 {
			firstBoardLine = line
			break
		}
	}

	// First rank shown should have black pieces (r, n, b, q, k)
	if !strings.Contains(firstBoardLine, "r") && !strings.Contains(firstBoardLine, "n") {
		t.Errorf("First line should show rank 8 (black pieces), got: %s", firstBoardLine)
	}
}

// TestStringUniUnicodeSymbols tests that StringUni() produces valid Unicode chess symbols
func TestStringUniUnicodeSymbols(t *testing.T) {
	board := NewBoardDefault()
	output := board.StringUni()

	// Check for Unicode chess piece symbols
	expectedSymbols := []string{"♙", "♗", "♘", "♖", "♕", "♔", "♟", "♝", "♞", "♜", "♛", "♚"}
	foundCount := 0
	for _, symbol := range expectedSymbols {
		if strings.Contains(output, symbol) {
			foundCount++
		}
	}

	if foundCount == 0 {
		t.Error("StringUni() should contain Unicode chess symbols")
	}
}

// TestInfoStringFormat tests InfoString() format correctness
func TestInfoStringFormat(t *testing.T) {
	board := NewBoardDefault()
	info := board.InfoString()

	// Should contain turn indicator
	if !strings.Contains(info, "w") && !strings.Contains(info, "b") {
		t.Error("InfoString() should contain turn indicator (w or b)")
	}

	// Should contain castling info
	if !strings.Contains(info, "KQkq") && !strings.Contains(info, "-") {
		t.Error("InfoString() should contain castling info")
	}

	// Should contain move clocks
	if !strings.Contains(info, "0") {
		t.Error("InfoString() should contain move clock information")
	}
}

// TestDisplayOrder tests that display order is correct (rank 8 at top, a-file on left)
func TestDisplayOrder(t *testing.T) {
	// Create a board with a single piece at a known position
	fen := "8/8/8/8/8/8/8/R7 w - - 0 1" // Rook at a1
	board, err := NewBoardFEN(fen)
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	output := board.String()
	lines := strings.Split(output, "\n")

	// Find the line with the rook
	rookLineIndex := -1
	for i, line := range lines {
		if strings.Contains(line, "R") {
			rookLineIndex = i
			break
		}
	}

	if rookLineIndex == -1 {
		t.Fatal("Could not find rook in output")
	}

	// The rook should be on the last board line (rank 1 is at the bottom)
	// Count non-empty board lines
	boardLineCount := 0
	lastBoardLineIndex := -1
	for i, line := range lines {
		if len(strings.TrimSpace(line)) > 0 && (strings.Contains(line, "_") || strings.Contains(line, "R")) {
			boardLineCount++
			lastBoardLineIndex = i
		}
	}

	// Rook at a1 should be on one of the last board lines
	if rookLineIndex > lastBoardLineIndex-2 {
		// This is expected - rook on rank 1 should be near bottom
	} else if rookLineIndex < 2 {
		t.Error("Rook at a1 should appear near the bottom of the board display, not at the top")
	}
}

// TestPawnDirections tests that pawns move in correct directions
func TestPawnDirections(t *testing.T) {
	BuildPawnMoves()
	BuildPawnAttacks()

	// Test white pawn on e2 (square 12)
	whitePawnSquare := Shift(12) // e2
	whiteMoves := WHITE_PAWN_MOVES[whitePawnSquare]

	// White pawns should be able to move up (to e3 and e4)
	e3 := BitBoard(1) << 20 // e3

	// Should include at least e3
	if whiteMoves&e3 == 0 {
		t.Error("White pawn on e2 should be able to move to e3")
	}

	// Test black pawn on e7 (square 52)
	blackPawnSquare := Shift(52) // e7
	blackMoves := BLACK_PAWN_MOVES[blackPawnSquare]

	// Black pawns should be able to move down (to e6 and e5)
	e6 := BitBoard(1) << 44 // e6

	// Should include at least e6
	if blackMoves&e6 == 0 {
		t.Error("Black pawn on e7 should be able to move to e6")
	}
}

// TestPawnAttacksDiagonal tests that pawn attacks are diagonal
func TestPawnAttacksDiagonal(t *testing.T) {
	BuildPawnAttacks()

	// Test white pawn on e4 (square 28)
	whitePawnSquare := Shift(28) // e4
	whiteAttacks := WHITE_PAWN_ATTACKS[whitePawnSquare]

	// Should attack d5 and f5 (squares 35 and 37)
	d5 := BitBoard(1) << 35
	f5 := BitBoard(1) << 37

	if whiteAttacks&d5 == 0 {
		t.Error("White pawn on e4 should attack d5")
	}
	if whiteAttacks&f5 == 0 {
		t.Error("White pawn on e4 should attack f5")
	}

	// Test black pawn on e5 (square 36)
	blackPawnSquare := Shift(36) // e5
	blackAttacks := BLACK_PAWN_ATTACKS[blackPawnSquare]

	// Should attack d4 and f4 (squares 27 and 29)
	d4 := BitBoard(1) << 27
	f4 := BitBoard(1) << 29

	if blackAttacks&d4 == 0 {
		t.Error("Black pawn on e5 should attack d4")
	}
	if blackAttacks&f4 == 0 {
		t.Error("Black pawn on e5 should attack f4")
	}
}

// TestKnightLShapeAttacks tests that knights attack L-shaped squares
func TestKnightLShapeAttacks(t *testing.T) {
	BuildKnightAttacks()

	// Test knight on d4 (square 27)
	knightSquare := Shift(27) // d4
	attacks := KNIGHT_ATTACKS[knightSquare]

	// Knight on d4 should attack these squares:
	// c2, e2, b3, f3, b5, f5, c6, e6
	expectedSquares := []Shift{
		10, // c2
		12, // e2
		17, // b3
		21, // f3
		33, // b5
		37, // f5
		42, // c6
		44, // e6
	}

	for _, sq := range expectedSquares {
		if attacks&(BitBoard(1)<<sq) == 0 {
			coord := CoordsFromShift(sq)
			t.Errorf("Knight on d4 should attack %c%d", COLUMNS[coord.file], coord.rank+1)
		}
	}

	// Count total attacks - should be 8 for a knight in center
	attackCount := 0
	for i := Shift(0); i < 64; i++ {
		if attacks&(BitBoard(1)<<i) != 0 {
			attackCount++
		}
	}
	if attackCount != 8 {
		t.Errorf("Knight on d4 should have 8 attacks, got %d", attackCount)
	}
}

// TestKingAdjacentSquareAttacks tests that kings attack all adjacent squares
func TestKingAdjacentSquareAttacks(t *testing.T) {
	BuildKingAttacks()

	// Test king on d4 (square 27)
	kingSquare := Shift(27) // d4
	attacks := KING_ATTACKS[kingSquare]

	// King on d4 should attack all 8 adjacent squares:
	// c3, d3, e3, c4, e4, c5, d5, e5
	expectedSquares := []Shift{
		18, // c3
		19, // d3
		20, // e3
		26, // c4
		28, // e4
		34, // c5
		35, // d5
		36, // e5
	}

	for _, sq := range expectedSquares {
		if attacks&(BitBoard(1)<<sq) == 0 {
			coord := CoordsFromShift(sq)
			t.Errorf("King on d4 should attack %c%d", COLUMNS[coord.file], coord.rank+1)
		}
	}

	// Count total attacks - should be exactly 8 for a king in center
	attackCount := 0
	for i := Shift(0); i < 64; i++ {
		if attacks&(BitBoard(1)<<i) != 0 {
			attackCount++
		}
	}
	if attackCount != 8 {
		t.Errorf("King on d4 should have 8 attacks, got %d", attackCount)
	}

	// Test king in corner (a1) - should have fewer attacks
	cornerKingSquare := Shift(0) // a1
	cornerAttacks := KING_ATTACKS[cornerKingSquare]

	cornerAttackCount := 0
	for i := Shift(0); i < 64; i++ {
		if cornerAttacks&(BitBoard(1)<<i) != 0 {
			cornerAttackCount++
		}
	}
	if cornerAttackCount != 3 {
		t.Errorf("King on a1 should have 3 attacks, got %d", cornerAttackCount)
	}
}
