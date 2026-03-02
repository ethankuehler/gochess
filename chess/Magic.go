package chess

import (
	"errors"
	"math/rand/v2"
)

// Magic limit
const MAGIC_LIMIT = 1_000_000

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
	return (COLUMN_MASK << col) | (ROW_MASK << row * ROW_COL_SIZE)
}

func GetBishopMask(coord Coordinates) BitBoard {
	return RayCast(ShiftFromCoords(coord), 0, 0, BISHOP_RAY)
}

func FindMagic(coord Coordinates) ([]BitBoard, error) {
	mask := GetRookMask(coord)
	shift := ShiftFromCoords(coord)
	for _ = range MAGIC_LIMIT {
		test_magic := rand.Uint64() & rand.Uint64() & rand.Uint64()
		magicE := MagicEntry{mask, test_magic, shift}
		table, err := TryRookMagic(shift, magicE)
		if err != nil {
			continue
		}
		return table, nil
	}
	return nil, errors.New("hit magic limit, magic not found")

}

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

func RayCast(inital Shift, blockers BitBoard, _ BitBoard, r Ray) BitBoard {
	coord := CoordsFromShift(inital)
	row, col := int(coord.row), int(coord.col)
	var attacks BitBoard = 0

	for _, direction := range r {
		dRow, dCol := direction[0], direction[1]
		if dRow == 0 && dCol == 0 {
			continue
		}
		rayRow, rayCol := row+dRow, col+dCol
		for rayRow >= 0 && rayRow < ROW_COL_SIZE && rayCol >= 0 && rayCol < ROW_COL_SIZE {
			loc := BitBoard(1) << ShiftFromCoords(Coordinates{uint64(rayRow), uint64(rayCol)})
			attacks |= loc
			if blockers&loc > 0 {
				break
			}
			rayRow += dRow
			rayCol += dCol
		}
	}

	return attacks
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
