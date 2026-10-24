package chess

import (
	"encoding/csv"
	"log"
	"os"
	"strconv"
)

// loadAttackTable loads one 64-entry attack table from a CSV with columns
// square, start square bit, and attack bitboard.
func loadAttackTable(fileName string) []BitBoard {
	target := make([]BitBoard, squareCount)
	data, err := readCSV(fileName)
	if err != nil {
		log.Fatalf("could not read %s: %v", fileName, err)
	}
	if len(data) != squareCount+1 {
		log.Fatalf("%s: expected %d data rows, got %d", fileName, squareCount, len(data))
	}

	seen := make(map[uint64]bool, squareCount)
	for _, record := range data[1:] {
		values, err := readRecord(record)
		if err != nil {
			log.Fatalf("%s: %v", fileName, err)
		}
		if len(values) != 3 {
			log.Fatalf("%s: expected 3 columns", fileName)
		}
		if values[0] >= squareCount {
			log.Fatalf("%s: square %d out of range", fileName, values[0])
		}
		if seen[values[0]] {
			log.Fatalf("%s: duplicate square %d", fileName, values[0])
		}
		seen[values[0]] = true
		target[values[0]] = BitBoard(values[2])
	}
	return target
}

// readCSV opens and parses a CSV file. Every record must have the same
// number of fields as the header.
func readCSV(fileName string) ([][]string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = 0

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	return records, nil
}

// readRecord parses every field in a CSV record as an unsigned integer.
func readRecord(record []string) ([]uint64, error) {
	values := make([]uint64, len(record))
	for i, field := range record {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}
