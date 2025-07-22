package chess

import (
	"errors"
	"math/rand/v2"
)

// there are only 64 knight moves on a chess board
// each index is the shift of the knight, the value is the attack
var KNIGHT_ATTACKS []uint64
var KING_ATTACKS []uint64

// pawns are split up into attacks and move's
// Black and white pecies are split up due to the fact that they are different for pawns.
var (
	WHITE_PAWN_ATTACKS []uint64
	WHITE_PAWN_MOVES   []uint64
	BLACK_PAWN_ATTACKS []uint64
	BLACK_PAWN_MOVES   []uint64
)

// sliding piececs
var (
	ROOK_MAGIC     []MagicEntry //magic numbers
	BISHOP_MAGIC   []MagicEntry
	ROOK_ATTTACKS  [][]uint64
	BISHOP_ATTACKS [][]uint64
)

type MagicEntry struct {
	Mask  uint64
	Magic uint64
	Index uint8
}

// array of vector that tell in which directions for the Ray caster to cast
type Ray [4][2]int

var (
	ROOK_RAY   = Ray{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	BISHOP_RAY = Ray{{1, 1}, {-1, -1}, {1, -1}, {-1, 1}}
)

func MagicIndex(entry MagicEntry, board uint64) uint64 {
	blockers := board & entry.Mask
	hash := blockers * entry.Magic
	index := hash >> (64 - entry.Index)
	return index
}

func GetRookAttack(loc Shift, board int64) uint64 {
	magic := ROOK_MAGIC[loc]
	idx := MagicIndex(magic, uint64(board))
	return ROOK_ATTTACKS[loc][idx]
}

func GetBishopAttack(loc Shift, board int64) uint64 {
	magic := BISHOP_MAGIC[loc]
	idx := MagicIndex(magic, uint64(board))
	return BISHOP_ATTACKS[loc][idx]
}

func GetRookMask(coord Coordinates) uint64 {
	row, col := coord.col, coord.row
	return (COLUMN_MASK << col) | (ROW_MASK << row * 8)
}

func GetBishopMask(coord Coordinates) uint64 {
	//TODO: not done
	return 0
}

func FindMagic(coord Coordinates) []uint64 {
	mask := GetRookMask(coord)
	shift := ShiftFromCoords(coord)
	for {
		test_magic := rand.Uint64() & rand.Uint64() & rand.Uint64()
		magicE := MagicEntry{test_magic, mask, uint8(shift)}
		table, err := TryRookMagic(shift, magicE)
		if err != nil {
			continue
		}
		return table
	}
}

func TryRookMagic(loc Shift, magic MagicEntry) ([]uint64, error) {
	table := make([]uint64, 1<<(64-magic.Index)) //TODO: this need to be check to see if its correct
	var blockers uint64 = 0
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

// TODO:: fix types, this is also a sign that the inital types aren't that good and will need to be changed.
func RayCast(inital Shift, blockers uint64, mask uint64, r Ray) uint64 {

	return 0
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
