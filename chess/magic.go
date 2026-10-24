package chess

import (
	"errors"
	"fmt"
	"log"
	"math/bits"
	"math/rand/v2"
)

// Magic limit
const MAGIC_LIMIT = 1_000_000

// LoadMagicsFromDisk controls whether the sliding-piece builders restore
// magics from the data/ CSVs (true) or search for new ones (false). It
// defaults to true; when a CSV is missing or invalid the builder logs the
// problem and falls back to a fresh search.
var LoadMagicsFromDisk = true

// SaveMagicsToDisk makes the sliding-piece builders write freshly generated
// magics to the data/ CSVs after a random search. It has no effect when
// LoadMagicsFromDisk is set.
var SaveMagicsToDisk = false

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
	rookAttacks    [][]BitBoard
	BISHOP_ATTACKS [][]BitBoard
)

// MagicEntry holds a magic-bitboard lookup for one square: Mask is the
// relevant occupancy squares, Magic the multiplier, and IndexBits the number
// of set bits in Mask. The attack table holds 1 << IndexBits entries.
type MagicEntry struct {
	Mask      BitBoard
	Magic     uint64
	IndexBits uint
}

// NewMagicEntry computes IndexBits from Mask so the two can never disagree.
func NewMagicEntry(mask BitBoard, magic uint64) MagicEntry {
	return MagicEntry{mask, magic, uint(bits.OnesCount64(uint64(mask)))}
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
	index := hash >> (64 - entry.IndexBits)
	return index
}

func GetRookAttack(loc Shift, board BitBoard) BitBoard {
	magic := ROOK_MAGIC[loc]
	idx := MagicIndex(magic, board)
	return rookAttacks[loc][idx]
}

func GetBishopAttack(loc Shift, board BitBoard) BitBoard {
	magic := BISHOP_MAGIC[loc]
	idx := MagicIndex(magic, board)
	return BISHOP_ATTACKS[loc][idx]
}

// GetQueenAttack combines the rook and bishop lookups; queens need no
// tables of their own.
func GetQueenAttack(loc Shift, board BitBoard) BitBoard {
	return GetRookAttack(loc, board) | GetBishopAttack(loc, board)
}

// RelevantBlockers returns the occupancy mask for a sliding piece on coord:
// the squares along its rays whose occupancy can change the attack set.
// The terminal edge square of each ray is excluded (a ray attacks its
// terminal square whether it is occupied or not, so that bit carries no
// information), as is the origin square.
func RelevantBlockers(coord Coordinates, ray Ray) BitBoard {
	var mask BitBoard = 0
	row, col := int(coord.row), int(coord.col)

	for _, direction := range ray {
		dRow, dCol := direction[0], direction[1]
		if dRow == 0 && dCol == 0 {
			continue
		}
		rayRow, rayCol := row, col
		for {
			nextRow, nextCol := rayRow+dRow, rayCol+dCol
			if nextRow < 0 || nextRow >= ROW_COL_SIZE || nextCol < 0 || nextCol >= ROW_COL_SIZE {
				break
			}
			mask |= BitBoard(1) << ShiftFromCoords(Coordinates{row: uint64(rayRow), col: uint64(rayCol)})
			rayRow, rayCol = nextRow, nextCol
		}
	}

	return mask &^ (BitBoard(1) << ShiftFromCoords(coord))
}

func GetBishopMask(coord Coordinates) BitBoard {
	return RayCast(ShiftFromCoords(coord), 0, BISHOP_RAY)
}

func FindMagic(coord Coordinates, ray Ray) (MagicEntry, []BitBoard, error) {
	mask := RelevantBlockers(coord, ray)
	shift := ShiftFromCoords(coord)
	for _ = range MAGIC_LIMIT {
		candidate := rand.Uint64() & rand.Uint64() & rand.Uint64()
		entry := NewMagicEntry(mask, candidate)
		table, err := TryMagic(shift, entry, ray)
		if err != nil {
			continue
		}
		return entry, table, nil
	}
	return MagicEntry{}, nil, errors.New("hit magic limit, magic not found")

}

