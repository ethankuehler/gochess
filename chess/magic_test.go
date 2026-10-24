package chess

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPawnAttacks(t *testing.T) {
	BuildPawnAttacks()

	if whitePawnAttacks == nil {
		t.Error("nil map")
	}

	if len(whitePawnAttacks) != 64 {
		t.Errorf("map incorrect size, 64 != %d", len(whitePawnAttacks))
	}
}

func TestPawnMoves(t *testing.T) {
	BuildPawnMoves()

	if whitePawnMoves == nil {
		t.Error("White nil map")
	}

	if len(whitePawnMoves) != 64 {
		t.Errorf("White map incorrect size, 64 != %d", len(whitePawnAttacks))
	}

	if blackPawnMoves == nil {
		t.Error("Black nil map")
	}

	if len(blackPawnMoves) != 64 {
		t.Errorf("Black map incorrect size, 64 != %d", len(whitePawnAttacks))
	}

	records, err := readCSV("data/white_pawn_move.csv")
	if err != nil {
		t.Fatalf("could not open csv %v", err)
	}

	for _, record := range records[1:] {
		val, err := readRecord(record)
		if err != nil {
			t.Fatalf("could not convert record %v", err)
		}
		if whitePawnMoves[val[0]] != BitBoard(val[2]) {
			t.Errorf("white move did not match gen=%d actual=%d", whitePawnMoves[val[0]], val[2])
		}
	}

	records, err = readCSV("data/black_pawn_move.csv")
	if err != nil {
		t.Fatalf("could not open csv %v", err)
	}

	for _, record := range records[1:] {
		val, err := readRecord(record)
		if err != nil {
			t.Fatalf("could not convert record %v", err)
		}
		if blackPawnMoves[val[0]] != BitBoard(val[2]) {
			t.Errorf("black move did not match gen=%d actual=%d", blackPawnMoves[val[0]], val[2])
		}
	}
}

func slidingAttacksReference(start Square, blockers BitBoard, directions Directions) BitBoard {
	startRow := int(start) / boardSize
	startCol := int(start) % boardSize
	var attacks BitBoard = 0

	for _, direction := range directions {
		dRank, dFile := direction[0], direction[1]
		if dRank == 0 && dFile == 0 {
			continue
		}
		rank, file := startRow+dRank, startCol+dFile
		for rank >= 0 && rank < boardSize && file >= 0 && file < boardSize {
			loc := BitBoard(1) << Square(file+rank*boardSize)
			attacks |= loc
			if blockers&loc > 0 {
				break
			}
			rank += dRank
			file += dFile
		}
	}

	return attacks
}

type slidingAttacksExample struct {
	Name       string   `json:"name"`
	Start      string   `json:"start"`
	Blockers   []string `json:"blockers"`
	Expected   []string `json:"expected"`
	Directions string   `json:"directions"`
}

func loadSlidingAttacksExamples(path string) ([]slidingAttacksExample, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	tests := make([]slidingAttacksExample, 0)
	if err := json.Unmarshal(data, &tests); err != nil {
		return nil, err
	}
	return tests, nil
}

func parseDirections(directions string) (Directions, error) {
	switch directions {
	case "rook":
		return rookDirections, nil
	case "bishop":
		return bishopDirections, nil
	default:
		return Directions{}, fmt.Errorf("invalid directions %q", directions)
	}
}

func TestSlidingAttacksExamples(t *testing.T) {
	tests, err := loadSlidingAttacksExamples("data/sliding_attacks_examples.json")
	if err != nil {
		t.Fatalf("could not load raycast examples: %v", err)
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			directions, err := parseDirections(test.Directions)
			if err != nil {
				t.Fatal(err)
			}
			start, err := ParseSquare(test.Start)
			if err != nil {
				t.Fatal(err)
			}
			blockers, err := SquaresToBitBoard(test.Blockers)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := SquaresToBitBoard(test.Expected)
			if err != nil {
				t.Fatal(err)
			}
			got := SlidingAttacks(start, blockers, directions)

			if got != expected {
				t.Errorf("raycast mismatch\nexpected:\n%s\ngot:\n%s", expected.String(), got.String())
			}
		})
	}
}

func TestSlidingAttacksReferenceCoverage(t *testing.T) {
	fullBoard := BitBoard(^uint64(0))

	for start := Square(0); start < Square(squareCount); start++ {
		source := BitBoard(1) << start
		occupancies := []BitBoard{0, fullBoard &^ source}
		for blocker := Square(0); blocker < Square(squareCount); blocker++ {
			if blocker == start {
				continue
			}
			occupancies = append(occupancies, BitBoard(1)<<blocker)
		}

		for _, blockers := range occupancies {
			rookExpected := slidingAttacksReference(start, blockers, rookDirections)
			rookGot := SlidingAttacks(start, blockers, rookDirections)
			if rookGot != rookExpected {
				t.Fatalf("rook mismatch at start=%d blockers=%d", start, blockers)
			}
			if rookGot&source != 0 {
				t.Fatalf("rook attack includes source square at start=%d", start)
			}

			bishopExpected := slidingAttacksReference(start, blockers, bishopDirections)
			bishopGot := SlidingAttacks(start, blockers, bishopDirections)
			if bishopGot != bishopExpected {
				t.Fatalf("bishop mismatch at start=%d blockers=%d", start, blockers)
			}
			if bishopGot&source != 0 {
				t.Fatalf("bishop attack includes source square at start=%d", start)
			}
		}
	}
}

