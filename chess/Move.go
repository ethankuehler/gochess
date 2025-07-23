package chess

import (
	"errors"
	"fmt"
	"iter"
	"log"
	"slices"
)

type Move struct {
	start    BitBoard //starting position of piece
	end      BitBoard //ending position of piece
	encoding uint16   //encoding for move information
}

// Genrates a new move with no code, TODO: handel codes in the right way.
// Idea for codes might be somthing that engine will generate after ponder.
func NewMoveUCI(UCI string) (*Move, error) {
	s := fmt.Sprintf("Invalid UCI code %s", UCI)
	if len(UCI) != 4 {
		return nil, errors.New(s)
	}

	start, err := LocFromAlg(UCI[:2])
	if err != nil {
		return nil, errors.Join(errors.New(s), err)
	}

	end, err := LocFromAlg(UCI[2:])
	if err != nil {
		return nil, errors.Join(errors.New(s), err)
	}

	return &Move{start, end, 0}, nil
}

func (m *Move) String() string {
	scol, srow, ecol, erow := 0, 0, 0, 0
	colMask := COLUMN_MASK
	rowMask := ROW_MASK
	for i := range 8 {
		if (colMask & m.start) > 0 {
			scol = i
		}
		if (rowMask & m.start) > 0 {
			srow = i
		}
		if (colMask & m.end) > 0 {
			ecol = i
		}
		if (rowMask & m.end) > 0 {
			erow = i
		}
		colMask = colMask << 1
		rowMask = rowMask << 8
	}

	return fmt.Sprintf("%c%d%c%d", COLUMNS[scol], srow+1, COLUMNS[ecol], erow+1)
}

// find algebraic position from position
func AlgFromLoc(loc BitBoard) string {
	col, row := 0, 0
	colMask := COLUMN_MASK
	rowMask := ROW_MASK
	for i := range 8 {
		if colMask&loc > 0 {
			col = i
		}
		if rowMask&loc > 0 {
			row = i
		}
		colMask = colMask << 1
		rowMask = rowMask << 8
	}

	return fmt.Sprintf("%c%d", COLUMNS[col], row+1)
}

func RowColFromAlg(alg string) (uint64, uint64, error) {
	col := slices.Index(COLUMNS, rune(alg[0]))
	if col == -1 {
		s := fmt.Sprintf("Invalid algerbraic notation %s", alg)
		return 0, 0, errors.New(s)
	}

	row := int(alg[1]-'0') - 1 //its imporant to subtract by 1
	if row < 0 || row >= 8 {
		s := fmt.Sprintf("Invalid algerbraic notation %s", alg)
		return 0, 0, errors.New(s)
	}
	return uint64(row), uint64(col), nil
}

func ShiftFromAlg(alg string) (Shift, error) {
	row, col, err := RowColFromAlg(alg)
	if err != nil {
		return 0, err
	}
	return Shift(col + row*8), nil
}

// Given algerbraic notation for a position (e.g. c5) calculate the position.
func LocFromAlg(alg string) (BitBoard, error) {
	shift, err := ShiftFromAlg(alg)
	if err != nil {
		return 0, err
	}
	return 1 << shift, nil
}

func ShiftIter(start_str, stop_str string) iter.Seq[Shift] {
	start, err := ShiftFromAlg(start_str)
	if err != nil {
		log.Fatal(err.Error())
	}
	stop, err := ShiftFromAlg(stop_str)
	if err != nil {
		log.Fatal(err.Error())
	}
	return func(yield func(Shift) bool) {
		for i := start; i <= stop; i++ {
			if !yield(i) {
				return
			}
		}
	}
}
