package chess

import (
	"bytes"
	"fmt"
	"math/bits"
	"slices"
	"strconv"
	"strings"
)

// BitBoard is a set of board squares, one bit per square.
type BitBoard uint64

// Position is a complete chess position: piece placement, side to move,
// castling rights, the en passant target, and both move clocks.
type Position struct {
	pieceBitboards  [12]BitBoard // one bitboard per piece type and colour
	enPassantSquare BitBoard     // en passant target square
	flags           uint8        // packed turn and castling rights
	halfmoveClock   uint16       // half moves since the last pawn advance or capture
	fullmoveNumber  uint16       // full moves
}

func (b BitBoard) String() string {
	var buffer bytes.Buffer

	for rank := boardSize - 1; rank >= 0; rank-- {
		for file := 0; file < boardSize; file++ {
			mask := BitBoard(1) << SquareFromFileRank(FileRank{rank: uint64(rank), file: uint64(file)})
			if b&mask > 0 {
				buffer.WriteString(" 1 ")
			} else {
				buffer.WriteString(" . ")
			}
		}
		buffer.WriteRune('\n')
	}
	return buffer.String()
}

// SquaresToBitBoard converts algebraic square strings (e.g. "a1", "e4") to
// a bitboard mask.
func SquaresToBitBoard(squares []string) (BitBoard, error) {
	var board BitBoard
	for _, square := range squares {
		squareBit, err := BitBoardFromAlgebraic(square)
		if err != nil {
			return 0, fmt.Errorf("invalid square %q: %w", square, err)
		}
		board |= squareBit
	}
	return board, nil
}

// NewStartingPosition returns the standard chess starting position.
func NewStartingPosition() *Position {
	b := Position{}

	// Set the white pieces on their home squares.
	b.pieceBitboards[Pawn] = 0b1111111100000000
	b.pieceBitboards[Bishop] = 0b00100100
	b.pieceBitboards[Knight] = 0b01000010
	b.pieceBitboards[Rook] = 0b10000001
	b.pieceBitboards[Queen] = 0b00001000 // d1 (bit 3) - standard chess position
	b.pieceBitboards[King] = 0b00010000  // e1 (bit 4) - standard chess position

	// Copy the pieces for black.
	b.pieceBitboards[Pawn+blackPieceOffset] = b.pieceBitboards[Pawn] << 40
	b.pieceBitboards[Bishop+blackPieceOffset] = b.pieceBitboards[Bishop] << 56
	b.pieceBitboards[Knight+blackPieceOffset] = b.pieceBitboards[Knight] << 56
	b.pieceBitboards[Rook+blackPieceOffset] = b.pieceBitboards[Rook] << 56
	b.pieceBitboards[Queen+blackPieceOffset] = b.pieceBitboards[Queen] << 56
	b.pieceBitboards[King+blackPieceOffset] = b.pieceBitboards[King] << 56

	// White to move with all castling rights.
	b.flags |= whiteToMove | whiteKingsideCastling | whiteQueensideCastling | blackKingsideCastling | blackQueensideCastling

	// The halfmove clock starts at zero and the fullmove number at one.
	b.halfmoveClock = 0
	b.fullmoveNumber = 1

	return &b
}

