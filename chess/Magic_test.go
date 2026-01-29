package chess

import (
	"encoding/csv"
	"os"
	"strings"
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
	rank, file := coord.rank, coord.file
	
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
	rank, file := coord.rank, coord.file
	
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

// parseFENToBlockers converts a FEN board string to a blocker BitBoard
// FEN format: "8/8/2P5/8/8/8/8/8" where numbers are empty squares, letters are pieces
// Both uppercase (white) and lowercase (black) pieces are treated as blockers
// The FEN string must contain exactly 8 ranks separated by '/', with each rank
// representing a row of the board from rank 8 (top) to rank 1 (bottom)
func parseFENToBlockers(fen string) BitBoard {
	var blockers BitBoard = 0
	ranks := strings.Split(fen, "/")
	
	// Validate that we have exactly 8 ranks
	if len(ranks) != 8 {
		return blockers // Return empty if invalid
	}
	
	// FEN starts from rank 8 down to rank 1
	for rankIdx, rankStr := range ranks {
		rank := 7 - rankIdx // Convert to 0-indexed from bottom
		file := 0
		
		for _, ch := range rankStr {
			if ch >= '1' && ch <= '8' {
				// Number means empty squares
				file += int(ch - '0')
			} else {
				// Any letter (piece) is a blocker - both uppercase and lowercase
				if file < 8 {
					square := file + rank*8
					blockers |= BitBoard(1) << square
				}
				file++
			}
		}
		
		// Each rank should have exactly 8 squares
		// If file > 8, the FEN was malformed, but we continue processing
	}
	
	return blockers
}

// parseExpectedSquares converts a comma-separated list of algebraic squares to a BitBoard
func parseExpectedSquares(squaresStr string) (BitBoard, error) {
	var expected BitBoard = 0
	if squaresStr == "" {
		return expected, nil
	}
	
	squares := strings.Split(squaresStr, ",")
	for _, sq := range squares {
		sq = strings.TrimSpace(sq)
		if sq == "" {
			continue
		}
		loc, err := LocFromAlg(sq)
		if err != nil {
			return 0, err
		}
		expected |= loc
	}
	
	return expected, nil
}

// getRayForPieceType returns the appropriate Ray for the piece type
func getRayForPieceType(pieceType string) Ray {
	switch strings.ToLower(pieceType) {
	case "rook":
		return ROOK_RAY
	case "bishop":
		return BISHOP_RAY
	default:
		return Ray{}
	}
}

// getMaskForPieceType returns the appropriate mask for the piece type at the given square
func getMaskForPieceType(pieceType string, square Shift) BitBoard {
	coord := CoordsFromShift(square)
	rank, file := coord.rank, coord.file
	
	switch strings.ToLower(pieceType) {
	case "rook":
		// Rook can move along file and rank
		return (COLUMN_MASK << file) | (ROW_MASK << (rank * 8))
	case "bishop":
		// Bishop can move diagonally
		var mask BitBoard = 0
		for i := 0; i < 8; i++ {
			for j := 0; j < 8; j++ {
				// Check if on same diagonal (i is rank, j is file)
				if (i-int(rank)) == (j-int(file)) || (i-int(rank)) == -(j-int(file)) {
					mask |= BitBoard(1) << (j + i*8)
				}
			}
		}
		return mask
	default:
		return 0
	}
}

// TestRayCastFromConfig tests RayCast using configuration from CSV file
func TestRayCastFromConfig(t *testing.T) {
	file, err := os.Open("data/raycast_tests.csv")
	if err != nil {
		t.Fatalf("Failed to open raycast_tests.csv: %v", err)
	}
	defer file.Close()
	
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}
	
	// Skip header row
	for i, record := range records[1:] {
		// Skip empty records
		if len(record) == 0 || (len(record) == 1 && record[0] == "") {
			continue
		}
		
		if len(record) != 5 {
			t.Logf("Test %d: Skipping invalid record format, expected 5 fields, got %d", i, len(record))
			continue
		}
		
		name := strings.TrimSpace(record[0])
		pieceType := strings.TrimSpace(record[1])
		pieceSquare := strings.TrimSpace(record[2])
		fenBlockers := strings.TrimSpace(record[3])
		expectedSquaresStr := strings.TrimSpace(record[4])
		
		// Validate test name is not empty
		if name == "" {
			t.Logf("Test %d: Skipping test with empty name", i)
			continue
		}
		
		t.Run(name, func(t *testing.T) {
			// Parse piece square
			square, err := ShiftFromAlg(pieceSquare)
			if err != nil {
				t.Fatalf("Invalid piece square %s: %v", pieceSquare, err)
			}
			
			// Parse FEN to blockers
			blockers := parseFENToBlockers(fenBlockers)
			
			// Get mask for piece type
			mask := getMaskForPieceType(pieceType, square)
			
			// Get ray for piece type
			ray := getRayForPieceType(pieceType)
			
			// Run RayCast
			result := RayCast(square, blockers, mask, ray)
			
			// Parse expected squares
			expected, err := parseExpectedSquares(expectedSquaresStr)
			if err != nil {
				t.Fatalf("Failed to parse expected squares: %v", err)
			}
			
			// Compare result with expected
			if result != expected {
				t.Errorf("RayCast failed for %s:\n  Got:      %064b\n  Expected: %064b", name, result, expected)
				
				// Show which squares differ for debugging
				diff := result ^ expected
				if diff != 0 {
					t.Logf("Difference in squares:")
					for sq := Shift(0); sq < 64; sq++ {
						bit := BitBoard(1) << sq
						if diff&bit != 0 {
							coord := CoordsFromShift(sq)
							file := coord.file
							rank := coord.rank
							fileChar := COLUMNS[file]
							rankNum := rank + 1
							inResult := result&bit != 0
							inExpected := expected&bit != 0
							t.Logf("  Square %c%d: result=%v expected=%v", fileChar, rankNum, inResult, inExpected)
						}
					}
				}
			}
		})
	}
}

