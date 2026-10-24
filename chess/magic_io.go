package chess

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

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
		return nil, fmt.Errorf("%s: expected %d data rows, got %d", fileName, squareCount, len(data))
	}

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
		target[values[0]] = newMagicEntry(BitBoard(values[1]), values[2])
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
		file.Close()
		return err
	}
	return file.Close()
}
