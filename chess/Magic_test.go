package chess

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestPawnAttacks(t *testing.T) {
	BuildPawnAttacks()

	if WHITE_PAWN_ATTACKS == nil {
		t.Error("nil map")
	}

	if len(WHITE_PAWN_ATTACKS) != 64 {
		t.Errorf("map incorrect size, 64 != %d", len(WHITE_PAWN_ATTACKS))
	}
}

func TestPawnMoves(t *testing.T) {
	BuildPawnMoves()

	if WHITE_PAWN_MOVES == nil {
		t.Error("White nil map")
	}

	if len(WHITE_PAWN_MOVES) != 64 {
		t.Errorf("White map incorrect size, 64 != %d", len(WHITE_PAWN_ATTACKS))
	}

	if BLACK_PAWN_MOVES == nil {
		t.Error("Black nil map")
	}

	if len(BLACK_PAWN_MOVES) != 64 {
		t.Errorf("Black map incorrect size, 64 != %d", len(WHITE_PAWN_ATTACKS))
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
		if WHITE_PAWN_MOVES[val[0]] != BitBoard(val[2]) {
			t.Errorf("white move did not match gen=%d actual=%d", WHITE_PAWN_MOVES[val[0]], val[2])
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
		if BLACK_PAWN_MOVES[val[0]] != BitBoard(val[2]) {
			t.Errorf("black move did not match gen=%d actual=%d", BLACK_PAWN_MOVES[val[0]], val[2])
		}
	}
}

func rayCastReference(start Shift, blockers BitBoard, ray Ray) BitBoard {
	startRow := int(start) / ROW_COL_SIZE
	startCol := int(start) % ROW_COL_SIZE
	var attacks BitBoard = 0

	for _, direction := range ray {
		dRow, dCol := direction[0], direction[1]
		if dRow == 0 && dCol == 0 {
			continue
		}
		row, col := startRow+dRow, startCol+dCol
		for row >= 0 && row < ROW_COL_SIZE && col >= 0 && col < ROW_COL_SIZE {
			loc := BitBoard(1) << Shift(col+row*ROW_COL_SIZE)
			attacks |= loc
			if blockers&loc > 0 {
				break
			}
			row += dRow
			col += dCol
		}
	}

	return attacks
}

type rayCastExample struct {
	Name     string   `json:"name"`
	Start    string   `json:"start"`
	Blockers []string `json:"blockers"`
	Expected []string `json:"expected"`
	Ray      string   `json:"ray"`
}

func loadRayCastExamples(path string) ([]rayCastExample, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	tests := make([]rayCastExample, 0)
	if err := json.Unmarshal(data, &tests); err != nil {
		return nil, err
	}
	return tests, nil
}

func rayFromString(ray string) (Ray, error) {
	switch ray {
	case "rook":
		return ROOK_RAY, nil
	case "bishop":
		return BISHOP_RAY, nil
	default:
		return Ray{}, fmt.Errorf("invalid ray %q", ray)
	}
}

func TestRayCastExamples(t *testing.T) {
	tests, err := loadRayCastExamples("data/raycast_examples.json")
	if err != nil {
		t.Fatalf("could not load raycast examples: %v", err)
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			ray, err := rayFromString(test.Ray)
			if err != nil {
				t.Fatal(err)
			}
			start, err := ShiftFromAlg(test.Start)
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
			got := RayCast(start, blockers, 0, ray)

			if got != expected {
				t.Errorf("raycast mismatch\nexpected:\n%s\ngot:\n%s", expected.String(), got.String())
			}
		})
	}
}

func TestRayCastReferenceCoverage(t *testing.T) {
	fullBoard := BitBoard(^uint64(0))

	for start := Shift(0); start < Shift(SHIFT_SIZE); start++ {
		source := BitBoard(1) << start
		occupancies := []BitBoard{0, fullBoard &^ source}
		for blocker := Shift(0); blocker < Shift(SHIFT_SIZE); blocker++ {
			if blocker == start {
				continue
			}
			occupancies = append(occupancies, BitBoard(1)<<blocker)
		}

		for _, blockers := range occupancies {
			rookExpected := rayCastReference(start, blockers, ROOK_RAY)
			rookGot := RayCast(start, blockers, 0, ROOK_RAY)
			if rookGot != rookExpected {
				t.Fatalf("rook mismatch at start=%d blockers=%d", start, blockers)
			}
			if rookGot&source != 0 {
				t.Fatalf("rook attack includes source square at start=%d", start)
			}

			bishopExpected := rayCastReference(start, blockers, BISHOP_RAY)
			bishopGot := RayCast(start, blockers, 0, BISHOP_RAY)
			if bishopGot != bishopExpected {
				t.Fatalf("bishop mismatch at start=%d blockers=%d", start, blockers)
			}
			if bishopGot&source != 0 {
				t.Fatalf("bishop attack includes source square at start=%d", start)
			}
		}
	}
}

func TestRayCastWithEmptyRay(t *testing.T) {
	start, _ := ShiftFromAlg("d4")
	blockers, _ := SquaresToBitBoard([]string{"d5", "e4", "e5"})
	got := RayCast(start, blockers, 0, Ray{})
	if got != 0 {
		t.Fatalf("expected no attacks for empty ray, got:\n%s", got.String())
	}
}

type bishopMaskExample struct {
	Name     string   `json:"name"`
	Start    string   `json:"start"`
	Expected []string `json:"expected"`
}

func loadBishopMaskExamples(path string) ([]bishopMaskExample, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	tests := make([]bishopMaskExample, 0)
	if err := json.Unmarshal(data, &tests); err != nil {
		return nil, err
	}
	return tests, nil
}

func TestGetBishopMask(t *testing.T) {
	tests, err := loadBishopMaskExamples("data/bishop_mask_examples.json")
	if err != nil {
		t.Fatalf("could not load bishop mask examples: %v", err)
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			shift, err := ShiftFromAlg(test.Start)
			if err != nil {
				t.Fatalf("bad start square %q: %v", test.Start, err)
			}

			mask := GetBishopMask(CoordsFromShift(shift))

			expected, err := SquaresToBitBoard(test.Expected)
			if err != nil {
				t.Fatalf("bad expected squares %v: %v", test.Expected, err)
			}

			if mask != expected {
				t.Errorf("bishop mask mismatch for %s\nexpected:\n%s\ngot:\n%s", test.Start, expected.String(), mask.String())
			}

			source := BitBoard(1) << shift
			if mask&source != 0 {
				t.Errorf("source square %s should not be in bishop mask", test.Start)
			}
		})
	}
}
