package chess

import (
	"errors"
	"fmt"
	"slices"
)

type Move struct {
	start    uint64 //starting position of piece
	end      uint64 //ending position of piece
	encoding uint16 //encoding for move information
}

// Genrates a new move with no code, TODO: handel codes in the right way.
// Idea for codes might be somthing that engine will generate after ponder.
func NewMoveUCI(UCI string) (*Move, error) {
	s := fmt.Sprintf("Invalid UCI code %s", UCI)
	if len(UCI) != 4 {
		return nil, errors.New(s)
	}

	start, err := CalcLocFromAlg(UCI[:2])
	if err != nil {
		return nil, errors.Join(errors.New(s), err)
	}

	end, err := CalcLocFromAlg(UCI[2:])
	if err != nil {
		return nil, errors.Join(errors.New(s), err)
	}

	return &Move{start, end, 0}, nil
}

func (m *Move) String() string {
	scol, srow, ecol, erow := 0, 0, 0, 0
	colMask := COLONM_MASK
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

	return fmt.Sprintf("%c%d%c%d", COLONMS[scol], srow+1, COLONMS[ecol], erow+1)
}

// find algebraic position from position
func AlgFromLoc(loc uint64) string {
	col, row := 0, 0
	colMask := COLONM_MASK
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

	return fmt.Sprintf("%c%d", COLONMS[col], row+1)
}

// Given algerbraic notation for a position (e.g. c5) calculate the position.
func CalcLocFromAlg(alg string) (uint64, error) {
	col := slices.Index(COLONMS, rune(alg[0]))
	if col == -1 {
		s := fmt.Sprintf("Invalid algerbraic notation %s", alg)
		return 0, errors.New(s)
	}

	row := int(alg[1]-'0') - 1 //its imporant to subtract by 1
	if row < 0 || row >= 8 {
		s := fmt.Sprintf("Invalid algerbraic notation %s", alg)
		return 0, errors.New(s)
	}

	return 1 << (col + row*8), nil
}
