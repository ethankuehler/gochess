package chess

import (
	"errors"
	"fmt"
	"log"
	"math/bits"
	"math/rand/v2"
)

// Magic limit
const magicAttemptLimit = 1_000_000

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
var knightAttacks []BitBoard
var kingAttacks []BitBoard

// pawns are split up into attacks and move's
// Black and white pecies are split up due to the fact that they are different for pawns.
var (
	whitePawnAttacks []BitBoard
	whitePawnMoves   []BitBoard
	blackPawnAttacks []BitBoard
	blackPawnMoves   []BitBoard
)

// sliding piececs
var (
	rookMagicEntries   []MagicEntry //magic numbers
	bishopMagicEntries []MagicEntry
	rookAttackTables   [][]BitBoard
	bishopAttackTables [][]BitBoard
)

// MagicEntry holds a magic-bitboard lookup for one square: Mask is the
// relevant occupancy squares, Magic the multiplier, and IndexBits the number
// of set bits in Mask. The attack table holds 1 << IndexBits entries.
type MagicEntry struct {
	Mask      BitBoard
	Magic     uint64
	IndexBits uint
}

// newMagicEntry computes IndexBits from Mask so the two can never disagree.
func newMagicEntry(mask BitBoard, magic uint64) MagicEntry {
	return MagicEntry{mask, magic, uint(bits.OnesCount64(uint64(mask)))}
}

// array of vector that tell in which directions for the Directions caster to cast
type Directions [4][2]int

