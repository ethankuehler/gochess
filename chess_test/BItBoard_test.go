package chess

import (
	"encoding/csv"
	"log"
	"os"
	"testing"

	"github.com/ethankuehler/gochess/chess"
)

func TestNewBaordFEN(t *testing.T) {
	// Open the CSV file
	file, err := os.Open("../test_data/FEN.csv")
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
		var b, err = chess.NewBoardFEN(record[0])
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