// TestGetBishopMask tests the GetBishopMask function for various squares
func TestGetBishopMask(t *testing.T) {
	tests := []struct {
		name     string
		square   Shift
		minBits  int // Minimum number of bits that should be set
		maxBits  int // Maximum number of bits that should be set
	}{
		{"center_d4", 27, 7, 11},  // Center square should have good diagonal coverage
		{"corner_a1", 0, 0, 7},     // Corner has limited diagonal
		{"corner_h8", 63, 0, 7},    // Corner has limited diagonal
		{"edge_e1", 4, 4, 7},       // Edge square
		{"near_center_d5", 35, 7, 11}, // Near center
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coord := CoordsFromShift(tt.square)
			mask := GetBishopMask(coord)
			
			// Count bits in mask
			bitCount := 0
			for i := Shift(0); i < 64; i++ {
				if mask&(BitBoard(1)<<i) != 0 {
					bitCount++
				}
			}
			
			if bitCount < tt.minBits || bitCount > tt.maxBits {
				t.Errorf("Mask bit count %d not in range [%d, %d]", bitCount, tt.minBits, tt.maxBits)
			}
			
			// Verify the square itself is not in the mask
			if mask&(BitBoard(1)<<tt.square) != 0 {
				t.Error("Mask should not include the piece's own square")
			}
		})
	}
}

// TestBuildRookAttacks tests that rook attacks are built correctly
func TestBuildRookAttacks(t *testing.T) {
	BuildRookAttacks()
	
	if ROOK_MAGIC == nil {
		t.Fatal("ROOK_MAGIC is nil")
	}
	
	if len(ROOK_MAGIC) != 64 {
		t.Fatalf("ROOK_MAGIC should have 64 entries, got %d", len(ROOK_MAGIC))
	}
	
	if ROOK_ATTACKS == nil {
		t.Fatal("ROOK_ATTACKS is nil")
	}
	
	if len(ROOK_ATTACKS) != 64 {
		t.Fatalf("ROOK_ATTACKS should have 64 entries, got %d", len(ROOK_ATTACKS))
	}
	
	// Test a simple case: rook at d4 with no blockers
	square := Shift(27) // d4
	board := BitBoard(0)
	attacks := GetRookAttack(square, board)
	
	// Should be able to attack along rank and file
	coord := CoordsFromShift(square)
	mask := (COLUMN_MASK << coord.file) | (ROW_MASK << (coord.rank * 8))
	expectedAttacks := mask & ^(BitBoard(1) << square) // Exclude starting square
	
	if attacks != expectedAttacks {
		t.Errorf("Rook attacks from d4 with no blockers incorrect:\n  Got:      %064b\n  Expected: %064b", attacks, expectedAttacks)
	}
}

