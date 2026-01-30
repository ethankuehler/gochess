package chess

import (
	"bytes"
	"errors"
	"slices"
	"strconv"
	"strings"
)

type BitBoard uint64

const EMPTY_BOARD BitBoard = 0

type BoardState struct {
	pieces          [12]BitBoard //BitBoard, encoding for all pieces on board.
	enpassant       BitBoard     //location of piece that can preform enpassant
	encoding        uint8        //Encoding for castle and turn information.
	halfmove_clock  uint16       //Number of half moves since last pawn advance or piece capture, for 50 move rule.
	fullmove_number uint16       //Number of full moves.
}

func (b BitBoard) String() string {
	var buffer bytes.Buffer
	var mask BitBoard = 1 << 63

	for i := range 64 + 8 {
		if i%9 == 0 {
			buffer.WriteRune('\n')
			continue
		}
		if b&mask > 0 {
			buffer.WriteString(" 1 ")
		} else {
			buffer.WriteString(" . ")
		}
		mask = mask >> 1
	}

	return buffer.String()
}

// New BitBoard with starting setup
func NewBoardDefault() *BoardState {
	b := BoardState{}
	//setting all the white pieces on the home squares
	b.pieces[PAWN] = 0b1111111100000000
	b.pieces[BISHOP] = 0b00100100
	b.pieces[KNIGHT] = 0b01000010
	b.pieces[ROOK] = 0b10000001
	b.pieces[QUEEN] = 0b00010000
	b.pieces[KING] = 0b00001000

	//copying over the piceces but now for black
	b.pieces[PAWN+BLACK_OFFSET] = b.pieces[PAWN] << 40
	b.pieces[BISHOP+BLACK_OFFSET] = b.pieces[BISHOP] << 56
	b.pieces[KNIGHT+BLACK_OFFSET] = b.pieces[KNIGHT] << 56
	b.pieces[ROOK+BLACK_OFFSET] = b.pieces[ROOK] << 56
	b.pieces[QUEEN+BLACK_OFFSET] = b.pieces[QUEEN] << 56
	b.pieces[KING+BLACK_OFFSET] = b.pieces[KING] << 56

	//default encoding at the start of a chess game.
	b.encoding |= TURN_MASK | WHITEOO_MASK | WHITEOOO_MASK | BLACKOO_MASK | BLACKOOO_MASK

	//setting all move clocks to zero
	b.halfmove_clock = 0
	b.fullmove_number = 0

	return &b
}

// Generates a bit board from a FEN notation.
func NewBoardFEN(FEN string) (*BoardState, error) {
	b := BoardState{}

	fields := strings.Fields(FEN)

	if len(fields) != 6 {
		return nil, errors.New("invalid FEN")
	}

	var mask BitBoard = 1 << 63
	for _, v := range fields[0] {
		if v == '/' {
			continue
		}
		idx := slices.Index(PICECES_SYM, string(v))
		if idx != -1 {
			b.pieces[idx] |= mask
			mask = mask >> 1
		} else {
			mask = mask >> (v - 48)
		}
	}
	if mask != 0 {
		return nil, errors.New("invald FEN, piece placement invalid")
	}

	b.encoding = 0

	//players turn
	if fields[1] == "w" {
		b.encoding |= TURN_MASK
	} else if fields[1] != "b" {
		return nil, errors.New("invald FEN, invalid turn")
	}

	//castling
	castle_info := fields[2]
	for i, v := range CASTLE_SYM {
		if strings.Contains(castle_info, v) {
			b.encoding |= 1 << (i + 1)
		}
	}

	//Enpassant
	if fields[3] == "-" {
		b.enpassant = 0
	} else {
		enpassant, err := LocFromAlg(fields[3])
		if err != nil {
			return nil, err
		}
		b.enpassant = enpassant
	}

	//turn number
	v, err := strconv.Atoi(fields[4])
	if err != nil {
		return nil, err
	}
	b.halfmove_clock = uint16(v)

	v, err = strconv.Atoi(fields[5])
	if err != nil {
		return nil, err
	}
	b.fullmove_number = uint16(v)

	return &b, nil
}

