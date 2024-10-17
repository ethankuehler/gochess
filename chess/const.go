package chess

// index for certin pieces.
const (
	PAWN         = 0
	BISHOP       = 1
	KNIGHT       = 2
	ROOK         = 3
	QUEEN        = 4
	KING         = 5
	BLACK_OFFSET = 6
)

// mask for game encoding.
const (
	TURN_MASK     uint8 = 1
	WHITEOO_MASK  uint8 = 1 << 1
	WHITEOOO_MASK uint8 = 1 << 2
	BLACKOO_MASK  uint8 = 1 << 3
	BLACKOOO_MASK uint8 = 1 << 4
)

// precalculated positional masking.
const (
	ROW_MASK    uint64 = 255
	COLONM_MASK uint64 = 72340172838076673
)

// string information for formating and covnerting different chess notations.
var (
	PICECES_SYM = []string{"P", "B", "N", "R", "Q", "K", "p", "b", "n", "r", "q", "k"}
	CASTLE_SYM  = []string{"K", "Q", "k", "q"}
	COLONMS     = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g'}
)
