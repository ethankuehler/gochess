package chess

import (
	"encoding/csv"
	"os"
	"strconv"
	"testing"

	"github.com/ethankuehler/gochess/chess"
)

func TestPawnAttacks(t *testing.T) {
	chess.BuildPawnAttacks()
	if chess.WHITE_PAWN_ATTACKS == nil {
		t.Error("nil map")
	}

	if len(chess.WHITE_PAWN_ATTACKS) != 48 {
		t.Errorf("map incorrect size, 64 != %d", len(chess.WHITE_PAWN_ATTACKS))
	}

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

func convertRecord(record []string) (uint64, uint64, error) {
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

func TestPawnMoves(t *testing.T) {
	chess.BuildPawnMoves()
	if chess.WHITE_PAWN_MOVES == nil {
		t.Error("White nil map")
	}

	if len(chess.WHITE_PAWN_MOVES) != 48 {
		t.Errorf("White map incorrect size, 64 != %d", len(chess.WHITE_PAWN_ATTACKS))
	}

	if chess.BLACK_PAWN_MOVES == nil {
		t.Error("Black nil map")
	}

	if len(chess.BLACK_PAWN_MOVES) != 48 {
		t.Errorf("Black map incorrect size, 64 != %d", len(chess.WHITE_PAWN_ATTACKS))
	}

	records, err := readCsv("../test_data/white_pawn_move.csv")
	if err != nil {
		t.Fatalf("could not open csv %v", err)
	}

	for _, record := range records[1:] {
		loc, mask, err := convertRecord(record)
		if err != nil {
			t.Fatalf("could not convert record %v", err)
		}
		if chess.WHITE_PAWN_MOVES[uint64(loc)] != uint64(mask) {
			t.Errorf("white move did not match gen=%d actual=%d", chess.WHITE_PAWN_MOVES[uint64(loc)], mask)
		}
	}

	records, err = readCsv("../test_data/black_pawn_move.csv")
	if err != nil {
		t.Fatalf("could not open csv %v", err)
	}

	for _, record := range records[1:] {
		loc, mask, err := convertRecord(record)
		if err != nil {
			t.Fatalf("could not convert record %v", err)
		}
		if chess.BLACK_PAWN_MOVES[uint64(loc)] != uint64(mask) {
			t.Errorf("black move did not match gen=%d actual=%d", chess.BLACK_PAWN_MOVES[uint64(loc)], mask)
		}
	}

}
