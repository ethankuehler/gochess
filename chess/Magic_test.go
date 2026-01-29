package chess

import (
	"testing"
)

func TestPawnAttacks(t *testing.T) {
	BuildPawnAttacks()

	if WHITE_PAWN_ATTACKS == nil {
		t.Error("nil map")
	}

	if len(WHITE_PAWN_ATTACKS) != 64 {
		t.Errorf("map incorrect size, 64 != %d", len(WHITE_PAWN_ATTACKS))
	}
}

func TestPawnMoves(t *testing.T) {
	BuildPawnMoves()

	if WHITE_PAWN_MOVES == nil {
		t.Error("White nil map")
	}

	if len(WHITE_PAWN_MOVES) != 64 {
		t.Errorf("White map incorrect size, 64 != %d", len(WHITE_PAWN_ATTACKS))
	}

	if BLACK_PAWN_MOVES == nil {
		t.Error("Black nil map")
	}

	if len(BLACK_PAWN_MOVES) != 64 {
		t.Errorf("Black map incorrect size, 64 != %d", len(WHITE_PAWN_ATTACKS))
	}

	records, err := readCSV("data/white_pawn_move.csv")
	if err != nil {
		t.Fatalf("could not open csv %v", err)
	}

	for _, record := range records[1:] {
		val, err := readRecord(record)
		if err != nil {
			t.Fatalf("could not convert record %v", err)
		}
		if WHITE_PAWN_MOVES[val[0]] != BitBoard(val[2]) {
			t.Errorf("white move did not match gen=%d actual=%d", WHITE_PAWN_MOVES[val[0]], val[2])
		}
	}

	records, err = readCSV("data/black_pawn_move.csv")
	if err != nil {
		t.Fatalf("could not open csv %v", err)
	}

	for _, record := range records[1:] {
		val, err := readRecord(record)
		if err != nil {
			t.Fatalf("could not convert record %v", err)
		}
		if BLACK_PAWN_MOVES[val[0]] != BitBoard(val[2]) {
			t.Errorf("black move did not match gen=%d actual=%d", BLACK_PAWN_MOVES[val[0]], val[2])
		}
	}
}

// TestRayCastRookCenter tests rook movement from the center of the board with no blockers
func TestRayCastRookCenter(t *testing.T) {
	// Test rook at d4 (square 27) with no blockers
	initial := Shift(27) // d4
	blockers := BitBoard(0)
	// Mask should allow all squares on the same rank and file
	// Note: Need parentheses due to operator precedence: << and * have same precedence
	mask := (COLUMN_MASK << 3) | (ROW_MASK << (3 * 8)) // d-file and 4th rank
	
	result := RayCast(initial, blockers, mask, ROOK_RAY)
	
	// Should be able to move to all squares on d-file and 4th rank except starting square
	// d-file: d1, d2, d3, d5, d6, d7, d8 (7 squares)
	// 4th rank: a4, b4, c4, e4, f4, g4, h4 (7 squares)
	// Total: 14 squares
	expected := mask & ^(BitBoard(1) << initial) // All mask squares except starting position
	
	if result != expected {
		t.Errorf("RayCast rook center failed: got %064b, want %064b", result, expected)
	}
}

// TestRayCastRookWithBlocker tests rook movement with a blocker
func TestRayCastRookWithBlocker(t *testing.T) {
	// Test rook at d4 (square 27) with blocker at d6 (square 43)
	initial := Shift(27)           // d4
	blockers := BitBoard(1) << 43  // d6
	mask := (COLUMN_MASK << 3) | (ROW_MASK << (3 * 8)) // d-file and 4th rank
	
	result := RayCast(initial, blockers, mask, ROOK_RAY)
	
	// Should include d6 (the blocker) but not d7, d8
	// Should include all of 4th rank and d1, d2, d3, d5, d6
	expected := BitBoard(0)
	// Add d-file below: d1, d2, d3 (squares 3, 11, 19)
	expected |= (1 << 3) | (1 << 11) | (1 << 19)
	// Add d-file above up to blocker: d5, d6 (squares 35, 43)
	expected |= (1 << 35) | (1 << 43)
	// Add 4th rank: a4, b4, c4, e4, f4, g4, h4 (squares 24, 25, 26, 28, 29, 30, 31)
	expected |= (1 << 24) | (1 << 25) | (1 << 26) | (1 << 28) | (1 << 29) | (1 << 30) | (1 << 31)
	
	if result != expected {
		t.Errorf("RayCast rook with blocker failed:\ngot  %064b\nwant %064b", result, expected)
	}
}