func TestSlidingAttacksWithEmptyDirections(t *testing.T) {
	start, _ := ParseSquare("d4")
	blockers, _ := SquaresToBitBoard([]string{"d5", "e4", "e5"})
	got := SlidingAttacks(start, blockers, Directions{})
	if got != 0 {
		t.Fatalf("expected no attacks for empty directions, got:\n%s", got.String())
	}
}

func TestRelevantBlockers(t *testing.T) {
	cases := []struct {
		square     string
		directions Directions
		bits       int
	}{
		{"a1", rookDirections, 12},
		{"a4", rookDirections, 11},
		{"d4", rookDirections, 10},
		{"h8", rookDirections, 12},
		{"a1", bishopDirections, 6},
		{"d4", bishopDirections, 9},
		{"h8", bishopDirections, 6},
	}

	for _, c := range cases {
		square, err := ParseSquare(c.square)
		if err != nil {
			t.Fatalf("bad square %q: %v", c.square, err)
		}
		mask := RelevantBlockers(FileRankFromSquare(square), c.directions)
		if got := bits.OnesCount64(uint64(mask)); got != c.bits {
			t.Errorf("%s: expected %d relevant bits, got %d", c.square, c.bits, got)
		}
		if mask&(BitBoard(1)<<square) != 0 {
			t.Errorf("%s: mask includes origin square", c.square)
		}
	}
}

func TestMagicAttacksAgainstSlidingAttacks(t *testing.T) {
	BuildRookAttacks()
	BuildBishopAttacks()

	rng := rand.New(rand.NewPCG(1, 2))
	for square := Square(0); square < squareCount; square++ {
		for range 200 {
			occupancy := BitBoard(rng.Uint64())
			rookWant := SlidingAttacks(square, occupancy, rookDirections)
			if got := RookAttack(square, occupancy); got != rookWant {
				t.Fatalf("rook mismatch at square=%d\nexpected:\n%s\ngot:\n%s", square, rookWant.String(), got.String())
			}
			bishopWant := SlidingAttacks(square, occupancy, bishopDirections)
			if got := BishopAttack(square, occupancy); got != bishopWant {
				t.Fatalf("bishop mismatch at square=%d\nexpected:\n%s\ngot:\n%s", square, bishopWant.String(), got.String())
			}
		}
	}
}

func TestMagicsSaveLoadRoundTrip(t *testing.T) {
	BuildRookAttacks()

	path := filepath.Join(t.TempDir(), "rook_magics.csv")
	if err := SaveMagicEntries(path, rookMagicEntries); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadMagicEntries(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded) != len(rookMagicEntries) {
		t.Fatalf("loaded %d entries, expected %d", len(loaded), len(rookMagicEntries))
	}
	for i := range rookMagicEntries {
		if loaded[i] != rookMagicEntries[i] {
			t.Errorf("square %d: loaded %+v != generated %+v", i, loaded[i], rookMagicEntries[i])
		}
	}
}

func TestBuildMagicsFromDisk(t *testing.T) {
	previous := LoadMagicsFromDisk
	LoadMagicsFromDisk = true
	t.Cleanup(func() { LoadMagicsFromDisk = previous })

	BuildRookAttacks()
	BuildBishopAttacks()

	rng := rand.New(rand.NewPCG(3, 4))
	for square := Square(0); square < squareCount; square++ {
		for range 10 {
			occupancy := BitBoard(rng.Uint64())
			if got, want := RookAttack(square, occupancy), SlidingAttacks(square, occupancy, rookDirections); got != want {
				t.Fatalf("disk-loaded rook mismatch at square=%d\nexpected:\n%s\ngot:\n%s", square, want.String(), got.String())
			}
			if got, want := BishopAttack(square, occupancy), SlidingAttacks(square, occupancy, bishopDirections); got != want {
				t.Fatalf("disk-loaded bishop mismatch at square=%d\nexpected:\n%s\ngot:\n%s", square, want.String(), got.String())
			}
		}
	}
}

