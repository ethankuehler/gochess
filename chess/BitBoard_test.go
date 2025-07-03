package chess

import (
	"encoding/csv"
	"log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Change working directory to project root
	os.Chdir("..")
	os.Exit(m.Run())
}

func TestNewBaordFEN(t *testing.T) {
	// Open the CSV file
	file, err := os.Open("data/FEN.csv")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// Read all the records from the CSV
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV file: %v", err)
	}

	// Loop through and print each record
	for _, record := range records {
		var b, err = NewBoardFEN(record[0])
		if err != nil {
			t.Errorf("ERROR: could not decode FEN %s", record[0])
			continue
		}
		out := b.FEN()
		if out != record[0] {
			t.Errorf("FEN dose not match, input = %s, output = %s", record[0], out)
		}
	}

}

func TestOccupied(t *testing.T) {
	// Open the CSV file
	file, err := os.Open("data/FEN.csv")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// Read all the records from the CSV
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV file: %v", err)
	}

	// Loop through and print each record
	for _, record := range records {
		var b, err = NewBoardFEN(record[0])
		if err != nil {
			t.Errorf("ERROR: could not decode FEN %s", record[0])
			continue
		}
		occupied_white := b.Occupied(WHITE)
		var comp_white uint64
		comp_white = 0
		for i := PAWN; i <= KING; i++ {
			comp_white |= b.GetPieces(WHITE, Piece(i))
		}
		if occupied_white != comp_white {
			t.Errorf("ERROR: WHITE Occupied %b, comp %b", occupied_white, comp_white)
		}

		occupied_black := b.Occupied(BLACK)
		var comp_black uint64
		comp_black = 0
		for i := PAWN; i <= KING; i++ {
			comp_black |= b.GetPieces(BLACK, Piece(i))
		}
		if occupied_black != comp_black {
			t.Errorf("ERROR: BLACK Occupied %b, comp %b", occupied_black, comp_black)
		}

		occupied_both := b.Occupied(BOTH)
		if occupied_both != (comp_white | comp_black) {
			t.Errorf("ERROR: BOTH Occupied %b, comp %b", occupied_black, comp_black)
		}

	}

}
