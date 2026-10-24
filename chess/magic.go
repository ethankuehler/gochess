package chess

import (
	"errors"
	"fmt"
	"log"
	"math/bits"
	"math/rand/v2"
)

// magicAttemptLimit bounds the random search for one square's magic.
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

// knightAttacks and kingAttacks are indexed by square.
var knightAttacks []BitBoard
var kingAttacks []BitBoard

// Pawn attack and move tables are split by colour.
var (
	whitePawnAttacks []BitBoard
	whitePawnMoves   []BitBoard
	blackPawnAttacks []BitBoard
	blackPawnMoves   []BitBoard
)

// Sliding-piece magic entries and attack tables.
var (
	rookMagicEntries   []MagicEntry
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
	return MagicEntry{
		Mask:      mask,
		Magic:     magic,
		IndexBits: uint(bits.OnesCount64(uint64(mask))),
	}
}

// Directions is a set of movement vectors as {rank delta, file delta}.
type Directions [4][2]int

var (
	rookDirections   = Directions{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	bishopDirections = Directions{{1, 1}, {-1, -1}, {1, -1}, {-1, 1}}
)

// magicIndex hashes the relevant blockers into an attack-table index.
func magicIndex(entry MagicEntry, occupancy BitBoard) uint64 {
	blockers := occupancy & entry.Mask
	hash := uint64(blockers) * entry.Magic
	return hash >> (64 - entry.IndexBits)
}

// RookAttack returns the rook attacks from square given the occupancy.
func RookAttack(square Square, occupancy BitBoard) BitBoard {
	index := magicIndex(rookMagicEntries[square], occupancy)
	return rookAttackTables[square][index]
}

// BishopAttack returns the bishop attacks from square given the occupancy.
func BishopAttack(square Square, occupancy BitBoard) BitBoard {
	index := magicIndex(bishopMagicEntries[square], occupancy)
	return bishopAttackTables[square][index]
}

// QueenAttack combines the rook and bishop lookups; queens need no
// tables of their own.
func QueenAttack(square Square, occupancy BitBoard) BitBoard {
	return RookAttack(square, occupancy) | BishopAttack(square, occupancy)
}

// RelevantBlockers returns the occupancy mask for a sliding piece on fr:
// the squares along its rays whose occupancy can change the attack set.
// The terminal edge square of each ray is excluded (a ray attacks its
// terminal square whether it is occupied or not, so that bit carries no
// information), as is the origin square.
func RelevantBlockers(fr FileRank, directions Directions) BitBoard {
	var mask BitBoard
	rank, file := int(fr.rank), int(fr.file)

	for _, direction := range directions {
		dRank, dFile := direction[0], direction[1]
		if dRank == 0 && dFile == 0 {
			continue
		}
		rayRank, rayFile := rank, file
		for {
			nextRank, nextFile := rayRank+dRank, rayFile+dFile
			if nextRank < 0 || nextRank >= boardSize || nextFile < 0 || nextFile >= boardSize {
				break
			}
			mask |= BitBoard(1) << SquareFromFileRank(FileRank{rank: uint64(rayRank), file: uint64(rayFile)})
			rayRank, rayFile = nextRank, nextFile
		}
	}

	return mask &^ (BitBoard(1) << SquareFromFileRank(fr))
}

// findMagic searches for a collision-free magic entry and attack table.
func findMagic(fr FileRank, directions Directions) (MagicEntry, []BitBoard, error) {
	mask := RelevantBlockers(fr, directions)
	square := SquareFromFileRank(fr)
	for range magicAttemptLimit {
		candidate := rand.Uint64() & rand.Uint64() & rand.Uint64()
		entry := newMagicEntry(mask, candidate)
		table, err := tryMagic(square, entry, directions)
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
func tryMagic(square Square, entry MagicEntry, directions Directions) ([]BitBoard, error) {
	table := make([]BitBoard, 1<<entry.IndexBits)
	var blockers BitBoard
	mask := entry.Mask

	for {
		attacks := SlidingAttacks(square, blockers, directions)
		tableEntry := &table[magicIndex(entry, blockers)]
		if *tableEntry == 0 {
			*tableEntry = attacks
		} else if *tableEntry != attacks {
			return nil, errors.New("invalid magic")
		}

		blockers = (blockers - mask) & mask
		if blockers == 0 {
			break
		}
	}

	return table, nil
}

// SlidingAttacks returns the attacks from square for the given occupancy
// along directions.
func SlidingAttacks(square Square, occupancy BitBoard, directions Directions) BitBoard {
	fr := FileRankFromSquare(square)
	rank, file := int(fr.rank), int(fr.file)
	var attacks BitBoard

	for _, direction := range directions {
		dRank, dFile := direction[0], direction[1]
		if dRank == 0 && dFile == 0 {
			continue
		}
		rayRank, rayFile := rank+dRank, file+dFile
		for rayRank >= 0 && rayRank < boardSize && rayFile >= 0 && rayFile < boardSize {
			squareBit := BitBoard(1) << SquareFromFileRank(FileRank{rank: uint64(rayRank), file: uint64(rayFile)})
			attacks |= squareBit
			if occupancy&squareBit > 0 {
				break
			}
			rayRank += dRank
			rayFile += dFile
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
		magics, err := LoadMagicEntries(fileName)
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

	for square := Square(0); square < squareCount; square++ {
		entry, table, err := findMagic(FileRankFromSquare(square), directions)
		if err != nil {
			log.Fatalf("could not find magic for square %d: %v", square, err)
		}
		magics[square] = entry
		tables[square] = table
	}

	if SaveMagicsToDisk {
		if err := SaveMagicEntries(fileName, magics); err != nil {
			log.Fatalf("could not save magics to %s: %v", fileName, err)
		}
	}
	return magics, tables
}

// buildMagicTables rebuilds the attack tables for a set of magics, failing if
// any stored magic does not produce a collision-free table.
func buildMagicTables(magics []MagicEntry, directions Directions) ([][]BitBoard, error) {
	tables := make([][]BitBoard, squareCount)
	for square := Square(0); square < squareCount; square++ {
		table, err := tryMagic(square, magics[square], directions)
		if err != nil {
			return nil, fmt.Errorf("square %d: %w", square, err)
		}
		tables[square] = table
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
