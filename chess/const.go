package chess

import (
	"iter"
)

// Square is a board index: 0 = a1 and 63 = h8.
type Square int

const squareCount = 64
const boardSize = 8

// FileRank is a zero-based file and rank coordinate.
type FileRank struct {
	file uint64
	rank uint64
}

// FileRankFromSquare converts a square index to file/rank coordinates.
func FileRankFromSquare(square Square) FileRank {
	return FileRank{file: uint64(square % boardSize), rank: uint64(square / boardSize)}
}

// SquareFromFileRank converts file/rank coordinates to a square index.
func SquareFromFileRank(fr FileRank) Square {
	return Square(fr.file + fr.rank*boardSize)
}

// File returns the zero-based file (0 = a).
func (fr FileRank) File() uint64 { return fr.file }

// Rank returns the zero-based rank (0 = rank 1).
func (fr FileRank) Rank() uint64 { return fr.rank }

// Piece identifies a piece type; black pieces use blackPieceOffset.
type Piece int

const (
	NoPiece Piece = -1
	Pawn    Piece = 0
	Bishop  Piece = 1
	Knight  Piece = 2
	Rook    Piece = 3
	Queen   Piece = 4
	King    Piece = 5
)

const blackPieceOffset = 6

// PieceTypes yields the piece indices for a colour.
func PieceTypes(colour Colour) iter.Seq[Piece] {
	var start Piece
	var stop Piece
	switch colour {
	case White:
		start = 0
		stop = blackPieceOffset
	case Black:
		start = blackPieceOffset
		stop = blackPieceOffset + 6
	case Both:
		start = 0
		stop = 12
	}

	return func(yield func(Piece) bool) {
		for i := start; i < stop; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

// Position flag bits.
const (
	whiteToMove            uint8 = 1
	whiteKingsideCastling  uint8 = 1 << 1
	whiteQueensideCastling uint8 = 1 << 2
	blackKingsideCastling  uint8 = 1 << 3
	blackQueensideCastling uint8 = 1 << 4
)

// Colour selects one side, or Both when iterating.
type Colour int

const (
	Both  Colour = -1
	White Colour = 0
	Black Colour = 1
)

// Opposite returns the other colour.
func (c Colour) Opposite() Colour {
	switch c {
	case White:
		return Black
	case Black:
		return White
	default:
		return Both
	}
}

// rank1Mask selects the first rank.
const rank1Mask BitBoard = 255

// File masks are used when shifting pawns to find attacks.
const (
	fileAMask BitBoard = 0x0101010101010101
	fileHMask BitBoard = 0x8080808080808080
)

// Named squares used by castling and move generation.
const (
	a1 Square = 0
	b1 Square = 1
	c1 Square = 2
	d1 Square = 3
	e1 Square = 4
	f1 Square = 5
	g1 Square = 6
	h1 Square = 7

	a8 Square = 56
	b8 Square = 57
	c8 Square = 58
	d8 Square = 59
	e8 Square = 60
	f8 Square = 61
	g8 Square = 62
	h8 Square = 63
)

// Piece and board notation tables. The piece symbols follow the Piece
// constants: white pieces first, then black pieces offset by blackPieceOffset.
var (
	pieceSymbols        = []string{"P", "B", "N", "R", "Q", "K", "p", "b", "n", "r", "q", "k"}
	unicodePieceSymbols = []string{"♙", "♗", "♘", "♖", "♕", "♔", "♟", "♝", "♞", "♜", "♛", "♚"}
	castlingSymbols     = []string{"K", "Q", "k", "q"}
	fileNames           = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'}
)
