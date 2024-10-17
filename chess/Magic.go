package chess

var (
	ROOK_MAGIC   []MagicEntry
	BISHOP_MAGIC []MagicEntry
)

type MagicEntry struct {
	Mask  uint64
	Magic uint64
	Index uint8
}
