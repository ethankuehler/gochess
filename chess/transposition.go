package chess

import "unsafe"

// Transposition table sizing. The table is allocated in mebibytes and rounded
// down to a power-of-two number of entries so that indexing is a mask.
const (
	DefaultHashMB = 16
	MinHashMB     = 1
	MaxHashMB     = 1024

	// maxTTDepth is the largest draft an int8 entry can hold. The search never
	// stores the root itself at a draft above maxPly.
	maxTTDepth = 127

	// ttMaxHalfmoveClock disables table use close to the fifty-move rule. The
	// Zobrist key deliberately excludes the move clocks, so a score found at a
	// low clock can be misleading at a high one; dropping the table near the
	// horizon is a cheap way to avoid claiming a draw that is not there.
	ttMaxHalfmoveClock = 80
)

// Mate scores in the table are stored relative to the position rather than to
// the root, so a mate found at one ply can be reused at another. Scores this
// far from zero encode forced mates; see ScoreToMate.
const mateScoreTTThreshold = MateScore - maxPly

// ttBound is the alpha-beta result type stored with an entry.
type ttBound uint8

const (
	ttExact ttBound = iota // the score is exact
	ttLower                // the true score is at least this high
	ttUpper                // the true score is at most this high
)

// ttEntry is one transposition-table slot. The full 64-bit key is stored so a
// probe can reject index collisions without relying on a shorter signature.
// The struct is 24 bytes on 64-bit platforms.
type ttEntry struct {
	key   uint64
	move  Move
	score int32
	depth int8
	bound ttBound
	age   uint8
}

const ttEntryBytes = int(unsafe.Sizeof(ttEntry{}))

// TTStats counts transposition-table activity during one search. Probes counts
// every lookup, Hits counts lookups that found the same position, Cutoffs
// counts hits that shortened the search, and Stores counts entries written.
type TTStats struct {
	Probes  uint64 `json:"probes"`
	Hits    uint64 `json:"hits"`
	Cutoffs uint64 `json:"cutoffs"`
	Stores  uint64 `json:"stores"`
}

// TranspositionTable caches search results by Zobrist key. One table is meant
// for one search at a time; it is not safe for concurrent use.
//
// Replacement is depth-preferred within a search and always-replace for
// entries left by an earlier search. That keeps deep results for the current
// iteration while preventing a long game from filling the table with stale
// positions. Age is a per-search generation counter, incremented by newSearch.
type TranspositionTable struct {
	entries []ttEntry
	mask    uint64
	age     uint8
}

// NewTranspositionTable allocates a table of approximately sizeMB mebibytes.
// The size is clamped to [MinHashMB, MaxHashMB] and rounded down to a power of
// two entries, so the usable memory can be somewhat smaller than requested.
func NewTranspositionTable(sizeMB int) *TranspositionTable {
	entries := ttEntryCount(sizeMB)
	return &TranspositionTable{
		entries: make([]ttEntry, entries),
		mask:    uint64(entries - 1),
	}
}

// ttEntryCount clamps sizeMB and rounds the slot count down to a power of two.
// It is separate from allocation so sizing can be tested without reserving the
// maximum amount of memory.
func ttEntryCount(sizeMB int) int {
	if sizeMB < MinHashMB {
		sizeMB = MinHashMB
	}
	if sizeMB > MaxHashMB {
		sizeMB = MaxHashMB
	}
	count := sizeMB * 1024 * 1024 / ttEntryBytes
	entries := 1
	for entries*2 <= count {
		entries *= 2
	}
	return entries
}

// Clear empties the table and resets its generation. It is used by ucinewgame
// and by the Clear Hash option.
func (t *TranspositionTable) Clear() {
	if t == nil {
		return
	}
	clear(t.entries)
	t.age = 0
}

// Entries reports the number of slots in the table.
func (t *TranspositionTable) Entries() int {
	if t == nil {
		return 0
	}
	return len(t.entries)
}

// newSearch starts a new generation. Entries from older generations are
// replaced even when their draft is higher. When the 8-bit generation wraps,
// the table is cleared so ancient entries cannot masquerade as current ones.
func (t *TranspositionTable) newSearch() {
	if t == nil {
		return
	}
	t.age++
	if t.age == 0 {
		clear(t.entries)
	}
}

// probe returns the entry at key's index. ok is false on an empty slot or a
// different position. Index collisions are normal; the key comparison is what
// makes a hit exact.
func (t *TranspositionTable) probe(key uint64) (ttEntry, bool) {
	if t == nil {
		return ttEntry{}, false
	}
	entry := t.entries[key&t.mask]
	return entry, entry.key == key
}

// store writes an entry using the depth-preferred replacement policy. Equal
// keys keep the deeper result; slots owned by an older search are overwritten
// regardless of depth.
func (t *TranspositionTable) store(key uint64, depth int, score int32, bound ttBound, move Move) {
	if t == nil {
		return
	}
	if depth < 0 {
		depth = 0
	}
	if depth > maxTTDepth {
		depth = maxTTDepth
	}

	entry := &t.entries[key&t.mask]
	if entry.key == key {
		if int(entry.depth) > depth {
			return
		}
	} else if entry.key != 0 && entry.age == t.age && int(entry.depth) > depth {
		return
	}

	*entry = ttEntry{
		key:   key,
		move:  move,
		score: score,
		depth: int8(depth),
		bound: bound,
		age:   t.age,
	}
}

// scoreToTT converts a search score from root-relative mate distance to a
// position-relative value. A mate in three from ply 5 and a mate in three from
// ply 9 must hash to the same value.
func scoreToTT(score, ply int) int32 {
	switch {
	case score >= mateScoreTTThreshold:
		score += ply
	case score <= -mateScoreTTThreshold:
		score -= ply
	}
	return int32(score)
}

// scoreFromTT reverses scoreToTT for the ply at which the entry is probed.
func scoreFromTT(score int32, ply int) int {
	value := int(score)
	switch {
	case value >= mateScoreTTThreshold:
		value -= ply
	case value <= -mateScoreTTThreshold:
		value += ply
	}
	return value
}
