package chess

import (
	"fmt"
	"slices"
	"strings"
)

// Move flags. The low three bits of flags hold the promotion Piece when
// promotionFlag is set; the remaining bits describe the move. Flags currently
// occupy bits 0-7; bits 8-15 are reserved for future use.
const (
	promotionFlag  uint16 = 1 << 3
	captureFlag    uint16 = 1 << 4
	doublePushFlag uint16 = 1 << 5
	enPassantFlag  uint16 = 1 << 6
	castlingFlag   uint16 = 1 << 7

	promotionPieceMask uint16 = 0b111
)

// Move is a from/to square pair plus move flags. It is deliberately kept to
// four bytes so move buffers and the principal-variation table stay small: the
// squares are stored as uint8 indices in [0, 63] and flags is a uint16 bit
// field.
type Move struct {
	from  uint8
	to    uint8
	flags uint16
}

// newMove builds a Move from square indices that must be in [0, squareCount).
// It is the only place that narrows a Square to the byte-sized stored fields.
func newMove(from, to Square, flags uint16) Move {
	return Move{from: uint8(from), to: uint8(to), flags: flags}
}

// ParseUCIMove creates a move from a UCI string such as "e2e4" or "e7e8q".
// It parses notation only: it cannot know whether a move is a capture,
// castling, en passant, or a double pawn push. Match a parsed move against
// Position.LegalMoves before applying it.
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

// String renders the move in UCI notation, including a promotion suffix.
func (m Move) String() string {
	uci := SquareName(Square(m.from)) + SquareName(Square(m.to))
	if promotion, ok := m.Promotion(); ok {
		uci += strings.ToLower(pieceSymbols[promotion])
	}
	return uci
}

// From returns the square the move starts on.
func (m Move) From() Square { return Square(m.from) }

// To returns the square the move ends on.
func (m Move) To() Square { return Square(m.to) }

// Promotion returns the piece a pawn promotes to. ok is false for moves that
// are not promotions.
func (m Move) Promotion() (promotion Piece, ok bool) {
	if m.flags&promotionFlag == 0 {
		return NoPiece, false
	}
	return Piece(m.flags & promotionPieceMask), true
}

// IsCapture reports whether the move captures a piece, including en passant.
func (m Move) IsCapture() bool { return m.flags&captureFlag != 0 }

// IsEnPassant reports whether the move is an en passant capture.
func (m Move) IsEnPassant() bool { return m.flags&enPassantFlag != 0 }

// IsCastling reports whether the move castles.
func (m Move) IsCastling() bool { return m.flags&castlingFlag != 0 }

// IsDoublePush reports whether the move is a two-square pawn push.
func (m Move) IsDoublePush() bool { return m.flags&doublePushFlag != 0 }

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