// ParseFEN builds a board from a FEN string, validating every field.
func ParseFEN(fen string) (*Position, error) {
	b := Position{}

	fields := strings.Fields(fen)
	if len(fields) != 6 {
		return nil, fmt.Errorf("invalid FEN %q: expected 6 fields, got %d", fen, len(fields))
	}

	ranks := strings.Split(fields[0], "/")
	if len(ranks) != boardSize {
		return nil, fmt.Errorf("invalid FEN %q: piece placement must have 8 ranks", fen)
	}

	var mask BitBoard = 1 << 63
	for _, rank := range ranks {
		width := 0
		for i := len(rank) - 1; i >= 0; i-- {
			v := rank[i]
			if idx := slices.Index(pieceSymbols, string(v)); idx != -1 {
				if width >= boardSize {
					return nil, fmt.Errorf("invalid FEN %q: rank %q has more than 8 squares", fen, rank)
				}
				b.pieceBitboards[idx] |= mask
				mask >>= 1
				width++
				continue
			}
			if v < '1' || v > '8' {
				return nil, fmt.Errorf("invalid FEN %q: unexpected %q in piece placement", fen, v)
			}
			empty := int(v - '0')
			if width+empty > boardSize {
				return nil, fmt.Errorf("invalid FEN %q: rank %q has more than 8 squares", fen, rank)
			}
			mask >>= empty
			width += empty
		}
		if width != boardSize {
			return nil, fmt.Errorf("invalid FEN %q: rank %q does not have 8 squares", fen, rank)
		}
	}
	if mask != 0 {
		return nil, fmt.Errorf("invalid FEN %q: piece placement is invalid", fen)
	}

	if bits.OnesCount64(uint64(b.pieceBitboards[King])) != 1 {
		return nil, fmt.Errorf("invalid FEN %q: white must have exactly one king", fen)
	}
	if bits.OnesCount64(uint64(b.pieceBitboards[King+blackPieceOffset])) != 1 {
		return nil, fmt.Errorf("invalid FEN %q: black must have exactly one king", fen)
	}

	for _, side := range []struct {
		name   string
		offset int
	}{{"white", 0}, {"black", blackPieceOffset}} {
		count := 0
		for piece := Pawn; piece <= King; piece++ {
			count += bits.OnesCount64(uint64(b.pieceBitboards[int(piece)+side.offset]))
		}
		if count > 16 {
			return nil, fmt.Errorf("invalid FEN %q: %s has more than 16 pieces", fen, side.name)
		}
		if pawns := bits.OnesCount64(uint64(b.pieceBitboards[int(Pawn)+side.offset])); pawns > 8 {
			return nil, fmt.Errorf("invalid FEN %q: %s has %d pawns, at most 8 are possible", fen, side.name, pawns)
		}
	}

	const lastRank BitBoard = rank1Mask << 56
	for _, offset := range []int{0, blackPieceOffset} {
		if b.pieceBitboards[int(Pawn)+offset]&(rank1Mask|lastRank) != 0 {
			return nil, fmt.Errorf("invalid FEN %q: pawns cannot be on the first or last rank", fen)
		}
	}

	b.flags = 0

	// Players turn.
	switch fields[1] {
	case "w":
		b.flags |= whiteToMove
	case "b":
	default:
		return nil, fmt.Errorf("invalid FEN %q: invalid side to move %q", fen, fields[1])
	}

	// Castling rights.
	castling := fields[2]
	if castling != "-" {
		seen := make(map[rune]bool, len(castling))
		for _, v := range castling {
			if !strings.ContainsRune("KQkq", v) {
				return nil, fmt.Errorf("invalid FEN %q: invalid castling right %q", fen, v)
			}
			if seen[v] {
				return nil, fmt.Errorf("invalid FEN %q: duplicate castling right %q", fen, v)
			}
			seen[v] = true
		}
	}
	for i, v := range castlingSymbols {
		if strings.Contains(castling, v) {
			b.flags |= 1 << (i + 1)
		}
	}

	// En passant target.
	if fields[3] == "-" {
		b.enPassantSquare = 0
	} else {
		fr, err := ParseFileRank(fields[3])
		if err != nil {
			return nil, fmt.Errorf("invalid FEN %q: en passant square: %w", fen, err)
		}
		if fr.rank != 2 && fr.rank != 5 {
			return nil, fmt.Errorf("invalid FEN %q: en passant square %q must be on rank 3 or 6", fen, fields[3])
		}
		b.enPassantSquare = BitBoard(1) << SquareFromFileRank(fr)
	}

	// Halfmove clock.
	v, err := strconv.Atoi(fields[4])
	if err != nil {
		return nil, fmt.Errorf("invalid FEN %q: halfmove clock %q is not a number", fen, fields[4])
	}
	if v < 0 || v > 65535 {
		return nil, fmt.Errorf("invalid FEN %q: halfmove clock %d is out of range", fen, v)
	}
	b.halfmoveClock = uint16(v)

	// Fullmove number. Standard FEN numbering starts at 1.
	v, err = strconv.Atoi(fields[5])
	if err != nil {
		return nil, fmt.Errorf("invalid FEN %q: fullmove number %q is not a number", fen, fields[5])
	}
	if v < 1 || v > 65535 {
		return nil, fmt.Errorf("invalid FEN %q: fullmove number %d is out of range", fen, v)
	}
	b.fullmoveNumber = uint16(v)

	return &b, nil
}

