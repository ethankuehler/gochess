package chess

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"strconv"
)

func LoadMagicEntries(fileName string) ([]MagicEntry, error) {
	data, err := readCSV(fileName)
	if err != nil {
		return nil, err
	}
	return parseMagicEntries(data, fileName)
}

func LoadMagicEntriesFS(fsys fs.FS, fileName string) ([]MagicEntry, error) {
	if fsys == nil {
		return nil, fmt.Errorf("nil filesystem")
	}
	data, err := readCSVFS(fsys, fileName)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", fileName, err)
	}
	return parseMagicEntries(data, fileName)
}

func parseMagicEntries(data [][]string, fileName string) ([]MagicEntry, error) {
	if len(data) != squareCount+1 {
		return nil, fmt.Errorf("%s: expected %d data rows, got %d", fileName, squareCount, len(data))
	}

	target := make([]MagicEntry, squareCount)
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