// TestBuildBishopAttacks tests that bishop attacks are built correctly
func TestBuildBishopAttacks(t *testing.T) {
	BuildBishopAttacks()
	
	if BISHOP_MAGIC == nil {
		t.Fatal("BISHOP_MAGIC is nil")
	}
	
	if len(BISHOP_MAGIC) != 64 {
		t.Fatalf("BISHOP_MAGIC should have 64 entries, got %d", len(BISHOP_MAGIC))
	}
	
	if BISHOP_ATTACKS == nil {
		t.Fatal("BISHOP_ATTACKS is nil")
	}
	
	if len(BISHOP_ATTACKS) != 64 {
		t.Fatalf("BISHOP_ATTACKS should have 64 entries, got %d", len(BISHOP_ATTACKS))
	}
	
	// Test a simple case: bishop at d4 with no blockers
	square := Shift(27) // d4
	board := BitBoard(0)
	attacks := GetBishopAttack(square, board)
	
	// Should attack diagonals - verify it attacks some key squares
	keySquares := []Shift{
		18, // c3
		20, // e3
		34, // c5
		36, // e5
	}
	
	for _, sq := range keySquares {
		if attacks&(BitBoard(1)<<sq) == 0 {
			coord := CoordsFromShift(sq)
			t.Errorf("Bishop from d4 should attack %c%d", COLUMNS[coord.file], coord.rank+1)
		}
	}
}

// TestGetRookAttackWithBlockers tests rook attacks with various blocker configurations
func TestGetRookAttackWithBlockers(t *testing.T) {
	BuildRookAttacks()
	
	// Rook at d4 (square 27) with blocker at d6 (square 43)
	square := Shift(27)
	blocker := BitBoard(1) << 43
	attacks := GetRookAttack(square, blocker)
	
	// Should include d6 but not d7
	if attacks&(BitBoard(1)<<43) == 0 {
		t.Error("Rook should attack the blocker square")
	}
	
	if attacks&(BitBoard(1)<<51) != 0 {
		t.Error("Rook should not attack beyond the blocker")
	}
}

// TestGetBishopAttackWithBlockers tests bishop attacks with various blocker configurations
func TestGetBishopAttackWithBlockers(t *testing.T) {
	BuildBishopAttacks()
	
	// Bishop at d4 (square 27) with blocker at f6 (square 45)
	square := Shift(27)
	blocker := BitBoard(1) << 45
	attacks := GetBishopAttack(square, blocker)
	
	// Should include f6 but not g7
	if attacks&(BitBoard(1)<<45) == 0 {
		t.Error("Bishop should attack the blocker square")
	}
	
	if attacks&(BitBoard(1)<<54) != 0 {
		t.Error("Bishop should not attack beyond the blocker")
	}
}

// TestGetQueenAttack tests that queen attacks combine rook and bishop
func TestGetQueenAttack(t *testing.T) {
	BuildRookAttacks()
	BuildBishopAttacks()
	
	// Queen at d4 (square 27) with no blockers
	square := Shift(27)
	board := BitBoard(0)
	
	queenAttacks := GetQueenAttack(square, board)
	rookAttacks := GetRookAttack(square, board)
	bishopAttacks := GetBishopAttack(square, board)
	
	// Queen attacks should be the union of rook and bishop attacks
	expected := rookAttacks | bishopAttacks
	
	if queenAttacks != expected {
		t.Error("Queen attacks should be the union of rook and bishop attacks")
	}
	
	// Verify queen attacks in all 8 directions
	// Horizontal/Vertical (rook moves)
	keyRookSquares := []Shift{19, 35, 26, 28} // d3, d5, c4, e4
	for _, sq := range keyRookSquares {
		if queenAttacks&(BitBoard(1)<<sq) == 0 {
			t.Errorf("Queen should attack square %d (rook direction)", sq)
		}
	}
	
	// Diagonal (bishop moves)
	keyBishopSquares := []Shift{18, 20, 34, 36} // c3, e3, c5, e5
	for _, sq := range keyBishopSquares {
		if queenAttacks&(BitBoard(1)<<sq) == 0 {
			t.Errorf("Queen should attack square %d (bishop direction)", sq)
		}
	}
}

// TestBuildAllAttacks tests that BuildAllAttacks initializes everything
func TestBuildAllAttacks(t *testing.T) {
	BuildAllAttacks()
	
	// Verify all attack tables are initialized
	if KNIGHT_ATTACKS == nil {
		t.Error("KNIGHT_ATTACKS not initialized")
	}
	
	if KING_ATTACKS == nil {
		t.Error("KING_ATTACKS not initialized")
	}
	
	if WHITE_PAWN_ATTACKS == nil {
		t.Error("WHITE_PAWN_ATTACKS not initialized")
	}
	
	if BLACK_PAWN_ATTACKS == nil {
		t.Error("BLACK_PAWN_ATTACKS not initialized")
	}
	
	if ROOK_MAGIC == nil {
		t.Error("ROOK_MAGIC not initialized")
	}
	
	if ROOK_ATTACKS == nil {
		t.Error("ROOK_ATTACKS not initialized")
	}
	
	if BISHOP_MAGIC == nil {
		t.Error("BISHOP_MAGIC not initialized")
	}
	
	if BISHOP_ATTACKS == nil {
		t.Error("BISHOP_ATTACKS not initialized")
	}
}
