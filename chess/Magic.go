package chess

// there are only 64 knight moves on a chess board
// each key is the location of the knight, the value is the attack
var KNIGHT_ATTACKS map[uint64]uint64

// pawns are split up into attacks and move's
var PAWN_ATTACKS map[uint64]uint64
var PAWN_MOVES map[uint64]uint64

// sliding piececs
var (
	ROOK_MAGIC     []MagicEntry
	BISHOP_MAGIC   []MagicEntry
	ROOK_ATTTACKS  []uint64
	BISHOP_ATTACKS []uint64
)

type MagicEntry struct {
	Mask  uint64
	Magic uint64
	Index uint8
}

func MagicIndex(entry MagicEntry, board uint64) uint64 {
	blockers := board & entry.Mask
	hash := blockers * entry.Magic
	index := hash >> (64 - entry.Index)
	return index
}

func GetRookAttack(location uint64, blockers uint64) uint64 {
	//magic := ROOK_MAGIC[location]
	//attacks := ROOK_ATTTACKS[location]
	return 0 // TODO: finish
}

func BuildKnightAttacks() {
	KNIGHT_ATTACKS = make(map[uint64]uint64)
	start, _ := ShiftFromAlg("a1")
	end, _ := ShiftFromAlg("h8")
	for i := start; i <= end; i++ {
		var loc uint64 = 1 << i
		mask_shift := i - KNIGHT_OFFSET
		if mask_shift > 0 {
			KNIGHT_ATTACKS[loc] = KNIGHT_MASK << mask_shift
		} else {
			KNIGHT_ATTACKS[loc] = KNIGHT_MASK >> -mask_shift
		}
	}
}

// TODO: deal with black, so far these only work for white pawns
func BuildPawnMoves() {
	PAWN_MOVES = make(map[uint64]uint64)
	//pawns on the 2nd rank move twice
	start, _ := ShiftFromAlg("a2")
	stop, _ := ShiftFromAlg("h2")
	for i := start; i <= stop; i++ {
		PAWN_MOVES[1<<i] = PAWN_MOVE_MASK2 << i
	}

	//rest of the pawns are normal
	start, _ = ShiftFromAlg("a3")
	stop, _ = ShiftFromAlg("h7")
	for i := start; i <= stop; i++ {
		PAWN_MOVES[1<<i] = PAWN_MOVE_MASK << i
	}

}

func BuildPawnAttacks() {
	PAWN_ATTACKS = make(map[uint64]uint64)
	start, _ := ShiftFromAlg("a2")
	stop, _ := ShiftFromAlg("h7")
	for i := start; i <= stop; i++ {
		loc := uint64(1 << i)
		shift := (i - PAWN_ATTACK_MASK_OFFSET)
		PAWN_ATTACKS[loc] = PAWN_ATTACK_MASK << shift
	}
}
