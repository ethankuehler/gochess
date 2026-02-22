package chess

import (
	"encoding/csv"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
)

// GenerateRookMagics generates magic numbers for all 64 squares for rooks
func GenerateRookMagics() []MagicEntry {
	magics := make([]MagicEntry, 64)

	for square := Shift(0); square < 64; square++ {
		coord := CoordsFromShift(square)
		mask := GetRookMask(coord)

		// Count bits in mask to determine index size
		bitCount := Shift(countBits(uint64(mask)))

		// Find a valid magic number
		fmt.Printf("Finding magic for rook square %d...\n", square)
		magicNumber := findRookMagic(square, mask, bitCount)

		magics[square] = MagicEntry{
			Mask:  mask,
			Magic: magicNumber,
			Index: bitCount,
		}
		fmt.Printf("  Found: 0x%X (bits: %d)\n", magicNumber, bitCount)
	}

	return magics
}

// GenerateBishopMagics generates magic numbers for all 64 squares for bishops
func GenerateBishopMagics() []MagicEntry {
	magics := make([]MagicEntry, 64)

	for square := Shift(0); square < 64; square++ {
		coord := CoordsFromShift(square)
		mask := GetBishopMask(coord)

		// Count bits in mask to determine index size
		bitCount := Shift(countBits(uint64(mask)))

		// Find a valid magic number
		fmt.Printf("Finding magic for bishop square %d...\n", square)
		magicNumber := findBishopMagic(square, mask, bitCount)

		magics[square] = MagicEntry{
			Mask:  mask,
			Magic: magicNumber,
			Index: bitCount,
		}
		fmt.Printf("  Found: 0x%X (bits: %d)\n", magicNumber, bitCount)
	}

	return magics
}

// findRookMagic searches for a valid magic number for a rook at the given square
func findRookMagic(square Shift, mask BitBoard, indexBits Shift) uint64 {
	for {
		// Generate random magic candidate (sparse random number)
		testMagic := rand.Uint64() & rand.Uint64() & rand.Uint64()

		magicEntry := MagicEntry{mask, testMagic, indexBits}
		if tryRookMagicForGeneration(square, magicEntry) {
			return testMagic
		}
	}
}

// findBishopMagic searches for a valid magic number for a bishop at the given square
func findBishopMagic(square Shift, mask BitBoard, indexBits Shift) uint64 {
	for {
		// Generate random magic candidate (sparse random number)
		testMagic := rand.Uint64() & rand.Uint64() & rand.Uint64()

		magicEntry := MagicEntry{mask, testMagic, indexBits}
		if tryBishopMagicForGeneration(square, magicEntry) {
			return testMagic
		}
	}
}

// tryRookMagicForGeneration tests if a magic number works for rook attacks
func tryRookMagicForGeneration(loc Shift, magic MagicEntry) bool {
	tableSize := 1 << magic.Index
	table := make([]BitBoard, tableSize)
	used := make([]bool, tableSize)

	var blockers BitBoard = 0
	mask := magic.Mask

	for {
		attacks := RayCast(loc, blockers, mask, ROOK_RAY)
		index := MagicIndex(magic, blockers)

		if used[index] {
			if table[index] != attacks {
				return false
			}
		} else {
			table[index] = attacks
			used[index] = true
		}

		blockers = (blockers - mask) & mask
		if blockers == 0 {
			break
		}
	}

	return true
}

// tryBishopMagicForGeneration tests if a magic number works for bishop attacks
func tryBishopMagicForGeneration(loc Shift, magic MagicEntry) bool {
	tableSize := 1 << magic.Index
	table := make([]BitBoard, tableSize)
	used := make([]bool, tableSize)

	var blockers BitBoard = 0
	mask := magic.Mask

	for {
		attacks := RayCast(loc, blockers, mask, BISHOP_RAY)
		index := MagicIndex(magic, blockers)

		if used[index] {
			if table[index] != attacks {
				return false
			}
		} else {
			table[index] = attacks
			used[index] = true
		}

		blockers = (blockers - mask) & mask
		if blockers == 0 {
			break
		}
	}

	return true
}

// countBits counts the number of set bits in a uint64
func countBits(n uint64) int {
	count := 0
	for n > 0 {
		count++
		n &= n - 1
	}
	return count
}

// SaveRookMagicsToCSV saves rook magic numbers to a CSV file
func SaveRookMagicsToCSV(magics []MagicEntry, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"square", "mask", "magic", "index_bits"})

	// Write magic entries
	for i, magic := range magics {
		row := []string{
			strconv.Itoa(i),
			strconv.FormatUint(uint64(magic.Mask), 10),
			strconv.FormatUint(magic.Magic, 10),
			strconv.Itoa(int(magic.Index)),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// SaveBishopMagicsToCSV saves bishop magic numbers to a CSV file
func SaveBishopMagicsToCSV(magics []MagicEntry, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"square", "mask", "magic", "index_bits"})

	// Write magic entries
	for i, magic := range magics {
		row := []string{
			strconv.Itoa(i),
			strconv.FormatUint(uint64(magic.Mask), 10),
			strconv.FormatUint(magic.Magic, 10),
			strconv.Itoa(int(magic.Index)),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// LoadMagicsFromCSV loads magic numbers from a CSV file
func LoadMagicsFromCSV(filename string) ([]MagicEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	// Skip header
	records = records[1:]

	magics := make([]MagicEntry, 64)
	for _, record := range records {
		square, _ := strconv.Atoi(record[0])
		mask, _ := strconv.ParseUint(record[1], 10, 64)
		magic, _ := strconv.ParseUint(record[2], 10, 64)
		indexBits, _ := strconv.Atoi(record[3])

		magics[square] = MagicEntry{
			Mask:  BitBoard(mask),
			Magic: magic,
			Index: Shift(indexBits),
		}
	}

	return magics, nil
}
