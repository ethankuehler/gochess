package chess

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestNewMove(t *testing.T) {
	for i := range 8 {
		for j := range 8 {
			for k := range 8 {
				for l := range 8 {
					scol := fileNames[i]
					srow := j + 1
					ecol := fileNames[k]
					erow := l + 1
					uci := fmt.Sprintf("%c%d%c%d", scol, srow, ecol, erow)
					if uci[:2] == uci[2:] {
						continue // same-square moves are invalid
					}
					m, err := ParseUCIMove(uci)
					if err != nil {
						t.Errorf("Not able to create new move, UCI=%s, error = %s", uci, err.Error())
						continue
					}
					new_uci := m.String()
					if new_uci != uci {
						t.Errorf("UCI's did not match, input = %s, output = %s", uci, new_uci)
					}
					from, _ := ParseSquare(uci[:2])
					to, _ := ParseSquare(uci[2:])
					if m.From() != from || m.To() != to {
						t.Errorf("%s: From/To = %d/%d, want %d/%d", uci, m.From(), m.To(), from, to)
					}
					if _, ok := m.Promotion(); ok {
						t.Errorf("%s: non-promotion reported as a promotion", uci)
					}
				}
			}
		}
	}
}

func TestNewMovePromotions(t *testing.T) {
	cases := []struct {
		uci  string
		want Piece
	}{
		{"e7e8q", Queen},
		{"e7e8r", Rook},
		{"e7e8b", Bishop},
		{"e7e8n", Knight},
		{"a7a8q", Queen},
		{"e7e8Q", Queen},
	}

	for _, tc := range cases {
		t.Run(tc.uci, func(t *testing.T) {
			m, err := ParseUCIMove(tc.uci)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := m.String(); got != strings.ToLower(tc.uci) {
				t.Errorf("String() = %q, want %q", got, strings.ToLower(tc.uci))
			}
			got, ok := m.Promotion()
			if !ok {
				t.Fatal("Promotion() reported no promotion")
			}
			if got != tc.want {
				t.Errorf("Promotion() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestNewMoveRejectsInvalid(t *testing.T) {
	for _, uci := range []string{
		"", "e2", "e2e", "e2e44", "e2e2", "e2e9", "i2e4",
		"e7e8k", "e7e8p", "e7e8x", "e7e8qq",
	} {
		t.Run(strconv.Quote(uci), func(t *testing.T) {
			if _, err := ParseUCIMove(uci); err == nil {
				t.Errorf("ParseUCIMove(%q) succeeded, want an error", uci)
			}
		})
	}
}

func TestRowColFromAlg(t *testing.T) {
	cases := []struct {
		alg      string
		row, col uint64
	}{
		{"a1", 0, 0},
		{"h8", 7, 7},
		{"d4", 3, 3},
	}

	for _, tc := range cases {
		row, col, err := ParseFileRank(tc.alg)
		if err != nil {
			t.Errorf("ParseFileRank(%q) returned error: %v", tc.alg, err)
			continue
		}
		if row != tc.row || col != tc.col {
			t.Errorf("ParseFileRank(%q) = (%d, %d), want (%d, %d)", tc.alg, row, col, tc.row, tc.col)
		}
	}

	for _, alg := range []string{"", "e", "e44", "i1", "e0", "e9", " 1"} {
		t.Run(strconv.Quote(alg), func(t *testing.T) {
			if _, _, err := ParseFileRank(alg); err == nil {
				t.Errorf("ParseFileRank(%q) succeeded, want an error", alg)
			}
		})
	}
}
