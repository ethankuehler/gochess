package chess

import (
	"encoding/csv"
	"log"
	"os"
	"strconv"
)

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

// LoadMagics reads a CSV of magic entries (square, mask, magic), one row
// per square. IndexBits is recomputed from the mask.
func LoadMagics(csv_file_name string) []MagicEntry {
	target := make([]MagicEntry, SHIFT_SIZE)
	data, err := readCSV(csv_file_name)
	if err != nil {
		log.Fatalf("Unable to read file: %s", err.Error())
	}

	if len(data) != SHIFT_SIZE+1 {
		log.Fatalf("Error, expected %d data rows, filename: %s", SHIFT_SIZE, csv_file_name)
	}

	for _, record := range data[1:] {
		val, err := readRecord(record)
		if err != nil {
			log.Fatalf("Error in data: %s", err.Error())
		}
		if len(val) != 3 {
			log.Fatalf("Error, data didnt have enough rows, filename: %s", csv_file_name)
		}
		target[val[0]] = NewMagicEntry(BitBoard(val[1]), val[2])
	}
	return target
}

// SaveMagics writes magic entries to a CSV (square, mask, magic), one row
// per square, in the same format LoadMagics reads.
func SaveMagics(csv_file_name string, entries []MagicEntry) {
	if len(entries) != SHIFT_SIZE {
		log.Fatalf("Error, expected %d magic entries, got %d", SHIFT_SIZE, len(entries))
	}

	file, err := os.Create(csv_file_name)
	if err != nil {
		log.Fatalf("Unable to create file: %s", err.Error())
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"square", "mask", "magic"})
	for i, entry := range entries {
		writer.Write([]string{
			strconv.Itoa(i),
			strconv.FormatUint(uint64(entry.Mask), 10),
			strconv.FormatUint(entry.Magic, 10),
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Fatalf("Unable to write file: %s", err.Error())
	}
}