// TestRayCastBishopCenter tests bishop movement from center with no blockers
func TestRayCastBishopCenter(t *testing.T) {
	// Test bishop at d4 (square 27) with no blockers
	initial := Shift(27)
	blockers := BitBoard(0)
	
	// For bishop, mask should include all diagonal squares
	// Create a simple mask that includes the main diagonals through d4
	var mask BitBoard = 0
	coord := CoordsFromShift(initial)
	// Note: coord.row contains file, coord.col contains rank (swapped!)
	rank, file := coord.col, coord.row
	
	// Add all diagonal squares to mask
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			// Check if on same diagonal (i is rank, j is file)
			if (i-int(rank)) == (j-int(file)) || (i-int(rank)) == -(j-int(file)) {
				mask |= BitBoard(1) << (j + i*8)
			}
		}
	}
	
	result := RayCast(initial, blockers, mask, BISHOP_RAY)
	
	// Should be able to move to all diagonal squares except starting square
	expected := mask & ^(BitBoard(1) << initial)
	
	if result != expected {
		t.Errorf("RayCast bishop center failed:\ngot  %064b\nwant %064b", result, expected)
	}
}

// TestRayCastBishopWithBlocker tests bishop movement with a blocker
func TestRayCastBishopWithBlocker(t *testing.T) {
	// Test bishop at d4 (square 27) with blocker at f6 (square 45)
	initial := Shift(27)           // d4
	blockers := BitBoard(1) << 45  // f6
	
	// Create diagonal mask
	var mask BitBoard = 0
	coord := CoordsFromShift(initial)
	// Note: coord.row contains file, coord.col contains rank (swapped!)
	rank, file := coord.col, coord.row
	
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			// Check if on same diagonal (i is rank, j is file)
			if (i-int(rank)) == (j-int(file)) || (i-int(rank)) == -(j-int(file)) {
				mask |= BitBoard(1) << (j + i*8)
			}
		}
	}
	
	result := RayCast(initial, blockers, mask, BISHOP_RAY)
	
	// Should include f6 (the blocker) but not g7, h8
	// Verify the blocker is included
	if result&blockers == 0 {
		t.Error("RayCast bishop with blocker did not include the blocker square")
	}
	
	// Verify g7 (square 54) is NOT included
	if result&(BitBoard(1)<<54) != 0 {
		t.Error("RayCast bishop with blocker included squares beyond the blocker")
	}
}

// TestRayCastCorner tests rook movement from a corner
func TestRayCastCorner(t *testing.T) {
	// Test rook at a1 (square 0) with no blockers
	initial := Shift(0)
	blockers := BitBoard(0)
	mask := (COLUMN_MASK << 0) | (ROW_MASK << (0 * 8)) // a-file and 1st rank
	
	result := RayCast(initial, blockers, mask, ROOK_RAY)
	
	// Should be able to move to all squares on a-file and 1st rank except a1
	expected := mask & ^(BitBoard(1) << initial)
	
	if result != expected {
		t.Errorf("RayCast corner failed:\ngot  %064b\nwant %064b", result, expected)
	}
}

// TestRayCastBlocked tests a piece blocked on all sides
func TestRayCastBlocked(t *testing.T) {
	// Test rook at d4 (square 27) blocked on all 4 sides
	initial := Shift(27)
	// Blockers at d3, d5, c4, e4 (squares 19, 35, 26, 28)
	blockers := (BitBoard(1) << 19) | (BitBoard(1) << 35) | (BitBoard(1) << 26) | (BitBoard(1) << 28)
	mask := (COLUMN_MASK << 3) | (ROW_MASK << (3 * 8))
	
	result := RayCast(initial, blockers, mask, ROOK_RAY)
	
	// Should only include the 4 blocker squares
	expected := blockers
	
	if result != expected {
		t.Errorf("RayCast blocked failed:\ngot  %064b\nwant %064b", result, expected)
	}
}

// TestRayCastEmptyRay tests with empty ray array
func TestRayCastEmptyRay(t *testing.T) {
	initial := Shift(27)
	blockers := BitBoard(0)
	mask := BitBoard(0xFFFFFFFFFFFFFFFF) // All squares
	emptyRay := Ray{}
	
	result := RayCast(initial, blockers, mask, emptyRay)
	
	// Should return empty bitboard with no directions
	if result != 0 {
		t.Errorf("RayCast with empty ray should return 0, got %064b", result)
	}
}
