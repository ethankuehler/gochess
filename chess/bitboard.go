package chess

import (
	"bytes"
	"fmt"
	"math/bits"
	"slices"
	"strconv"
	"strings"
)

type BitBoard uint64

type Position struct {
	pieces          [12]BitBoard // one bitboard per piece type and colour
	enPassantSquare BitBoard     // en passant target square
	encoding        uint8        // packed turn and castling rights
	halfmove_clock  uint16       // half moves since the last pawn advance or capture
	fullmove_number uint16       // full moves
}

func (b BitBoard) String() string {
	var buffer bytes.Buffer

	for row := boardSize - 1; row >= 0; row-- {
		for col := 0; col < boardSize; col++ {
			mask := BitBoard(1) << SquareFromFileRank(FileRank{row: uint64(row), col: uint64(col)})
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

// SquaresToBitBoard converts algebraic square strings (e.g. "a1", "e4") to a bitboard mask.
func SquaresToBitBoard(squares []string) (BitBoard, error) {
	var board BitBoard = 0
	for _, square := range squares {
		loc, err := BitBoardFromAlg(square)
		if err != nil {
			return 0, fmt.Errorf("invalid square %q: %w", square, err)
		}
		board |= loc
	}
	return board, nil
}

// NewStartingPosition returns the standard chess starting position.
func NewStartingPosition() *Position {
	b := Position{}
	//setting all the white pieces on the home squares
	b.pieces[Pawn] = 0b1111111100000000
	b.pieces[Bishop] = 0b00100100
	b.pieces[Knight] = 0b01000010
	b.pieces[Rook] = 0b10000001
	b.pieces[Queen] = 0b00001000 // d1 (bit 3) - standard chess position
	b.pieces[King] = 0b00010000  // e1 (bit 4) - standard chess position

	// Copy the pieces for black.
	b.pieces[Pawn+blackPieceOffset] = b.pieces[Pawn] << 40
	b.pieces[Bishop+blackPieceOffset] = b.pieces[Bishop] << 56
	b.pieces[Knight+blackPieceOffset] = b.pieces[Knight] << 56
	b.pieces[Rook+blackPieceOffset] = b.pieces[Rook] << 56
	b.pieces[Queen+blackPieceOffset] = b.pieces[Queen] << 56
	b.pieces[King+blackPieceOffset] = b.pieces[King] << 56

	//default encoding at the start of a chess game.
	b.encoding |= whiteToMove | whiteKingsideCastling | whiteQueensideCastling | blackKingsideCastling | blackQueensideCastling

	// The halfmove clock starts at zero and the fullmove number at one.
	b.halfmove_clock = 0
	b.fullmove_number = 1

	return &b
}

// ParseFEN builds a board from a FEN string, validating every field.
func ParseFEN(FEN string) (*Position, error) {
	b := Position{}

	fields := strings.Fields(FEN)
	if len(fields) != 6 {
		return nil, fmt.Errorf("invalid FEN %q: expected 6 fields, got %d", FEN, len(fields))
	}

	rows := strings.Split(fields[0], "/")
	if len(rows) != 8 {
		return nil, fmt.Errorf("invalid FEN %q: piece placement must have 8 ranks", FEN)
	}

	var mask BitBoard = 1 << 63
	for _, row := range rows {
		width := 0
		for i := len(row) - 1; i >= 0; i-- {
			v := row[i]
			if idx := slices.Index(pieceSymbols, string(v)); idx != -1 {
				if width >= boardSize {
					return nil, fmt.Errorf("invalid FEN %q: rank %q has more than 8 squares", FEN, row)
				}
				b.pieces[idx] |= mask
				mask >>= 1
				width++
				continue
			}
			if v < '1' || v > '8' {
				return nil, fmt.Errorf("invalid FEN %q: unexpected %q in piece placement", FEN, v)
			}
			empty := int(v - '0')
			if width+empty > boardSize {
				return nil, fmt.Errorf("invalid FEN %q: rank %q has more than 8 squares", FEN, row)
			}
			mask >>= empty
			width += empty
		}
		if width != boardSize {
			return nil, fmt.Errorf("invalid FEN %q: rank %q does not have 8 squares", FEN, row)
		}
	}
	if mask != 0 {
		return nil, fmt.Errorf("invalid FEN %q: piece placement is invalid", FEN)
	}

	if bits.OnesCount64(uint64(b.pieces[King])) != 1 {
		return nil, fmt.Errorf("invalid FEN %q: white must have exactly one king", FEN)
	}
	if bits.OnesCount64(uint64(b.pieces[King+blackPieceOffset])) != 1 {
		return nil, fmt.Errorf("invalid FEN %q: black must have exactly one king", FEN)
	}

	for _, side := range []struct {
		name   string
		offset int
	}{{"white", 0}, {"black", blackPieceOffset}} {
		count := 0
		for piece := Pawn; piece <= King; piece++ {
			count += bits.OnesCount64(uint64(b.pieces[int(piece)+side.offset]))
		}
		if count > 16 {
			return nil, fmt.Errorf("invalid FEN %q: %s has more than 16 pieces", FEN, side.name)
		}
		if pawns := bits.OnesCount64(uint64(b.pieces[int(Pawn)+side.offset])); pawns > 8 {
			return nil, fmt.Errorf("invalid FEN %q: %s has %d pawns, at most 8 are possible", FEN, side.name, pawns)
		}
	}

	const lastRank BitBoard = rank1Mask << 56
	for _, offset := range []int{0, blackPieceOffset} {
		if b.pieces[int(Pawn)+offset]&(rank1Mask|lastRank) != 0 {
			return nil, fmt.Errorf("invalid FEN %q: pawns cannot be on the first or last rank", FEN)
		}
	}

	b.encoding = 0

	// Players turn.
	switch fields[1] {
	case "w":
		b.encoding |= whiteToMove
	case "b":
	default:
		return nil, fmt.Errorf("invalid FEN %q: invalid side to move %q", FEN, fields[1])
	}

	// Castling rights.
	castling := fields[2]
	if castling != "-" {
		seen := make(map[rune]bool, len(castling))
		for _, v := range castling {
			if !strings.ContainsRune("KQkq", v) {
				return nil, fmt.Errorf("invalid FEN %q: invalid castling right %q", FEN, v)
			}
			if seen[v] {
				return nil, fmt.Errorf("invalid FEN %q: duplicate castling right %q", FEN, v)
			}
			seen[v] = true
		}
	}
	for i, v := range castlingSymbols {
		if strings.Contains(castling, v) {
			b.encoding |= 1 << (i + 1)
		}
	}

	// En passant target.
	if fields[3] == "-" {
		b.enPassantSquare = 0
	} else {
		row, col, err := ParseFileRank(fields[3])
		if err != nil {
			return nil, fmt.Errorf("invalid FEN %q: en passant square: %w", FEN, err)
		}
		if row != 2 && row != 5 {
			return nil, fmt.Errorf("invalid FEN %q: en passant square %q must be on rank 3 or 6", FEN, fields[3])
		}
		b.enPassantSquare = BitBoard(1) << SquareFromFileRank(FileRank{row: row, col: col})
	}

	// Halfmove clock.
	v, err := strconv.Atoi(fields[4])
	if err != nil {
		return nil, fmt.Errorf("invalid FEN %q: halfmove clock %q is not a number", FEN, fields[4])
	}
	if v < 0 || v > 65535 {
		return nil, fmt.Errorf("invalid FEN %q: halfmove clock %d is out of range", FEN, v)
	}
	b.halfmove_clock = uint16(v)

	// Fullmove number. Standard FEN numbering starts at 1.
	v, err = strconv.Atoi(fields[5])
	if err != nil {
		return nil, fmt.Errorf("invalid FEN %q: fullmove number %q is not a number", FEN, fields[5])
	}
	if v < 1 || v > 65535 {
		return nil, fmt.Errorf("invalid FEN %q: fullmove number %d is out of range", FEN, v)
	}
	b.fullmove_number = uint16(v)

	return &b, nil
}

// stateString returns the FEN fields that follow the piece placement.
func (b *Position) stateString() string {
	var buffer bytes.Buffer
	//Turn information
	buffer.WriteRune(' ')
	if b.encoding&whiteToMove > 0 {
		buffer.WriteRune('w')
	} else {
		buffer.WriteRune('b')
	}

	//Castle Information
	buffer.WriteRune(' ')
	canCastle := false
	for i, v := range castlingSymbols {
		if b.encoding&(1<<(i+1)) > 0 {
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
		buffer.WriteString(AlgFromLoc(b.enPassantSquare))
	} else {
		buffer.WriteRune('-')
	}

	buffer.WriteRune(' ')
	buffer.WriteString(strconv.Itoa(int(b.halfmove_clock)))

	buffer.WriteRune(' ')
	buffer.WriteString(strconv.Itoa(int(b.fullmove_number)))

	return buffer.String()
}

func (b *Position) format(symbols []string) string {
	boardStr := ""
	for row := 7; row >= 0; row-- {
		for col := 0; col < 8; col++ {
			mask := BitBoard(1) << SquareFromFileRank(FileRank{row: uint64(row), col: uint64(col)})
			s := " _ "
			for k, p := range b.pieces {
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

func (b *Position) UnicodeString() string {
	return b.format(unicodePieceSymbols)
}

func (b *Position) FEN() string {
	var buffer bytes.Buffer

	for row := 7; row >= 0; row-- {
		emptyCount := 0
		for col := 0; col < 8; col++ {
			mask := BitBoard(1) << SquareFromFileRank(FileRank{row: uint64(row), col: uint64(col)})

			// Determine whether a piece occupies the square.
			found := false
			for i, v := range b.pieces {
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
				emptyCount += 1
			}

		}
		if emptyCount != 0 {
			buffer.WriteString(strconv.Itoa(emptyCount))
		}
		if row != 0 {
			buffer.WriteRune('/')
		}
	}
	buffer.WriteString(b.stateString())
	return buffer.String()
}

// LegalMoves returns all legal moves from the current position. Move
// generation is not implemented yet, so it returns nil.
func (b *Position) LegalMoves() []Move {
	return nil //TODO: get this working
}

// Occupancy returns a mask of every occupied square for the given colour.
// Colour should be White, Black, or Both.
func (b *Position) Occupancy(colour Colour) BitBoard {
	var occupied BitBoard = 0
	for i := range PieceTypes(colour) {
		occupied |= b.pieces[i]
	}
	return occupied
}

// PieceBitboard returns the bitboard for one piece type and colour.
func (b *Position) PieceBitboard(colour Colour, piece Piece) BitBoard {
	if colour == Both || piece == NoPiece {
		panic("Invalid input in PieceBitboard, cannot be Both or NoPiece.")
	}

	var offset int
	if colour == White {
		offset = 0
	} else {
		offset = blackPieceOffset
	}

	return b.pieces[int(piece)+offset]
}
