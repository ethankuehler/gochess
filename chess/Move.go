package chess

import (
	"fmt"
	"iter"
	"log"
	"math/bits"
	"slices"
	"strings"
)

// promotionFlag marks a move that promotes a pawn. The low three bits of
// encoding then hold the promotion Piece (QUEEN, ROOK, BISHOP, or KNIGHT).
const promotionFlag uint16 = 1 << 3

type Move struct {
	start    BitBoard //starting position of piece
	end      BitBoard //ending position of piece
	encoding uint16   //flags; see promotionFlag
}

// NewMoveUCI creates a move from a UCI string such as "e2e4" or "e7e8q".
func NewMoveUCI(UCI string) (*Move, error) {
	if len(UCI) != 4 && len(UCI) != 5 {
		return nil, fmt.Errorf("invalid UCI move %q: expected 4 or 5 characters", UCI)
	}

	start, err := LocFromAlg(UCI[:2])
	if err != nil {
		return nil, fmt.Errorf("invalid UCI move %q: %w", UCI, err)
	}

	end, err := LocFromAlg(UCI[2:4])
	if err != nil {
		return nil, fmt.Errorf("invalid UCI move %q: %w", UCI, err)
	}

	if start == end {
		return nil, fmt.Errorf("invalid UCI move %q: start and end squares are the same", UCI)
	}

	var encoding uint16
	if len(UCI) == 5 {
		var promotion Piece
		switch strings.ToLower(UCI[4:]) {
		case "q":
			promotion = QUEEN
		case "r":
			promotion = ROOK
		case "b":
			promotion = BISHOP
		case "n":
			promotion = KNIGHT
		default:
			return nil, fmt.Errorf("invalid UCI move %q: promotion must be q, r, b, or n", UCI)
		}
		encoding = uint16(promotion) | promotionFlag
	}

	return &Move{start: start, end: end, encoding: encoding}, nil
}

// String renders the move in UCI notation, including a promotion suffix.
func (m Move) String() string {
	uci := AlgFromLoc(m.start) + AlgFromLoc(m.end)
	if promotion, ok := m.Promotion(); ok {
		uci += strings.ToLower(PICECES_SYM[promotion])
	}
	return uci
}

// From returns the square the move starts on.
func (m Move) From() Shift {
	return Shift(bits.TrailingZeros64(uint64(m.start)))
}

// To returns the square the move ends on.
func (m Move) To() Shift {
	return Shift(bits.TrailingZeros64(uint64(m.end)))
}

// Promotion returns the piece a pawn promotes to. ok is false for moves that
// are not promotions.
func (m Move) Promotion() (promotion Piece, ok bool) {
	if m.encoding&promotionFlag == 0 {
		return ALL, false
	}
	return Piece(m.encoding & 0b111), true
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
	if len(alg) != 2 {
		return 0, 0, fmt.Errorf("invalid algebraic notation %q: expected two characters", alg)
	}

	col := slices.Index(COLUMNS, rune(alg[0]))
	if col == -1 || alg[1] < '1' || alg[1] > '8' {
		return 0, 0, fmt.Errorf("invalid algebraic notation %q", alg)
	}

	row := int(alg[1]-'0') - 1
	return uint64(row), uint64(col), nil
}

func ShiftFromAlg(alg string) (Shift, error) {
	row, col, err := RowColFromAlg(alg)
	if err != nil {
		return 0, err
	}
	return ShiftFromCoords(Coordinates{row: row, col: col}), nil
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
