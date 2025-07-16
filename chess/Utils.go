package chess

import (
	"encoding/csv"
	"log"
	"os"
	"strconv"
)

func LoadAttacks(csv_file_name string) []uint64 {
	target := make([]uint64, LOCATION_SIZE)
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
			log.Fatal("Error, data didnt have enough rows, filename: %s", csv_file_name)
		}
		target[val[0]] = val[2]
	}

	return target
}

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
