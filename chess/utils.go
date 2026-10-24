package chess

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
)

func loadAttackTable(fileName string) []BitBoard {
	target := make([]BitBoard, squareCount)
	data, err := readCSV(fileName)
	if err != nil {
		log.Fatalf("Unable to read file: %s", err.Error())
	}

	for _, record := range data[1:] {
		val, err := readRecord(record)
		if err != nil {
			log.Fatalf("Error in data: %s", err.Error())
		}
		if len(val) != 3 {
			log.Fatalf("Error, data did not have enough rows, fileName: %s", fileName)
		}
		target[val[0]] = BitBoard(val[2])
	}
	return target
}

func readCSV(fileName string) ([][]string, error) {
	file, err := os.Open(fileName)
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
	for i, field := range record {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return nil, err
		}
		output[i] = uint64(value)
	}
	return output, nil
}

// LoadMagicEntries reads and validates a CSV of magic entries
// (square, mask, magic), one row per square. IndexBits is recomputed from
// the mask so it always agrees with the data.
func LoadMagicEntries(fileName string) ([]MagicEntry, error) {
	target := make([]MagicEntry, squareCount)
	data, err := readCSV(fileName)
	if err != nil {
		return nil, err
	}

	if len(data) != squareCount+1 {
		return nil, fmt.Errorf("expected %d data rows, fileName: %s", squareCount, fileName)
	}

	seen := make(map[uint64]bool, squareCount)
	for _, record := range data[1:] {
		val, err := readRecord(record)
		if err != nil {
			return nil, fmt.Errorf("error in data: %w", err)
		}
		if len(val) != 3 {
			return nil, fmt.Errorf("expected 3 columns, fileName: %s", fileName)
		}
		if val[0] >= squareCount {
			return nil, fmt.Errorf("square %d out of range, fileName: %s", val[0], fileName)
		}
		if seen[val[0]] {
			return nil, fmt.Errorf("duplicate square %d, fileName: %s", val[0], fileName)
		}
		seen[val[0]] = true
		target[val[0]] = newMagicEntry(BitBoard(val[1]), val[2])
	}
	return target, nil
}

// SaveMagicEntries writes magic entries to a CSV (square, mask, magic), one
// row per square, in the same format LoadMagicEntries reads.
func SaveMagicEntries(fileName string, entries []MagicEntry) error {
	if len(entries) != squareCount {
		return fmt.Errorf("expected %d magic entries, got %d", squareCount, len(entries))
	}

	file, err := os.Create(fileName)
	if err != nil {
		return err
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
	return writer.Error()
}
