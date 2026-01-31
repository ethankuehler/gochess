package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ethankuehler/gochess/chess"
)

func main() {
	// Initialize all attack tables
	chess.BuildAllAttacks()

	fmt.Println("Generating test data files...")

	// Generate pawn attack and move files
	if err := generatePawnAttacks(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating pawn attacks: %v\n", err)
		os.Exit(1)
	}

	if err := generatePawnMoves(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating pawn moves: %v\n", err)
		os.Exit(1)
	}

	// Generate knight attacks
	if err := generateKnightAttacks(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating knight attacks: %v\n", err)
		os.Exit(1)
	}

	// Generate king attacks
	if err := generateKingAttacks(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating king attacks: %v\n", err)
		os.Exit(1)
	}

	// Generate raycast tests
	if err := generateRaycastTests(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating raycast tests: %v\n", err)
		os.Exit(1)
	}

	// Generate FEN tests
	if err := generateFENTests(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating FEN tests: %v\n", err)
		os.Exit(1)
	}

	// Generate magic number files
	fmt.Println("Generating Rook Magic Numbers...")
	rookMagics := chess.GenerateRookMagics()
	if err := chess.SaveRookMagicsToCSV(rookMagics, "data/rook_magic.csv"); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving rook magics: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Rook magics saved successfully!")

	fmt.Println("Generating Bishop Magic Numbers...")
	bishopMagics := chess.GenerateBishopMagics()
	if err := chess.SaveBishopMagicsToCSV(bishopMagics, "data/bishop_magic.csv"); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving bishop magics: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Bishop magics saved successfully!")

	fmt.Println("\nAll test data files generated successfully!")
}