func TestSaveMagicsToDiskFlag(t *testing.T) {
	previousLoad, previousSave := LoadMagicsFromDisk, SaveMagicsToDisk
	LoadMagicsFromDisk = false
	SaveMagicsToDisk = true
	t.Cleanup(func() {
		LoadMagicsFromDisk = previousLoad
		SaveMagicsToDisk = previousSave
	})

	path := filepath.Join(t.TempDir(), "bishop_magics.csv")
	magics, _ := buildSliderAttacks(bishopDirections, path)

	loaded, err := LoadMagicEntries(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for i := range magics {
		if loaded[i] != magics[i] {
			t.Errorf("square %d: saved %+v != generated %+v", i, loaded[i], magics[i])
		}
	}
}

func TestMagicsLoadFromDiskByDefault(t *testing.T) {
	if !LoadMagicsFromDisk {
		t.Fatal("LoadMagicsFromDisk should default to true")
	}
	previous := LoadMagicsFromDisk
	t.Cleanup(func() { LoadMagicsFromDisk = previous })

	cases := []struct {
		name  string
		file  string
		build func()
		get   func() []MagicEntry
	}{
		{"rook", "data/rook_magics.csv", BuildRookAttacks, func() []MagicEntry { return rookMagicEntries }},
		{"bishop", "data/bishop_magics.csv", BuildBishopAttacks, func() []MagicEntry { return bishopMagicEntries }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.build()
			disk, err := LoadMagicEntries(tc.file)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			built := tc.get()
			if len(built) != len(disk) {
				t.Fatalf("built %d entries, disk has %d", len(built), len(disk))
			}
			for i := range disk {
				if built[i] != disk[i] {
					t.Fatalf("square %d: built %+v != disk %+v", i, built[i], disk[i])
				}
			}
		})
	}
}

func TestBuildSliderAttacksFallsBackWhenMissing(t *testing.T) {
	previousLoad, previousSave := LoadMagicsFromDisk, SaveMagicsToDisk
	LoadMagicsFromDisk = true
	SaveMagicsToDisk = false
	t.Cleanup(func() {
		LoadMagicsFromDisk = previousLoad
		SaveMagicsToDisk = previousSave
	})

	missing := filepath.Join(t.TempDir(), "missing.csv")
	magics, tables := buildSliderAttacks(bishopDirections, missing)
	if len(magics) != squareCount || len(tables) != squareCount {
		t.Fatalf("expected %d entries, got %d magics and %d tables", squareCount, len(magics), len(tables))
	}

	occupancy := BitBoard(0x00FF000000FF0000)
	for square := Square(0); square < squareCount; square++ {
		want := SlidingAttacks(square, occupancy, bishopDirections)
		got := tables[square][magicIndex(magics[square], occupancy)]
		if got != want {
			t.Fatalf("square %d: fallback attack mismatch\nexpected:\n%s\ngot:\n%s", square, want.String(), got.String())
		}
	}
}

func TestLoadMagicEntriesRejectsBadCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.csv")
	if err := os.WriteFile(path, []byte("square,mask,magic\n0,1,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMagicEntries(path); err == nil {
		t.Fatal("expected error for a truncated magic CSV")
	}

	var b strings.Builder
	b.WriteString("square,mask,magic\n")
	for i := 0; i < squareCount; i++ {
		if i == 7 {
			fmt.Fprintf(&b, "%d,1,not-a-number\n", i)
			continue
		}
		fmt.Fprintf(&b, "%d,1,%d\n", i, i+1)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMagicEntries(path); err == nil {
		t.Fatal("expected error for a non-numeric magic")
	}
}

func TestQueenAttack(t *testing.T) {
	BuildRookAttacks()
	BuildBishopAttacks()

	rng := rand.New(rand.NewPCG(5, 6))
	for square := Square(0); square < squareCount; square++ {
		for range 25 {
			occupancy := BitBoard(rng.Uint64())
			want := SlidingAttacks(square, occupancy, rookDirections) | SlidingAttacks(square, occupancy, bishopDirections)
			if got := QueenAttack(square, occupancy); got != want {
				t.Fatalf("queen mismatch at square=%d\nexpected:\n%s\ngot:\n%s", square, want.String(), got.String())
			}
		}
	}
}

func TestBuildAllAttacks(t *testing.T) {
	BuildAllAttacks()

	tables := map[string][]BitBoard{
		"knight":             knightAttacks,
		"king":               kingAttacks,
		"white pawn attacks": whitePawnAttacks,
		"white pawn moves":   whitePawnMoves,
		"black pawn attacks": blackPawnAttacks,
		"black pawn moves":   blackPawnMoves,
	}
	for name, table := range tables {
		if len(table) != squareCount {
			t.Errorf("%s table has %d entries, want %d", name, len(table), squareCount)
		}
	}

	if len(rookMagicEntries) != squareCount || len(rookAttackTables) != squareCount {
		t.Errorf("rook tables have %d magics and %d attack tables", len(rookMagicEntries), len(rookAttackTables))
	}
	if len(bishopMagicEntries) != squareCount || len(bishopAttackTables) != squareCount {
		t.Errorf("bishop tables have %d magics and %d attack tables", len(bishopMagicEntries), len(bishopAttackTables))
	}
}
