package chess

import (
	"encoding/csv"
	"log"
	"os"
	"strconv"
)

// there are only 64 knight moves on a chess board
// each key is the location of the knight, the value is the attack
var KNIGHT_ATTACKS map[uint64]uint64
var KING_ATTACKS map[uint64]uint64

// pawns are split up into attacks and move's
// Black and white pecies are split up due to the fact that they are different for pawns.
var (
	WHITE_PAWN_ATTACKS map[uint64]uint64
	WHITE_PAWN_MOVES   map[uint64]uint64
	BLACK_PAWN_ATTACKS map[uint64]uint64
	BLACK_PAWN_MOVES   map[uint64]uint64
)

// sliding piececs
var (
	ROOK_MAGIC     []MagicEntry
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

func GetRookAttack(location uint64, blockers uint64) uint64 {
	//magic := ROOK_MAGIC[location]
	//attacks := ROOK_ATTTACKS[location]
	return 0 // TODO: finish
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
	WHITE_PAWN_MOVES = make(map[uint64]uint64)
	BLACK_PAWN_MOVES = make(map[uint64]uint64)
	//White pawns on the 2nd rank move twice.
	for i := range ShiftIter("a2", "h2") {
		loc := uint64(1) << i
		mask := WHITE_PAWN_MOVE_MASK_2 << (i - WHITE_PAWN_MOVE_OFFSET_2)
		WHITE_PAWN_MOVES[loc] = mask
	}

	//Black pawns on the 7th also move twice.
	for i := range ShiftIter("a7", "h7") {
		loc := uint64(1) << i
		mask := BLACK_PAWN_MOVE_MASK_2 << (i - BLACK_PAWN_MOVE_OFFSET_2)
		BLACK_PAWN_MOVES[loc] = mask
	}

	//white pawns
	for i := range ShiftIter("a3", "h7") {
		loc := uint64(1) << i
		mask := WHITE_PAWN_MOVE_MASK << (i - WHITE_PAWN_MOVE_OFFSET)
		WHITE_PAWN_MOVES[loc] = mask
	}

	//black pawns
	for i := range ShiftIter("a2", "h6") {
		loc := uint64(1) << i
		mask := BLACK_PAWN_MOVE_MASK << (i - BLACK_PAWN_MOVE_OFFSET)
		BLACK_PAWN_MOVES[loc] = mask
	}
}

func BuildPawnAttacks() {
	WHITE_PAWN_ATTACKS = make(map[uint64]uint64)
	BLACK_PAWN_ATTACKS = make(map[uint64]uint64)

	for i := range ShiftIter("a2", "h7") {
		loc := uint64(1 << i)
		mask := WHITE_PAWN_ATTACK_MASK << (i - WHITE_PAWN_ATTACK_OFFSET)
		WHITE_PAWN_ATTACKS[loc] = mask
	}

	for i := range ShiftIter("a2", "h7") {
		loc := uint64(1 << i)
		mask := BLACK_PAWN_ATTACK_MASK << (i - BLACK_PAWN_ATTACK_OFFSET)
		WHITE_PAWN_ATTACKS[loc] = mask
	}
}

func LoadAttacks(csv_file_name string) map[uint64]uint64 {
	target_map := make(map[uint64]uint64)
	data, err := readCsv(csv_file_name)
	if err != nil {
		log.Fatalf("was not able to read file, %v", err)
	}
	for _, record := range data[1:] {
		start, attack, err := readRecord(record)
		if err != nil {
			log.Fatalf("Error in data, %v", err)
		}
		target_map[start] = attack
	}
	return target_map
}

func readCsv(filename string) ([][]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// Read all the records from the CSV
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	return records, nil
}

func readRecord(record []string) (uint64, uint64, error) {
	loc, err := strconv.ParseUint(record[1], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	mask, err := strconv.ParseUint(record[2], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return uint64(loc), uint64(mask), nil
}