func generatePawnAttacks() error {
	// White pawn attacks
	file, err := os.Create("data/white_pawn_attacks.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"", "start", "move"})
	for sq := chess.Shift(0); sq < 64; sq++ {
		attacks := chess.WHITE_PAWN_ATTACKS[sq]
		row := []string{
			strconv.Itoa(int(sq)),
			strconv.FormatUint(uint64(1<<sq), 10),
			strconv.FormatUint(uint64(attacks), 10),
		}
		writer.Write(row)
	}

	// Black pawn attacks
	file2, err := os.Create("data/black_pawn_attacks.csv")
	if err != nil {
		return err
	}
	defer file2.Close()

	writer2 := csv.NewWriter(file2)
	defer writer2.Flush()

	writer2.Write([]string{"", "start", "move"})
	for sq := chess.Shift(0); sq < 64; sq++ {
		attacks := chess.BLACK_PAWN_ATTACKS[sq]
		row := []string{
			strconv.Itoa(int(sq)),
			strconv.FormatUint(uint64(1<<sq), 10),
			strconv.FormatUint(uint64(attacks), 10),
		}
		writer2.Write(row)
	}

	fmt.Println("Generated pawn attacks files")
	return nil
}

func generatePawnMoves() error {
	// White pawn moves
	file, err := os.Create("data/white_pawn_move.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"", "start", "move"})
	for sq := chess.Shift(0); sq < 64; sq++ {
		moves := chess.WHITE_PAWN_MOVES[sq]
		row := []string{
			strconv.Itoa(int(sq)),
			strconv.FormatUint(uint64(1<<sq), 10),
			strconv.FormatUint(uint64(moves), 10),
		}
		writer.Write(row)
	}

	// Black pawn moves
	file2, err := os.Create("data/black_pawn_move.csv")
	if err != nil {
		return err
	}
	defer file2.Close()

	writer2 := csv.NewWriter(file2)
	defer writer2.Flush()

	writer2.Write([]string{"", "start", "move"})
	for sq := chess.Shift(0); sq < 64; sq++ {
		moves := chess.BLACK_PAWN_MOVES[sq]
		row := []string{
			strconv.Itoa(int(sq)),
			strconv.FormatUint(uint64(1<<sq), 10),
			strconv.FormatUint(uint64(moves), 10),
		}
		writer2.Write(row)
	}

	fmt.Println("Generated pawn moves files")
	return nil
}

func generateKnightAttacks() error {
	file, err := os.Create("data/knight_attacks.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"", "start", "move"})
	for sq := chess.Shift(0); sq < 64; sq++ {
		attacks := chess.KNIGHT_ATTACKS[sq]
		row := []string{
			strconv.Itoa(int(sq)),
			strconv.FormatUint(uint64(1<<sq), 10),
			strconv.FormatUint(uint64(attacks), 10),
		}
		writer.Write(row)
	}

	fmt.Println("Generated knight attacks file")
	return nil
}

func generateKingAttacks() error {
	file, err := os.Create("data/king_attacks.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"", "start", "move"})
	for sq := chess.Shift(0); sq < 64; sq++ {
		attacks := chess.KING_ATTACKS[sq]
		row := []string{
			strconv.Itoa(int(sq)),
			strconv.FormatUint(uint64(1<<sq), 10),
			strconv.FormatUint(uint64(attacks), 10),
		}
		writer.Write(row)
	}

	fmt.Println("Generated king attacks file")
	return nil
}

func squaresToString(bb chess.BitBoard) string {
	var squares []string
	for sq := chess.Shift(0); sq < 64; sq++ {
		if bb&(chess.BitBoard(1)<<sq) != 0 {
			loc := chess.BitBoard(1) << sq
			alg := chess.AlgFromLoc(loc)
			squares = append(squares, alg)
		}
	}
	return strings.Join(squares, ",")
}

func generateRaycastTests() error {
	file, err := os.Create("data/raycast_tests.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"name", "piece_type", "piece_square", "fen_blockers", "expected_squares"})

	// Rook tests
	tests := []struct {
		name        string
		pieceType   string
		square      string
		fenBlockers string
	}{
		{"rook_center_no_blockers", "rook", "d4", "8/8/8/8/8/8/8/8"},
		{"rook_center_one_blocker_up", "rook", "d4", "8/8/3P4/8/8/8/8/8"},
		{"rook_center_one_blocker_down", "rook", "d4", "8/8/8/8/8/8/3P4/8"},
		{"rook_center_one_blocker_right", "rook", "d4", "8/8/8/8/5P2/8/8/8"},
		{"rook_center_one_blocker_left", "rook", "d4", "8/8/8/8/1P6/8/8/8"},
		{"rook_center_multiple_blockers", "rook", "d4", "8/8/3P4/8/2P1P3/8/3P4/8"},
		{"rook_corner_a1_no_blockers", "rook", "a1", "8/8/8/8/8/8/8/8"},
		{"rook_corner_a1_with_blockers", "rook", "a1", "8/8/8/8/8/P7/8/2P5"},
		{"rook_corner_h8_no_blockers", "rook", "h8", "8/8/8/8/8/8/8/8"},
		{"rook_corner_h8_with_blockers", "rook", "h8", "3P4/8/8/8/8/8/8/7P"},
		{"rook_edge_e1_horizontal_blockers", "rook", "e1", "8/8/8/8/8/8/8/2P3P1"},
		{"rook_edge_a5_mixed_blockers", "rook", "a5", "8/8/P7/8/8/P7/8/8"},
		{"rook_fully_blocked_all_sides", "rook", "d4", "8/8/8/3P4/2P1P3/3P4/8/8"},
		// Bishop tests
		{"bishop_center_no_blockers", "bishop", "d4", "8/8/8/8/8/8/8/8"},
		{"bishop_center_one_blocker_ne", "bishop", "d4", "8/8/5P2/8/8/8/8/8"},
		{"bishop_center_one_blocker_nw", "bishop", "d4", "8/8/1P6/8/8/8/8/8"},
		{"bishop_center_one_blocker_se", "bishop", "d4", "8/8/8/8/8/8/5P2/8"},
		{"bishop_center_one_blocker_sw", "bishop", "d4", "8/8/8/8/8/2P5/8/8"},
		{"bishop_center_multiple_blockers", "bishop", "d4", "8/8/5P2/8/8/2P5/8/8"},
	}

	for _, test := range tests {
		square, _ := chess.ShiftFromAlg(test.square)
		
		// Parse FEN to blockers
		blockers := parseFENToBlockers(test.fenBlockers)
		
		// Get mask and ray based on piece type
		var mask chess.BitBoard
		var ray chess.Ray
		
		if test.pieceType == "rook" {
			// Get file and rank from coord
			file := uint64(square % 8)
			rank := uint64(square / 8)
			mask = (chess.COLUMN_MASK << file) | (chess.ROW_MASK << (rank * 8))
			ray = chess.ROOK_RAY
		} else { // bishop
			// Get file and rank from coord
			file := int(square % 8)
			rank := int(square / 8)
			mask = chess.BitBoard(0)
			for i := 0; i < 8; i++ {
				for j := 0; j < 8; j++ {
					if (i-rank) == (j-file) || (i-rank) == -(j-file) {
						mask |= chess.BitBoard(1) << (j + i*8)
					}
				}
			}
			ray = chess.BISHOP_RAY
		}
		
		// Run RayCast
		result := chess.RayCast(square, blockers, mask, ray)
		
		// Convert result to square list
		expectedSquares := squaresToString(result)
		
		row := []string{test.name, test.pieceType, test.square, test.fenBlockers, expectedSquares}
		writer.Write(row)
	}

	fmt.Println("Generated raycast tests file")
	return nil
}

func parseFENToBlockers(fen string) chess.BitBoard {
	var blockers chess.BitBoard = 0
	ranks := strings.Split(fen, "/")
	
	for rankIdx, rankStr := range ranks {
		fileIdx := 0
		for _, ch := range rankStr {
			if ch >= '1' && ch <= '8' {
				fileIdx += int(ch - '0')
			} else {
				// Any piece is a blocker
				shift := chess.Shift(fileIdx + (7-rankIdx)*8)
				blockers |= chess.BitBoard(1) << shift
				fileIdx++
			}
		}
	}
	
	return blockers
}

func generateFENTests() error {
	file, err := os.Create("data/FEN.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	// No CSV writer - just write raw text to match original format
	// Add standard FEN test cases
	tests := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		"rnbqkbnr/pp1ppppp/8/2p5/4P3/8/PPPP1PPP/RNBQKBNR w KQkq c6 0 2",
		"r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4",
		"8/8/8/8/8/8/8/8 w - - 0 1",
		"rnbq1bnr/ppp1pkpp/8/3pPp2/8/2N5/PPPP1PPP/R1BQKBNR w KQ d6 0 4",
		"rnbqkbnr/pppp3p/8/4pPp1/8/8/PPPPKPPP/RNBQ1BNR w kq g6 0 4",
	}

	for _, fen := range tests {
		_, err := file.WriteString(fen + ",\n")
		if err != nil {
			return err
		}
	}

	fmt.Println("Generated FEN tests file")
	return nil
}
