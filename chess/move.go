package chess

import (
	"fmt"
	"slices"
	"strings"
)

// promotionFlag marks a move that promotes a pawn. The low three bits of
// flags then hold the promotion Piece (Queen, Rook, Bishop, or Knight).
const promotionFlag uint16 = 1 << 3

// Move is a from/to square pair plus move flags.
type Move struct {
	from  Square
	to    Square
	flags uint16
}

// ParseUCIMove creates a move from a UCI string such as "e2e4" or "e7e8q".
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

	return &Move{from: from, to: to, flags: flags}, nil
}

// String renders the move in UCI notation, including a promotion suffix.
func (m Move) String() string {
	uci := SquareName(m.from) + SquareName(m.to)
	if promotion, ok := m.Promotion(); ok {
		uci += strings.ToLower(pieceSymbols[promotion])
	}
	return uci
}

// From returns the square the move starts on.
func (m Move) From() Square { return m.from }

// To returns the square the move ends on.
func (m Move) To() Square { return m.to }

// Promotion returns the piece a pawn promotes to. ok is false for moves that
// are not promotions.
func (m Move) Promotion() (promotion Piece, ok bool) {
	if m.flags&promotionFlag == 0 {
		return NoPiece, false
	}
	return Piece(m.flags & 0b111), true
}

// SquareName formats a square in algebraic notation (e.g. "e4").
func SquareName(square Square) string {
	fr := FileRankFromSquare(square)
	return fmt.Sprintf("%c%d", fileNames[fr.file], fr.rank+1)
}

// ParseFileRank parses algebraic notation (e.g. "e4") into file/rank coordinates.
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

// ParseSquare parses algebraic notation (e.g. "e4") into a square index.
func ParseSquare(notation string) (Square, error) {
	fr, err := ParseFileRank(notation)
	if err != nil {
		return 0, err
	}
	return SquareFromFileRank(fr), nil
}

// BitBoardFromAlgebraic parses algebraic notation into a single-square bitboard.
func BitBoardFromAlgebraic(notation string) (BitBoard, error) {
	square, err := ParseSquare(notation)
	if err != nil {
		return 0, err
	}
	return BitBoard(1) << square, nil
}
