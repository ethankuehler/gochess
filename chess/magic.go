package chess

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math/bits"
	"math/rand/v2"
)

const magicAttemptLimit = 1_000_000

var LoadMagicsFromDisk = true
var SaveMagicsToDisk = false

var knightAttacks []BitBoard
var kingAttacks []BitBoard

var (
	whitePawnAttacks []BitBoard
	whitePawnMoves   []BitBoard
	blackPawnAttacks []BitBoard
	blackPawnMoves   []BitBoard
)

var (
	rookMagicEntries   []MagicEntry
	bishopMagicEntries []MagicEntry
	rookAttackTables   [][]BitBoard
	bishopAttackTables [][]BitBoard
)

type MagicEntry struct {
	Mask      BitBoard
	Magic     uint64
	IndexBits uint
}

func newMagicEntry(mask BitBoard, magic uint64) MagicEntry {
	return MagicEntry{
		Mask:      mask,
		Magic:     magic,
		IndexBits: uint(bits.OnesCount64(uint64(mask))),
	}
}

// Directions contains {rank delta, file delta} vectors.
type Directions [4][2]int

var (
	rookDirections   = Directions{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	bishopDirections = Directions{{1, 1}, {-1, -1}, {1, -1}, {-1, 1}}
)

func magicIndex(entry MagicEntry, occupancy BitBoard) uint64 {
	blockers := occupancy & entry.Mask
	hash := uint64(blockers) * entry.Magic
	return hash >> (64 - entry.IndexBits)
}

func RookAttack(square Square, occupancy BitBoard) BitBoard {
	index := magicIndex(rookMagicEntries[square], occupancy)
	return rookAttackTables[square][index]
}

func BishopAttack(square Square, occupancy BitBoard) BitBoard {
	index := magicIndex(bishopMagicEntries[square], occupancy)
	return bishopAttackTables[square][index]
}

func QueenAttack(square Square, occupancy BitBoard) BitBoard {
	return RookAttack(square, occupancy) | BishopAttack(square, occupancy)
}

// RelevantBlockers excludes ray edges and the origin.
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

// Attack sets are never empty, so zero marks an unwritten table slot.
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

// BuildAllAttacksFromFS leaves existing tables untouched on failure.
func BuildAllAttacksFromFS(fsys fs.FS) error {
	knight, err := loadAttackTableFS(fsys, "knight_attacks.csv")
	if err != nil {
		return err
	}
	king, err := loadAttackTableFS(fsys, "king_attacks.csv")
	if err != nil {
		return err
	}
	whitePawnMovesTable, err := loadAttackTableFS(fsys, "white_pawn_move.csv")
	if err != nil {
		return err
	}
	blackPawnMovesTable, err := loadAttackTableFS(fsys, "black_pawn_move.csv")
	if err != nil {
		return err
	}
	whitePawnAttacksTable, err := loadAttackTableFS(fsys, "white_pawn_attacks.csv")
	if err != nil {
		return err
	}
	blackPawnAttacksTable, err := loadAttackTableFS(fsys, "black_pawn_attacks.csv")
	if err != nil {
		return err
	}
	rookMagics, rookTables, err := buildSliderAttacksFS(fsys, "rook_magics.csv", rookDirections)
	if err != nil {
		return err
	}
	bishopMagics, bishopTables, err := buildSliderAttacksFS(fsys, "bishop_magics.csv", bishopDirections)
	if err != nil {
		return err
	}

	knightAttacks = knight
	kingAttacks = king
	whitePawnMoves = whitePawnMovesTable
	blackPawnMoves = blackPawnMovesTable
	whitePawnAttacks = whitePawnAttacksTable
	blackPawnAttacks = blackPawnAttacksTable
	rookMagicEntries = rookMagics
	rookAttackTables = rookTables
	bishopMagicEntries = bishopMagics
	bishopAttackTables = bishopTables
	return nil
}

// buildSliderAttacksFS loads magics from fsys and rebuilds their attack tables.
func buildSliderAttacksFS(fsys fs.FS, fileName string, directions Directions) ([]MagicEntry, [][]BitBoard, error) {
	magics, err := LoadMagicEntriesFS(fsys, fileName)
	if err != nil {
		return nil, nil, err
	}
	tables, err := buildMagicTables(magics, directions)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", fileName, err)
	}
	return magics, tables, nil
}

func BuildRookAttacks() {
	rookMagicEntries, rookAttackTables = buildSliderAttacks(rookDirections, "data/rook_magics.csv")
}

func BuildBishopAttacks() {
	bishopMagicEntries, bishopAttackTables = buildSliderAttacks(bishopDirections, "data/bishop_magics.csv")
}

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