// stateString returns the FEN fields that follow the piece placement.
func (b *Position) stateString() string {
	var buffer bytes.Buffer
	// Side to move.
	buffer.WriteRune(' ')
	if b.flags&whiteToMove > 0 {
		buffer.WriteRune('w')
	} else {
		buffer.WriteRune('b')
	}

	// Castling rights.
	buffer.WriteRune(' ')
	canCastle := false
	for i, v := range castlingSymbols {
		if b.flags&(1<<(i+1)) > 0 {
			buffer.WriteString(v)
			canCastle = true
		}
	}
	if !canCastle {
		buffer.WriteRune('-')
	}

	// En passant target.
	buffer.WriteRune(' ')
	if b.enPassantSquare > 0 {
		buffer.WriteString(SquareName(Square(bits.TrailingZeros64(uint64(b.enPassantSquare)))))
	} else {
		buffer.WriteRune('-')
	}

	buffer.WriteRune(' ')
	buffer.WriteString(strconv.Itoa(int(b.halfmoveClock)))

	buffer.WriteRune(' ')
	buffer.WriteString(strconv.Itoa(int(b.fullmoveNumber)))

	return buffer.String()
}

func (b *Position) format(symbols []string) string {
	boardStr := ""
	for rank := boardSize - 1; rank >= 0; rank-- {
		for file := 0; file < boardSize; file++ {
			mask := BitBoard(1) << SquareFromFileRank(FileRank{rank: uint64(rank), file: uint64(file)})
			s := " _ "
			for k, p := range b.pieceBitboards {
				if mask&p > 0 {
					s = " " + symbols[k] + " "
					break
				}
			}
			boardStr += s
		}
		boardStr += "\n"
	}
	boardStr += b.stateString()
	return boardStr
}

// String returns the board as text using ASCII piece symbols.
func (b *Position) String() string {
	return b.format(pieceSymbols)
}

// UnicodeString returns the board as text using Unicode chess symbols.
func (b *Position) UnicodeString() string {
	return b.format(unicodePieceSymbols)
}

// FEN returns the position in FEN notation.
func (b *Position) FEN() string {
	var buffer bytes.Buffer

	for rank := boardSize - 1; rank >= 0; rank-- {
		emptyCount := 0
		for file := 0; file < boardSize; file++ {
			mask := BitBoard(1) << SquareFromFileRank(FileRank{rank: uint64(rank), file: uint64(file)})

			// Determine whether a piece occupies the square.
			found := false
			for i, v := range b.pieceBitboards {
				if v&mask > 0 {
					if emptyCount > 0 {
						buffer.WriteString(strconv.Itoa(emptyCount))
						emptyCount = 0
					}
					buffer.WriteString(pieceSymbols[i])
					found = true
					break
				}
			}

			// If no piece is found, count an empty square.
			if !found {
				emptyCount++
			}

		}
		if emptyCount != 0 {
			buffer.WriteString(strconv.Itoa(emptyCount))
		}
		if rank != 0 {
			buffer.WriteRune('/')
		}
	}
	buffer.WriteString(b.stateString())
	return buffer.String()
}

// Occupancy returns a mask of every occupied square for the given colour.
// Colour should be White, Black, or Both.
func (b *Position) Occupancy(colour Colour) BitBoard {
	var occupied BitBoard
	for i := range PieceTypes(colour) {
		occupied |= b.pieceBitboards[i]
	}
	return occupied
}

// PieceBitboard returns the bitboard for one piece type and colour.
func (b *Position) PieceBitboard(colour Colour, piece Piece) BitBoard {
	if colour != White && colour != Black {
		panic("PieceBitboard: colour must be White or Black")
	}
	if piece < Pawn || piece > King {
		panic("PieceBitboard: invalid piece")
	}

	offset := 0
	if colour == Black {
		offset = blackPieceOffset
	}

	return b.pieceBitboards[int(piece)+offset]
}
