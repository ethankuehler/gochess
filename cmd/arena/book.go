package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/ethankuehler/gochess/chess"
)

// opening is one book line: a name and the UCI moves played from the start
// position before the engines take over.
type opening struct {
	Name  string
	Moves []string
}

// loadBook reads a CSV with a "name,moves" header. Each line is validated by
// replaying it, so a typo fails before any game is played. Lines that transpose
// to an already-seen position are dropped to avoid pseudo-replication.
func loadBook(path string) ([]opening, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open book: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read book: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("book is empty")
	}

	start := 0
	if strings.EqualFold(strings.TrimSpace(rows[0][0]), "name") {
		start = 1
	}

	var openings []opening
	seen := make(map[chess.PositionKey]string)
	for i := start; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 2 {
			return nil, fmt.Errorf("book row %d: want name,moves", i+1)
		}
		name := strings.TrimSpace(row[0])
		moves := strings.Fields(strings.TrimSpace(row[1]))
		if name == "" || len(moves) == 0 {
			return nil, fmt.Errorf("book row %d: empty name or moves", i+1)
		}

		board := chess.NewStartingPosition()
		for _, move := range moves {
			next, _, err := board.ApplyUCIMove(move)
			if err != nil {
				return nil, fmt.Errorf("book %q: move %q: %w", name, move, err)
			}
			board = next
		}
		if len(moves) < 4 || len(moves) > 14 {
			return nil, fmt.Errorf("book %q: %d plies, want 4-14", name, len(moves))
		}
		if _, ok := seen[board.Key()]; ok {
			continue // transposition of an earlier line
		}
		seen[board.Key()] = name
		openings = append(openings, opening{Name: name, Moves: moves})
	}
	if len(openings) == 0 {
		return nil, fmt.Errorf("book has no usable lines")
	}
	return openings, nil
}
