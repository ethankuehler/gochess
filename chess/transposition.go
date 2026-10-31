package chess

import "unsafe"

const (
	DefaultHashMB = 16
	MinHashMB     = 1
	MaxHashMB     = 1024

	maxTTDepth = 127

	// The Zobrist key excludes clocks, so disable the table near fifty-move draws.
	ttMaxHalfmoveClock = 80
)

const mateScoreTTThreshold = MateScore - maxPly

type ttBound uint8

const (
	ttExact ttBound = iota
	ttLower
	ttUpper
)

type ttEntry struct {
	key   uint64
	move  Move
	score int32
	depth int8
	bound ttBound
	age   uint8
}

const ttEntryBytes = int(unsafe.Sizeof(ttEntry{}))

type TTStats struct {
	Probes  uint64 `json:"probes"`
	Hits    uint64 `json:"hits"`
	Cutoffs uint64 `json:"cutoffs"`
	Stores  uint64 `json:"stores"`
}

// TranspositionTable is single-threaded.
type TranspositionTable struct {
	entries []ttEntry
	mask    uint64
	age     uint8
}

func NewTranspositionTable(sizeMB int) *TranspositionTable {
	entries := ttEntryCount(sizeMB)
	return &TranspositionTable{
		entries: make([]ttEntry, entries),
		mask:    uint64(entries - 1),
	}
}

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

func (t *TranspositionTable) Clear() {
	if t == nil {
		return
	}
	clear(t.entries)
	t.age = 0
}

func (t *TranspositionTable) Entries() int {
	if t == nil {
		return 0
	}
	return len(t.entries)
}

// Clear on generation wrap so stale entries cannot look current.
func (t *TranspositionTable) newSearch() {
	if t == nil {
		return
	}
	t.age++
	if t.age == 0 {
		clear(t.entries)
	}
}

func (t *TranspositionTable) probe(key uint64) (ttEntry, bool) {
	if t == nil {
		return ttEntry{}, false
	}
	entry := t.entries[key&t.mask]
	return entry, entry.key == key
}

// store prefers deeper entries from the current search.
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

// Store mate distances relative to the position, not the root.
func scoreToTT(score, ply int) int32 {
	switch {
	case score >= mateScoreTTThreshold:
		score += ply
	case score <= -mateScoreTTThreshold:
		score -= ply
	}
	return int32(score)
}

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
