package chess

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
)

// there are only 64 knight moves on a chess board
// each index is the shift of the knight, the value is the attack
var KNIGHT_ATTACKS []BitBoard
var KING_ATTACKS []BitBoard

// pawns are split up into attacks and move's
// Black and white pieces are split up due to the fact that they are different for pawns.
var (
	WHITE_PAWN_ATTACKS []BitBoard
	WHITE_PAWN_MOVES   []BitBoard
	BLACK_PAWN_ATTACKS []BitBoard
	BLACK_PAWN_MOVES   []BitBoard
)

// sliding pieces
var (
	ROOK_MAGIC     []MagicEntry //magic numbers
	BISHOP_MAGIC   []MagicEntry
	ROOK_ATTACKS   [][]BitBoard
	BISHOP_ATTACKS [][]BitBoard
)

// MagicEntry holds the magic bitboard data for a single square.
// Used for efficiently computing sliding piece attacks (rook/bishop).
type MagicEntry struct {
	Mask  BitBoard // Relevant occupancy mask for this square
	Magic uint64   // Magic number for perfect hashing
	Index Shift    // Number of bits in the hash index
}

// Ray represents an array of direction vectors for raycasting.
// Each direction is a [2]int: [rank_delta, file_delta]
// array of vector that tell in which directions for the Ray caster to cast
type Ray [4][2]int