// Returns a string with turn, castle, enpassant and move number info
func (b *BoardState) InfoString() string {
	var buffer bytes.Buffer
	//Turn information
	buffer.WriteRune(' ')
	if b.encoding&TURN_MASK > 0 {
		buffer.WriteRune('w')
	} else {
		buffer.WriteRune('b')
	}

	//Castle Information
	buffer.WriteRune(' ')
	canCastle := false
	for i, v := range CASTLE_SYM {
		if b.encoding&(1<<(i+1)) > 0 {
			buffer.WriteString(v)
			canCastle = true
		}
	}
	if !canCastle {
		buffer.WriteRune('-')
	}

	//Enpassant
	buffer.WriteRune(' ')
	if b.enpassant > 0 {
		buffer.WriteString(AlgFromLoc(b.enpassant))
	} else {
		buffer.WriteRune('-')
	}

	buffer.WriteRune(' ')
	buffer.WriteString(strconv.Itoa(int(b.halfmove_clock)))

	buffer.WriteRune(' ')
	buffer.WriteString(strconv.Itoa(int(b.fullmove_number)))

	return buffer.String()
}

func (b *BoardState) toString(piceces []string) string {
	boardStr := ""
	//we start at the top right
	var mask BitBoard = 1 << 63

	for i := range 64 + 8 {
		//insert a newline at the end of every row.
		if i%9 == 0 {
			boardStr += "\n"
			continue
		}

		//Now find if there is a piece at the location loc and write it to s which by default is " _ ".
		s := " _ "
		for k, p := range b.pieces {
			if mask&p > 0 {
				s = " " + piceces[k] + " "
				break
			}
		}
		boardStr += s

		mask = mask >> 1
	}

	boardStr += b.InfoString()

	return boardStr
}

// Returns a string showing the location of every piece on the bord
func (b *BoardState) String() string {
	return b.toString(PICECES_SYM)
}

func (b *BoardState) StringUni() string {
	return b.toString(UNI_PICECES_SYM)
}

func (b *BoardState) FEN() string {
	var buffer bytes.Buffer

	row_count := 0
	count := 0
	for mask := BitBoard(1) << 63; 0 < mask; mask = mask >> 1 {

		// determins if there is a piece at the location loc
		found := false
		for i, v := range b.pieces {
			if v&mask > 0 {
				if count > 0 {
					buffer.WriteString(strconv.Itoa(count))
					count = 0
				}
				buffer.WriteString(PICECES_SYM[i])
				found = true
				break
			}
		}

		// if no piece is found
		if !found {
			count += 1
		}

		row_count += 1
		if row_count == 8 {
			if count != 0 {
				buffer.WriteString(strconv.Itoa(count))
				count = 0
			}
			if mask != 1 {
				buffer.WriteRune('/')
			}
			row_count = 0
		}
	}

	buffer.WriteString(b.InfoString())

	return buffer.String()
}

// Returns a list of all legal moves from a current baord position
func (b *BoardState) LegalMoves() []Move {
	return make([]Move, 1) //TODO: get this working
}

// Returns a mask of every Occupied sqaure on the chess board.
// Colour should be WHITE, BLACK, or BOTH.
func (b *BoardState) Occupied(colour Colour) BitBoard {
	var occupied BitBoard = 0
	for i := range PiecesIter(colour) {
		occupied |= b.pieces[i]
	}
	return occupied
}

// Returns the position of all piceces of a centrin colour and type.
func (b *BoardState) GetPieces(colour Colour, piece Piece) BitBoard {
	if colour == BOTH || piece == ALL {
		panic("Invalid input in GetPieces, cannot be BOTH or ALL.")
	}

	var offset int
	if colour == WHITE {
		offset = 0
	} else {
		offset = BLACK_OFFSET
	}

	return b.pieces[int(piece)+offset]
}
