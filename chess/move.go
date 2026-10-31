package chess

import (
	"fmt"
	"slices"
	"strings"
)

const (
	promotionFlag  uint16 = 1 << 3
	captureFlag    uint16 = 1 << 4
	doublePushFlag uint16 = 1 << 5
	enPassantFlag  uint16 = 1 << 6
	castlingFlag   uint16 = 1 << 7

	promotionPieceMask uint16 = 0b111
)

type Move struct {
	from  uint8
	to    uint8
	flags uint16
}

// newMove is the only place that narrows Square to the stored bytes.
func newMove(from, to Square, flags uint16) Move {
	return Move{from: uint8(from), to: uint8(to), flags: flags}
}

// ParseUCIMove parses notation only; match the result against LegalMoves before applying it.
func ParseUCIMove(uci string) (*Move, error) {
	if len(uci) != 4 && len(uci) != 5 {
		return nil, fmt.Errorf("invalid UCI move %q: expected 4 or 5 characters", uci)
	}

	from, err := ParseSquare(uci[:2])
	if err != nil {
		return nil, fmt.Errorf("invalid UCI move %q: %w", uci, err)
	}

	to, err := ParseSquare(uci[2:4])
	if err != nil {
		return nil, fmt.Errorf("invalid UCI move %q: %w", uci, err)
	}

	if from == to {
		return nil, fmt.Errorf("invalid UCI move %q: start and end squares are the same", uci)
	}

	var flags uint16
	if len(uci) == 5 {
		var promotion Piece
		switch strings.ToLower(uci[4:]) {
		case "q":
			promotion = Queen
		case "r":
			promotion = Rook
		case "b":
			promotion = Bishop
		case "n":
			promotion = Knight
		default:
			return nil, fmt.Errorf("invalid UCI move %q: promotion must be q, r, b, or n", uci)
		}
		flags = uint16(promotion) | promotionFlag
	}

	m := newMove(from, to, flags)
	return &m, nil
}

func (m Move) String() string {
	uci := SquareName(Square(m.from)) + SquareName(Square(m.to))
	if promotion, ok := m.Promotion(); ok {
		uci += strings.ToLower(pieceSymbols[promotion])
	}
	return uci
}

func (m Move) From() Square { return Square(m.from) }

func (m Move) To() Square { return Square(m.to) }

func (m Move) Promotion() (promotion Piece, ok bool) {
	if m.flags&promotionFlag == 0 {
		return NoPiece, false
	}
	return Piece(m.flags & promotionPieceMask), true
}

func (m Move) IsCapture() bool { return m.flags&captureFlag != 0 }

func (m Move) IsEnPassant() bool { return m.flags&enPassantFlag != 0 }

func (m Move) IsCastling() bool { return m.flags&castlingFlag != 0 }

func (m Move) IsDoublePush() bool { return m.flags&doublePushFlag != 0 }

func SquareName(square Square) string {
	fr := FileRankFromSquare(square)
	return fmt.Sprintf("%c%d", fileNames[fr.file], fr.rank+1)
}

func ParseFileRank(notation string) (FileRank, error) {
	if len(notation) != 2 {
		return FileRank{}, fmt.Errorf("invalid algebraic notation %q: expected two characters", notation)
	}

	file := slices.Index(fileNames, rune(notation[0]))
	if file == -1 || notation[1] < '1' || notation[1] > '8' {
		return FileRank{}, fmt.Errorf("invalid algebraic notation %q", notation)
	}

	rank := int(notation[1]-'0') - 1
	return FileRank{file: uint64(file), rank: uint64(rank)}, nil
}

func ParseSquare(notation string) (Square, error) {
	fr, err := ParseFileRank(notation)
	if err != nil {
		return 0, err
	}
	return SquareFromFileRank(fr), nil
}

func BitBoardFromAlgebraic(notation string) (BitBoard, error) {
	square, err := ParseSquare(notation)
	if err != nil {
		return 0, err
	}
	return BitBoard(1) << square, nil
}
