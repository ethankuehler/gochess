package chess

import (
	"fmt"
	"math/bits"
	"slices"
	"strings"
)

// promotionFlag marks a move that promotes a pawn. The low three bits of
// encoding then hold the promotion Piece (Queen, Rook, Bishop, or Knight).
const promotionFlag uint16 = 1 << 3

type Move struct {
	start    BitBoard //starting position of piece
	end      BitBoard //ending position of piece
	encoding uint16   //flags; see promotionFlag
}

// ParseUCIMove creates a move from a UCI string such as "e2e4" or "e7e8q".
func ParseUCIMove(UCI string) (*Move, error) {
	if len(UCI) != 4 && len(UCI) != 5 {
		return nil, fmt.Errorf("invalid UCI move %q: expected 4 or 5 characters", UCI)
	}

	start, err := BitBoardFromAlg(UCI[:2])
	if err != nil {
		return nil, fmt.Errorf("invalid UCI move %q: %w", UCI, err)
	}

	end, err := BitBoardFromAlg(UCI[2:4])
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
			promotion = Queen
		case "r":
			promotion = Rook
		case "b":
			promotion = Bishop
		case "n":
			promotion = Knight
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
		uci += strings.ToLower(pieceSymbols[promotion])
	}
	return uci
}

// From returns the square the move starts on.
func (m Move) From() Square {
	return Square(bits.TrailingZeros64(uint64(m.start)))
}

// To returns the square the move ends on.
func (m Move) To() Square {
	return Square(bits.TrailingZeros64(uint64(m.end)))
}

// Promotion returns the piece a pawn promotes to. ok is false for moves that
// are not promotions.
func (m Move) Promotion() (promotion Piece, ok bool) {
	if m.encoding&promotionFlag == 0 {
		return NoPiece, false
	}
	return Piece(m.encoding & 0b111), true
}

// find algebraic position from position
func AlgFromLoc(loc BitBoard) string {
	col, row := 0, 0
	colMask := fileAMask
	rowMask := rank1Mask
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

	return fmt.Sprintf("%c%d", fileNames[col], row+1)
}

func ParseFileRank(alg string) (uint64, uint64, error) {
	if len(alg) != 2 {
		return 0, 0, fmt.Errorf("invalid algebraic notation %q: expected two characters", alg)
	}

	col := slices.Index(fileNames, rune(alg[0]))
	if col == -1 || alg[1] < '1' || alg[1] > '8' {
		return 0, 0, fmt.Errorf("invalid algebraic notation %q", alg)
	}

	row := int(alg[1]-'0') - 1
	return uint64(row), uint64(col), nil
}

func ParseSquare(alg string) (Square, error) {
	row, col, err := ParseFileRank(alg)
	if err != nil {
		return 0, err
	}
	return SquareFromFileRank(FileRank{row: row, col: col}), nil
}

// Given algebraic notation for a position (e.g. c5) calculate the position.
func BitBoardFromAlg(alg string) (BitBoard, error) {
	shift, err := ParseSquare(alg)
	if err != nil {
		return 0, err
	}
	return 1 << shift, nil
}
