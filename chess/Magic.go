package chess

import (
	"errors"
	"log"
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

func MagicIndex(entry MagicEntry, board uint64) uint64 {
	blockers := board & entry.Mask
	hash := blockers * entry.Magic
	index := hash >> (64 - entry.Index)
	return index
}

func GetRookAttack(loc Location, board int64) uint64 {
	magic := ROOK_MAGIC[loc]
	idx := MagicIndex(magic, uint64(board))
	return ROOK_ATTTACKS[loc][idx]
}

func GetBishopAttack(loc Location, board int64) uint64 {
	magic := BISHOP_MAGIC[loc]
	idx := MagicIndex(magic, uint64(board))
	return BISHOP_ATTACKS[loc][idx]
}

func GetRookMask(alg string) uint64 {
	row, col, err := RowColFromAlg(alg)
	if err != nil {
		log.Fatal(err.Error())
	}
	return (COLUMN_MASK << col) | (ROW_MASK << row * 8)
}

func GetBishopMask(alg string) uint64 {
	//TODO: not done
	return 0
}

func FindMagicRook(alg string) []uint64 {
	loc, err := ShiftFromAlg(alg)
	if err != nil {
		log.Fatal(err.Error())
	}
	mask := GetRookMask(alg)
	for {
		test_magic := rand.Uint64() & rand.Uint64() & rand.Uint64()
		magicE := MagicEntry{test_magic, mask, uint8(loc)}
		table, err := TryRookMagic(Location(loc), magicE)
		if err != nil {
			continue
		}
		return table
	}
}

func TryRookMagic(loc Location, magic MagicEntry) ([]uint64, error) {
	table := make([]uint64, 1<<(64-magic.Index))
	var blockers uint64 = 0
	mask := magic.Mask

	for true {
		moves := RayCast(loc, blockers, mask)
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
func RayCast(inital Location, blockers uint64, mask uint64) uint64 {
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
