package chess

import (
	"errors"
	"fmt"
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