var (
	ROOK_RAY   = Ray{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	BISHOP_RAY = Ray{{1, 1}, {-1, -1}, {1, -1}, {-1, 1}}
)

// MagicIndex computes the hash index for a magic bitboard lookup.
// It masks the occupied squares, multiplies by the magic number,
// and shifts to produce an index into the attack table.
func MagicIndex(entry MagicEntry, board BitBoard) uint64 {
	blockers := board & entry.Mask
	hash := uint64(blockers) * entry.Magic
	index := hash >> (64 - entry.Index)
	return index
}

// GetRookAttack returns the attack bitboard for a rook at the given location.
// Uses magic bitboard technique for O(1) lookup.
// Parameters:
//   - loc: Square position of the rook (0-63)
//   - board: BitBoard representing all occupied squares
//
// Returns: BitBoard with all squares the rook can attack
func GetRookAttack(loc Shift, board BitBoard) BitBoard {
	magic := ROOK_MAGIC[loc]
	idx := MagicIndex(magic, board)
	return ROOK_ATTACKS[loc][idx]
}

// GetBishopAttack returns the attack bitboard for a bishop at the given location.
// Uses magic bitboard technique for O(1) lookup.
// Parameters:
//   - loc: Square position of the bishop (0-63)
//   - board: BitBoard representing all occupied squares
//
// Returns: BitBoard with all squares the bishop can attack
func GetBishopAttack(loc Shift, board BitBoard) BitBoard {
	magic := BISHOP_MAGIC[loc]
	idx := MagicIndex(magic, board)
	return BISHOP_ATTACKS[loc][idx]
}

// GetQueenAttack returns the attack bitboard for a queen at the given location.
// Queens combine rook and bishop movement patterns.
// Parameters:
//   - loc: Square position of the queen (0-63)
//   - board: BitBoard representing all occupied squares
//
// Returns: BitBoard with all squares the queen can attack
func GetQueenAttack(loc Shift, board BitBoard) BitBoard {
	return GetRookAttack(loc, board) | GetBishopAttack(loc, board)
}

// GetRookMask returns the relevant occupancy mask for a rook at the given coordinates.
// The mask includes all squares on the same rank and file as the rook.
func GetRookMask(coord Coordinates) BitBoard {
	rank, file := coord.rank, coord.file
	return (COLUMN_MASK << file) | (ROW_MASK << (rank * 8))
}

// GetBishopMask returns the relevant occupancy mask for a bishop at the given coordinates.
// The mask includes all squares on the diagonals passing through the bishop's position.
// Edge squares are typically excluded for magic bitboard optimization.
func GetBishopMask(coord Coordinates) BitBoard {
	rank, file := coord.rank, coord.file
	var mask BitBoard = 0

	// For each of the 4 diagonal directions
	for _, dir := range BISHOP_RAY {
		rankDelta, fileDelta := dir[0], dir[1]
		r, f := int(rank), int(file)

		// Move in direction until edge
		for {
			r += rankDelta
			f += fileDelta

			// Stop at board edges
			if r < 0 || r >= 8 || f < 0 || f >= 8 {
				break
			}

			// Optionally exclude edge squares for optimization
			// (common practice in magic bitboards to reduce table size)
			if r > 0 && r < 7 && f > 0 && f < 7 {
				square := Shift(f + r*8)
				mask |= BitBoard(1) << square
			}
		}
	}

	return mask
}

// FindMagic searches for a valid magic number and attack table for a rook at the given coordinates.
// This is a brute-force search that tests random magic numbers until one works.
// Returns the precomputed attack table for all possible blocker configurations.
func FindMagic(coord Coordinates) []BitBoard {
	mask := GetRookMask(coord)
	shift := ShiftFromCoords(coord)
	for {
		test_magic := rand.Uint64() & rand.Uint64() & rand.Uint64()
		magicE := MagicEntry{mask, test_magic, shift}
		table, err := TryRookMagic(shift, magicE)
		if err != nil {
			continue
		}
		return table
	}
}

// TryRookMagic attempts to build an attack table using the given magic number.
// It iterates through all possible blocker configurations and uses the magic number
// to hash them into the attack table. If any collisions occur with different attack
// patterns, the magic number is invalid.
// Parameters:
//   - loc: Square position of the rook (0-63)
//   - magic: MagicEntry containing the magic number and mask to test
//
// Returns: The attack table if successful, or an error if the magic number causes collisions
func TryRookMagic(loc Shift, magic MagicEntry) ([]BitBoard, error) {
	table := make([]BitBoard, 1<<(64-magic.Index)) //TODO: this need to be check to see if its correct
	var blockers BitBoard = 0
	mask := magic.Mask

	for true {
		moves := RayCast(loc, blockers, mask, ROOK_RAY)
		table_entry := &table[MagicIndex(magic, blockers)]
		if *table_entry == 0 {
			*table_entry = moves
		} else if *table_entry != moves {
			return nil, errors.New("invalid magic")
		}

		blockers = (blockers - mask) & mask
		if blockers == 0 {
			break
		}
	}

	return table, nil
}

// RayCast generates a bitboard of valid moves for a sliding piece from a given position.
// It casts rays in the directions specified by the Ray array until hitting a blocker or board edge.
// Parameters:
//   - initial: The starting position on the board (0-63)
//   - blockers: BitBoard of occupied squares that block movement
//   - mask: BitBoard mask limiting valid squares for this piece type
//   - r: Array of direction vectors [rank_delta, file_delta] to cast rays in
//
// Returns: BitBoard with all valid destination squares
func RayCast(initial Shift, blockers BitBoard, mask BitBoard, r Ray) BitBoard {
	var result BitBoard = 0
	coord := CoordsFromShift(initial)
	rank, file := coord.rank, coord.file

	// Cast a ray in each direction
	for _, dir := range r {
		rankDelta := dir[0]
		fileDelta := dir[1]

		// Skip directions with no movement (would cause infinite loop)
		if rankDelta == 0 && fileDelta == 0 {
			continue
		}

		// Start from the initial position and move in the direction
		currentRank := int(rank) + rankDelta
		currentFile := int(file) + fileDelta

		// Continue casting the ray until we hit a blocker or edge
		for currentRank >= 0 && currentRank < 8 && currentFile >= 0 && currentFile < 8 {
			// Calculate the shift for this square
			// shift = file + rank * 8
			square := Shift(currentFile + currentRank*8)
			squareBit := BitBoard(1) << square

			// Check if this square is within the mask
			if mask&squareBit != 0 {
				// Add this square to the result
				result |= squareBit

				// If this square has a blocker, stop the ray here (but include the blocker)
				if blockers&squareBit != 0 {
					break
				}
			}

			// Move to the next square in this direction
			currentRank += rankDelta
			currentFile += fileDelta
		}
	}

	return result
}

// BuildAllAttacks initializes all pre-computed attack tables.
// Call this once at program startup to load attack data from CSV files.
// Loads: Knight, King, Pawn attacks, and generates Rook and Bishop attacks using magic bitboards.
func BuildAllAttacks() {
	BuildAllAttacksWithOption(false)
}

// BuildAllAttacksWithOption initializes all pre-computed attack tables with generation option.
// If autoGenerate is true, missing magic number CSV files will be generated automatically.
func BuildAllAttacksWithOption(autoGenerate bool) {
	BuildKnightAttacks()
	BuildKingAttacks()
	BuildPawnMoves()
	BuildPawnAttacks()
	BuildRookAttacksWithOption(autoGenerate)
	BuildBishopAttacksWithOption(autoGenerate)
}

// BuildKnightAttacks loads the pre-computed knight attack table from CSV.
// The table contains attack bitboards for all 64 squares.
func BuildKnightAttacks() {
	file_name := "data/knight_attacks.csv"
	KNIGHT_ATTACKS = LoadAttacks(file_name)
}

// BuildKingAttacks loads the pre-computed king attack table from CSV.
// The table contains attack bitboards for all 64 squares.
func BuildKingAttacks() {
	file_name := "data/king_attacks.csv"
	KING_ATTACKS = LoadAttacks(file_name)
}

// BuildPawnMoves loads the pre-computed pawn movement tables from CSV.
// Separate tables for white and black pawns since they move in opposite directions.
func BuildPawnMoves() {
	file_name := "data/white_pawn_move.csv"
	WHITE_PAWN_MOVES = LoadAttacks(file_name)
	file_name = "data/black_pawn_move.csv"
	BLACK_PAWN_MOVES = LoadAttacks(file_name)
}

// BuildPawnAttacks loads the pre-computed pawn attack tables from CSV.
// Separate tables for white and black pawns since they attack diagonally in opposite directions.
func BuildPawnAttacks() {
	file_name := "data/white_pawn_attacks.csv"
	WHITE_PAWN_ATTACKS = LoadAttacks(file_name)
	file_name = "data/black_pawn_attacks.csv"
	BLACK_PAWN_ATTACKS = LoadAttacks(file_name)
}

// BuildRookAttacks loads the magic numbers and generates the attack lookup tables for rooks.
// This function initializes the ROOK_MAGIC and ROOK_ATTACKS global variables.
// It will panic if the CSV file doesn't exist. For auto-generation support, use BuildRookAttacksWithOption(true).
func BuildRookAttacks() {
	BuildRookAttacksWithOption(false)
}

// BuildRookAttacksWithOption loads or generates rook magic numbers based on options.
// If autoGenerate is true and the CSV file doesn't exist, it will generate and save magic numbers.
func BuildRookAttacksWithOption(autoGenerate bool) {
	// Try to load magic numbers from CSV
	magics, err := LoadMagicsFromCSV("data/rook_magic.csv")
	if err != nil {
		// Only auto-generate if the file doesn't exist
		// For other errors (permissions, corrupt data), fail immediately
		if autoGenerate && os.IsNotExist(err) {
			// Generate magic numbers if they don't exist
			magics = GenerateRookMagics()
			// Validate generated magics
			if magics == nil || len(magics) != 64 {
				panic("Failed to generate valid rook magic numbers")
			}
			// Try to save them for future use
			if saveErr := SaveRookMagicsToCSV(magics, "data/rook_magic.csv"); saveErr != nil {
				// Log warning but continue - generation succeeded
				fmt.Fprintf(os.Stderr, "Warning: Failed to save rook magic numbers: %v\n", saveErr)
			}
		} else {
			panic("Failed to load rook magic numbers: " + err.Error())
		}
	}

	// Initialize arrays
	ROOK_MAGIC = magics
	ROOK_ATTACKS = make([][]BitBoard, 64)

	// For each square, generate all attack patterns
	for square := Shift(0); square < 64; square++ {
		magic := ROOK_MAGIC[square]

		// Allocate attack table for this square
		tableSize := 1 << magic.Index
		ROOK_ATTACKS[square] = make([]BitBoard, tableSize)

		// Generate all possible blocker configurations
		var blockers BitBoard = 0
		mask := magic.Mask

		for {
			// Generate attacks for this blocker configuration
			attacks := RayCast(square, blockers, mask, ROOK_RAY)

			// Store in table at hashed index
			index := MagicIndex(magic, blockers)
			ROOK_ATTACKS[square][index] = attacks

			// Next blocker configuration (Carry-Rippler trick)
			blockers = (blockers - mask) & mask
			if blockers == 0 {
				break
			}
		}
	}
}

// BuildBishopAttacks loads the magic numbers and generates the attack lookup tables for bishops.
// This function initializes the BISHOP_MAGIC and BISHOP_ATTACKS global variables.
// It will panic if the CSV file doesn't exist. For auto-generation support, use BuildBishopAttacksWithOption(true).
func BuildBishopAttacks() {
	BuildBishopAttacksWithOption(false)
}

// BuildBishopAttacksWithOption loads or generates bishop magic numbers based on options.
// If autoGenerate is true and the CSV file doesn't exist, it will generate and save magic numbers.
func BuildBishopAttacksWithOption(autoGenerate bool) {
	// Try to load magic numbers from CSV
	magics, err := LoadMagicsFromCSV("data/bishop_magic.csv")
	if err != nil {
		// Only auto-generate if the file doesn't exist
		// For other errors (permissions, corrupt data), fail immediately
		if autoGenerate && os.IsNotExist(err) {
			// Generate magic numbers if they don't exist
			magics = GenerateBishopMagics()
			// Validate generated magics
			if magics == nil || len(magics) != 64 {
				panic("Failed to generate valid bishop magic numbers")
			}
			// Try to save them for future use
			if saveErr := SaveBishopMagicsToCSV(magics, "data/bishop_magic.csv"); saveErr != nil {
				// Log warning but continue - generation succeeded
				fmt.Fprintf(os.Stderr, "Warning: Failed to save bishop magic numbers: %v\n", saveErr)
			}
		} else {
			panic("Failed to load bishop magic numbers: " + err.Error())
		}
	}

	// Initialize arrays
	BISHOP_MAGIC = magics
	BISHOP_ATTACKS = make([][]BitBoard, 64)

	// For each square, generate all attack patterns
	for square := Shift(0); square < 64; square++ {
		magic := BISHOP_MAGIC[square]

		// Allocate attack table for this square
		tableSize := 1 << magic.Index
		BISHOP_ATTACKS[square] = make([]BitBoard, tableSize)

		// Generate all possible blocker configurations
		var blockers BitBoard = 0
		mask := magic.Mask

		for {
			// Generate attacks for this blocker configuration
			attacks := RayCast(square, blockers, mask, BISHOP_RAY)

			// Store in table at hashed index
			index := MagicIndex(magic, blockers)
			BISHOP_ATTACKS[square][index] = attacks

			// Next blocker configuration (Carry-Rippler trick)
			blockers = (blockers - mask) & mask
			if blockers == 0 {
				break
			}
		}
	}
}
