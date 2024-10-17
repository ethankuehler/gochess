package chess

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// index for certin pieces.
const PAWN = 0
const BISHOP = 1
const KNIGHT = 2
const ROOK = 3
const QUEEN = 4
const KING = 5
const BLACK_OFFSET = 6

// mask for game encoding.
const TURN_MASK uint8 = 1
const WHITEOO_MASK uint8 = 1 << 1
const WHITEOOO_MASK uint8 = 1 << 2
const BLACKOO_MASK uint8 = 1 << 3
const BLACKOOO_MASK uint8 = 1 << 4

// precalculated positional masking.
const ROW_MASK uint64 = 255
const COLONM_MASK uint64 = 72340172838076673

// string information for formating and covnerting different chess notations.
var PICECES_SYM = []string{"P", "B", "N", "R", "Q", "K", "p", "b", "n", "r", "q", "k"}
var CASTLE_SYM = []string{"K", "Q", "k", "q"}
var COLONMS = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g'}

// Given algerbraic notation for a position (e.g. c5) calculate the position.
func CalcLocFromAlg(alg string) (uint64, error) {
	col := slices.Index(COLONMS, rune(alg[0]))
	if col == -1 {
		s := fmt.Sprintf("Invalid algerbraic notation %s", alg)
		return 0, errors.New(s)
	}

	row := int(alg[1]-'0') - 1
	if row < 0 || row >= 8 {
		s := fmt.Sprintf("Invalid algerbraic notation %s", alg)
		return 0, errors.New(s)
	}

	return 1 << (col + row*8), nil
}

type BitBoard struct {
	pieces          [12]uint64 //BitBoard, encoding for all piceces on board.
	encoding        uint8      //Encoding for castle and turn information.
	halfmove_clock  uint16     //Number of half moves since last pawn advance or piece capture, for 50 move rule.
	fullmove_number uint16     //Number of full moves.
}

func NewBoardDefault() *BitBoard {
	b := BitBoard{}
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
func NewBoardFEN(FEN string) (*BitBoard, error) {
	b := BitBoard{}

	fields := strings.Fields(FEN)

	if len(fields) != 6 {
		return nil, errors.New("Invalid FEN.")
	}

	var loc uint64 = 1 << 63
	for _, v := range fields[0] {
		if v == '/' {
			continue
		}
		idx := slices.Index(PICECES_SYM, string(v))
		if idx != -1 {
			b.pieces[idx] |= loc
			loc = loc >> 1
		} else {
			loc = loc >> (v - 48)
		}
	}
	if loc != 0 {
		return nil, errors.New("Invald FEN, piece placement invalid.")
	}

	b.encoding = 0

	//players turn
	if fields[1] == "w" {
		b.encoding |= TURN_MASK
	} else if fields[1] != "b" {
		return nil, errors.New("Invald FEN, invalid turn")
	}

	//castling
	castle_info := fields[2]
	for i, v := range CASTLE_SYM {
		if strings.Contains(castle_info, v) {
			b.encoding |= 1 << i
		}
	}

	//TODO: deal with en passant and fields[3]

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

// returns a string showing the location of every piece on the bord
func (b BitBoard) String() string {
	boardStr := ""
	//we start at the top right
	var loc uint64 = 1 << 63

	for i := range 64 + 8 {
		//insert a newline at the end of every row.
		if i%9 == 0 {
			boardStr += "\n"
			continue
		}

		//Now find if there is a piece at the location loc and write it to s which by default is " _ ".
		s := " _ "
		for k, p := range b.pieces {
			if loc&p > 0 {
				s = " " + PICECES_SYM[k] + " "
				break
			}
		}
		boardStr += s

		loc = loc >> 1
	}

	//Add castle information to string
	boardStr += "\n"
	for i, v := range CASTLE_SYM {
		if b.encoding&(1<<i) > 0 {
			boardStr += v
		}
	}

	//en passant
	boardStr += " " + "-" + " " //TODO: en passant

	//move timer
	boardStr += fmt.Sprintf("%d", b.halfmove_clock) + " " + fmt.Sprintf("%d", b.fullmove_number)

	return boardStr
}

// returns a list of all legal moves from a current baord position
func (b BitBoard) LegalMoves() []Move {

	return make([]Move, 1)
}