// TryMagic attempts to fill an attack table for one square using the
// candidate magic, failing on any non-constructive collision (two blocker
// configurations with different attack sets hashing to the same slot).
// Attack sets are never empty, so a zero slot means "not yet written".
func TryMagic(loc Shift, magic MagicEntry, ray Ray) ([]BitBoard, error) {
	table := make([]BitBoard, 1<<magic.IndexBits)
	var blockers BitBoard = 0
	mask := magic.Mask

	for true {
		moves := RayCast(loc, blockers, ray)
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

func RayCast(square Shift, blockers BitBoard, r Ray) BitBoard {
	coord := CoordsFromShift(square)
	row, col := int(coord.row), int(coord.col)
	var attacks BitBoard = 0

	for _, direction := range r {
		dRow, dCol := direction[0], direction[1]
		if dRow == 0 && dCol == 0 {
			continue
		}
		rayRow, rayCol := row+dRow, col+dCol
		for rayRow >= 0 && rayRow < ROW_COL_SIZE && rayCol >= 0 && rayCol < ROW_COL_SIZE {
			loc := BitBoard(1) << ShiftFromCoords(Coordinates{row: uint64(rayRow), col: uint64(rayCol)})
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
	BuildRookAttacks()
	BuildBishopAttacks()
}

// BuildRookAttacks fills ROOK_MAGIC and rookAttacks for every square.
func BuildRookAttacks() {
	ROOK_MAGIC, rookAttacks = buildSliderAttacks(ROOK_RAY, "data/rook_magics.csv")
}

// BuildBishopAttacks fills BISHOP_MAGIC and BISHOP_ATTACKS for every square.
func BuildBishopAttacks() {
	BISHOP_MAGIC, BISHOP_ATTACKS = buildSliderAttacks(BISHOP_RAY, "data/bishop_magics.csv")
}

// buildSliderAttacks generates magic numbers and attack tables for every
// square. By default the magics are restored from the given CSV; when that
// file is missing or invalid the builder falls back to a random search.
func buildSliderAttacks(ray Ray, fileName string) ([]MagicEntry, [][]BitBoard) {
	if LoadMagicsFromDisk {
		magics, err := tryLoadMagics(fileName)
		if err == nil {
			tables, buildErr := buildMagicTables(magics, ray)
			if buildErr == nil {
				return magics, tables
			}
			err = buildErr
		}
		log.Printf("could not use magics from %s (%v); searching for new magics", fileName, err)
	}

	magics := make([]MagicEntry, SHIFT_SIZE)
	tables := make([][]BitBoard, SHIFT_SIZE)

	for shift := Shift(0); shift < SHIFT_SIZE; shift++ {
		entry, table, err := FindMagic(CoordsFromShift(shift), ray)
		if err != nil {
			log.Fatalf("could not find magic for square %d: %v", shift, err)
		}
		magics[shift] = entry
		tables[shift] = table
	}

	if SaveMagicsToDisk {
		SaveMagics(fileName, magics)
	}
	return magics, tables
}

// buildMagicTables rebuilds the attack tables for a set of magics, failing if
// any stored magic does not produce a collision-free table.
func buildMagicTables(magics []MagicEntry, ray Ray) ([][]BitBoard, error) {
	tables := make([][]BitBoard, SHIFT_SIZE)
	for shift := Shift(0); shift < SHIFT_SIZE; shift++ {
		table, err := TryMagic(shift, magics[shift], ray)
		if err != nil {
			return nil, fmt.Errorf("square %d: %w", shift, err)
		}
		tables[shift] = table
	}
	return tables, nil
}

func BuildKnightAttacks() {
	fileName := "data/knight_attacks.csv"
	KNIGHT_ATTACKS = LoadAttacks(fileName)
}

func BuildKingAttacks() {
	fileName := "data/king_attacks.csv"
	KING_ATTACKS = LoadAttacks(fileName)
}

func BuildPawnMoves() {
	fileName := "data/white_pawn_move.csv"
	WHITE_PAWN_MOVES = LoadAttacks(fileName)
	fileName = "data/black_pawn_move.csv"
	BLACK_PAWN_MOVES = LoadAttacks(fileName)
}

func BuildPawnAttacks() {
	fileName := "data/white_pawn_attacks.csv"
	WHITE_PAWN_ATTACKS = LoadAttacks(fileName)
	fileName = "data/black_pawn_attacks.csv"
	BLACK_PAWN_ATTACKS = LoadAttacks(fileName)

}
