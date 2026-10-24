package chess

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strconv"
)

// loadAttackTableFS loads one 64-entry attack table from a CSV in fsys with
// columns square, start square bit, and attack bitboard.
func loadAttackTableFS(fsys fs.FS, fileName string) ([]BitBoard, error) {
	if fsys == nil {
		return nil, fmt.Errorf("nil filesystem")
	}
	data, err := readCSVFS(fsys, fileName)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", fileName, err)
	}
	return parseAttackTable(data, fileName)
}

// loadAttackTable loads one attack table from disk and exits the process on
// failure. It is the historical helper used by the package-level builders.
func loadAttackTable(fileName string) []BitBoard {
	target, err := loadAttackTableFile(fileName)
	if err != nil {
		log.Fatal(err)
	}
	return target
}

// loadAttackTableFile is loadAttackTable without the fatal exit.
func loadAttackTableFile(fileName string) ([]BitBoard, error) {
	data, err := readCSV(fileName)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", fileName, err)
	}
	return parseAttackTable(data, fileName)
}

// parseAttackTable validates the CSV records and returns the table.
func parseAttackTable(data [][]string, fileName string) ([]BitBoard, error) {
	if len(data) != squareCount+1 {
		return nil, fmt.Errorf("%s: expected %d data rows, got %d", fileName, squareCount, len(data))
	}

	target := make([]BitBoard, squareCount)
	seen := make(map[uint64]bool, squareCount)
	for _, record := range data[1:] {
		values, err := readRecord(record)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fileName, err)
		}
		if len(values) != 3 {
			return nil, fmt.Errorf("%s: expected 3 columns", fileName)
		}
		if values[0] >= squareCount {
			return nil, fmt.Errorf("%s: square %d out of range", fileName, values[0])
		}
		if seen[values[0]] {
			return nil, fmt.Errorf("%s: duplicate square %d", fileName, values[0])
		}
		seen[values[0]] = true
		target[values[0]] = BitBoard(values[2])
	}
	return target, nil
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

// readCSVFS parses a CSV file from an fs.FS.
func readCSVFS(fsys fs.FS, fileName string) ([][]string, error) {
	file, err := fsys.Open(fileName)
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
