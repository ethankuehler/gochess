package chess

import (
	"errors"
	"math/rand/v2"
)

// there are only 64 knight moves on a chess board
// each index is the shift of the knight, the value is the attack
var KNIGHT_ATTACKS []BitBoard
var KING_ATTACKS []BitBoard

// pawns are split up into attacks and move's
// Black and white pecies are split up due to the fact that they are different for pawns.
var (
	WHITE_PAWN_ATTACKS []BitBoard
	WHITE_PAWN_MOVES   []BitBoard
	BLACK_PAWN_ATTACKS []BitBoard
	BLACK_PAWN_MOVES   []BitBoard
)

// sliding piececs
var (
	ROOK_MAGIC     []MagicEntry //magic numbers
	BISHOP_MAGIC   []MagicEntry
	ROOK_ATTTACKS  [][]BitBoard
	BISHOP_ATTACKS [][]BitBoard
)

type MagicEntry struct {
	Mask  BitBoard
	Magic uint64
	Index Shift
}

// array of vector that tell in which directions for the Ray caster to cast
type Ray [4][2]int

var (
	ROOK_RAY   = Ray{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	BISHOP_RAY = Ray{{1, 1}, {-1, -1}, {1, -1}, {-1, 1}}
)

func MagicIndex(entry MagicEntry, board BitBoard) uint64 {
	blockers := board & entry.Mask
	hash := uint64(blockers) * entry.Magic
	index := hash >> (64 - entry.Index)
	return index
}

func GetRookAttack(loc Shift, board BitBoard) BitBoard {
	magic := ROOK_MAGIC[loc]
	idx := MagicIndex(magic, board)
	return ROOK_ATTTACKS[loc][idx]
}

func GetBishopAttack(loc Shift, board BitBoard) BitBoard {
	magic := BISHOP_MAGIC[loc]
	idx := MagicIndex(magic, board)
	return BISHOP_ATTACKS[loc][idx]
}

func GetRookMask(coord Coordinates) BitBoard {
	row, col := coord.col, coord.row
	return (COLUMN_MASK << col) | (ROW_MASK << row * 8)
}

func GetBishopMask(coord Coordinates) BitBoard {
	//TODO: not done
	return 0
}

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

func TryRookMagic(loc Shift, magic MagicEntry) ([]BitBoard, error) {
	table := make([]BitBoard, 1<<(64-magic.Index)) //TODO: this need to be check to see if its correct
	var blockers BitBoard = 0
	mask := magic.Mask

	for true {
		moves := RayCast(loc, blockers, mask, Ray{})
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
//   - r: Array of direction vectors [row_delta, col_delta] to cast rays in
// Returns: BitBoard with all valid destination squares
func RayCast(initial Shift, blockers BitBoard, mask BitBoard, r Ray) BitBoard {
	var result BitBoard = 0
	coord := CoordsFromShift(initial)
	// Note: The Coordinates struct has swapped field names!
	// coord.col actually contains the rank, coord.row contains the file
	row, col := coord.col, coord.row // Swap to get correct values
	
	// Cast a ray in each direction
	for _, dir := range r {
		rowDelta := dir[0]
		colDelta := dir[1]
		
		// Skip directions with no movement (would cause infinite loop)
		if rowDelta == 0 && colDelta == 0 {
			continue
		}
		
		// Start from the initial position and move in the direction
		currentRow := int(row) + rowDelta
		currentCol := int(col) + colDelta
		
		// Continue casting the ray until we hit a blocker or edge
		for currentRow >= 0 && currentRow < 8 && currentCol >= 0 && currentCol < 8 {
			// Calculate the shift for this square
			// shift = col (file) + row (rank) * 8
			square := Shift(currentCol + currentRow*8)
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
			currentRow += rowDelta
			currentCol += colDelta
		}
	}
	
	return result
}

func BuildAllAttacks() {
	BuildKnightAttacks()
	BuildKingAttacks()
	BuildPawnMoves()
	BuildPawnAttacks()
	//BuildRookAttacks()
	//BuildBishopAttacks()
	//BuildQueenAttacks()
}

func BuildKnightAttacks() {
	file_name := "data/knight_attacks.csv"
	KNIGHT_ATTACKS = LoadAttacks(file_name)
}

func BuildKingAttacks() {
	file_name := "data/king_attacks.csv"
	KING_ATTACKS = LoadAttacks(file_name)
}

func BuildPawnMoves() {
	file_name := "data/white_pawn_move.csv"
	WHITE_PAWN_MOVES = LoadAttacks(file_name)
	file_name = "data/black_pawn_move.csv"
	BLACK_PAWN_MOVES = LoadAttacks(file_name)
}

func BuildPawnAttacks() {
	file_name := "data/white_pawn_attacks.csv"
	WHITE_PAWN_ATTACKS = LoadAttacks(file_name)
	file_name = "data/black_pawn_attacks.csv"
	BLACK_PAWN_ATTACKS = LoadAttacks(file_name)

}
