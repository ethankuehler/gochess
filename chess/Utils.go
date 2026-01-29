package chess

import (
	"encoding/csv"
	"log"
	"os"
	"strconv"
)

// LoadAttacks loads attack bitboards from a CSV file.
// The CSV file should have columns: index, [other data], bitboard_value
// Returns a slice of 64 BitBoards indexed by square position.
func LoadAttacks(csv_file_name string) []BitBoard {
	target := make([]BitBoard, SHIFT_SIZE)
	data, err := readCSV(csv_file_name)
	if err != nil {
		log.Fatalf("Unable to read file: %s", err.Error())
	}

	for _, record := range data[1:] {
		val, err := readRecord(record)
		if err != nil {
			log.Fatalf("Error in data: %s", err.Error())
		}
		if len(val) != 3 {
			log.Fatalf("Error, data didnt have enough rows, filename: %s", csv_file_name)
		}
		target[val[0]] = BitBoard(val[2])
	}
	return target
}

// readCSV reads a CSV file and returns all records as a 2D string slice.
func readCSV(filename string) ([][]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = 0 // Tells reader to throw error if # of fields per record changes.

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	return records, nil
}

// readRecord converts a CSV record (slice of strings) to a slice of uint64 values.
func readRecord(record []string) ([]uint64, error) {
	output := make([]uint64, len(record))
	for i, str := range record {
		conversion, err := strconv.ParseUint(str, 10, 64)
		if err != nil {
			return nil, err
		}
		output[i] = uint64(conversion)
	}
	return output, nil
}
