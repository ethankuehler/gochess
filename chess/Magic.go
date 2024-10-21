package chess

var (
	ROOK_MAGIC      []MagicEntry
	BISHOP_MAGIC    []MagicEntry
	ROOK_ATTTACKS   []uint64
	BISHOPE_ATTACKS []uint64
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