var (
	rookDirections   = Directions{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	bishopDirections = Directions{{1, 1}, {-1, -1}, {1, -1}, {-1, 1}}
)

func magicIndex(entry MagicEntry, board BitBoard) uint64 {
	blockers := board & entry.Mask
	hash := uint64(blockers) * entry.Magic
	index := hash >> (64 - entry.IndexBits)
	return index
}

func RookAttack(loc Square, board BitBoard) BitBoard {
	magic := rookMagicEntries[loc]
	idx := magicIndex(magic, board)
	return rookAttackTables[loc][idx]
}

func BishopAttack(loc Square, board BitBoard) BitBoard {
	magic := bishopMagicEntries[loc]
	idx := magicIndex(magic, board)
	return bishopAttackTables[loc][idx]
}

// QueenAttack combines the rook and bishop lookups; queens need no
// tables of their own.
func QueenAttack(loc Square, board BitBoard) BitBoard {
	return RookAttack(loc, board) | BishopAttack(loc, board)
}

// RelevantBlockers returns the occupancy mask for a sliding piece on coord:
// the squares along its rays whose occupancy can change the attack set.
// The terminal edge square of each directions is excluded (a directions attacks its
// terminal square whether it is occupied or not, so that bit carries no
// information), as is the origin square.
func RelevantBlockers(coord FileRank, directions Directions) BitBoard {
	var mask BitBoard = 0
	row, col := int(coord.row), int(coord.col)

	for _, direction := range directions {
		dRow, dCol := direction[0], direction[1]
		if dRow == 0 && dCol == 0 {
			continue
		}
		rayRow, rayCol := row, col
		for {
			nextRow, nextCol := rayRow+dRow, rayCol+dCol
			if nextRow < 0 || nextRow >= boardSize || nextCol < 0 || nextCol >= boardSize {
				break
			}
			mask |= BitBoard(1) << SquareFromFileRank(FileRank{row: uint64(rayRow), col: uint64(rayCol)})
			rayRow, rayCol = nextRow, nextCol
		}
	}

	return mask &^ (BitBoard(1) << SquareFromFileRank(coord))
}

func GetBishopMask(coord FileRank) BitBoard {
	return SlidingAttacks(SquareFromFileRank(coord), 0, bishopDirections)
}

func findMagic(coord FileRank, directions Directions) (MagicEntry, []BitBoard, error) {
	mask := RelevantBlockers(coord, directions)
	shift := SquareFromFileRank(coord)
	for _ = range magicAttemptLimit {
		candidate := rand.Uint64() & rand.Uint64() & rand.Uint64()
		entry := newMagicEntry(mask, candidate)
		table, err := tryMagic(shift, entry, directions)
		if err != nil {
			continue
		}
		return entry, table, nil
	}
	return MagicEntry{}, nil, errors.New("hit magic limit, magic not found")

}

// tryMagic attempts to fill an attack table for one square using the
// candidate magic, failing on any non-constructive collision (two blocker
// configurations with different attack sets hashing to the same slot).
// Attack sets are never empty, so a zero slot means "not yet written".
func tryMagic(loc Square, magic MagicEntry, directions Directions) ([]BitBoard, error) {
	table := make([]BitBoard, 1<<magic.IndexBits)
	var blockers BitBoard = 0
	mask := magic.Mask

	for true {
		moves := SlidingAttacks(loc, blockers, directions)
		table_entry := &table[magicIndex(magic, blockers)]
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

func SlidingAttacks(square Square, blockers BitBoard, r Directions) BitBoard {
	coord := FileRankFromSquare(square)
	row, col := int(coord.row), int(coord.col)
	var attacks BitBoard = 0

	for _, direction := range r {
		dRow, dCol := direction[0], direction[1]
		if dRow == 0 && dCol == 0 {
			continue
		}
		rayRow, rayCol := row+dRow, col+dCol
		for rayRow >= 0 && rayRow < boardSize && rayCol >= 0 && rayCol < boardSize {
			loc := BitBoard(1) << SquareFromFileRank(FileRank{row: uint64(rayRow), col: uint64(rayCol)})
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

// BuildRookAttacks fills rookMagicEntries and rookAttackTables for every square.
func BuildRookAttacks() {
	rookMagicEntries, rookAttackTables = buildSliderAttacks(rookDirections, "data/rook_magics.csv")
}

// BuildBishopAttacks fills bishopMagicEntries and bishopAttackTables for every square.
func BuildBishopAttacks() {
	bishopMagicEntries, bishopAttackTables = buildSliderAttacks(bishopDirections, "data/bishop_magics.csv")
}

// buildSliderAttacks generates magic numbers and attack tables for every
// square. By default the magics are restored from the given CSV; when that
// file is missing or invalid the builder falls back to a random search.
func buildSliderAttacks(directions Directions, fileName string) ([]MagicEntry, [][]BitBoard) {
	if LoadMagicsFromDisk {
		magics, err := tryLoadMagics(fileName)
		if err == nil {
			tables, buildErr := buildMagicTables(magics, directions)
			if buildErr == nil {
				return magics, tables
			}
			err = buildErr
		}
		log.Printf("could not use magics from %s (%v); searching for new magics", fileName, err)
	}

	magics := make([]MagicEntry, squareCount)
	tables := make([][]BitBoard, squareCount)

	for shift := Square(0); shift < squareCount; shift++ {
		entry, table, err := findMagic(FileRankFromSquare(shift), directions)
		if err != nil {
			log.Fatalf("could not find magic for square %d: %v", shift, err)
		}
		magics[shift] = entry
		tables[shift] = table
	}

	if SaveMagicsToDisk {
		SaveMagicEntries(fileName, magics)
	}
	return magics, tables
}

// buildMagicTables rebuilds the attack tables for a set of magics, failing if
// any stored magic does not produce a collision-free table.
func buildMagicTables(magics []MagicEntry, directions Directions) ([][]BitBoard, error) {
	tables := make([][]BitBoard, squareCount)
	for shift := Square(0); shift < squareCount; shift++ {
		table, err := tryMagic(shift, magics[shift], directions)
		if err != nil {
			return nil, fmt.Errorf("square %d: %w", shift, err)
		}
		tables[shift] = table
	}
	return tables, nil
}

func BuildKnightAttacks() {
	fileName := "data/knight_attacks.csv"
	knightAttacks = loadAttackTable(fileName)
}

func BuildKingAttacks() {
	fileName := "data/king_attacks.csv"
	kingAttacks = loadAttackTable(fileName)
}

func BuildPawnMoves() {
	fileName := "data/white_pawn_move.csv"
	whitePawnMoves = loadAttackTable(fileName)
	fileName = "data/black_pawn_move.csv"
	blackPawnMoves = loadAttackTable(fileName)
}

func BuildPawnAttacks() {
	fileName := "data/white_pawn_attacks.csv"
	whitePawnAttacks = loadAttackTable(fileName)
	fileName = "data/black_pawn_attacks.csv"
	blackPawnAttacks = loadAttackTable(fileName)

}
