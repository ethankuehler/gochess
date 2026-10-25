package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/data"
)

func TestMain(m *testing.M) {
	if err := chess.BuildAllAttacksFromFS(data.FS); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestEloDiff(t *testing.T) {
	if got := eloDiff(0.5); got != 0 {
		t.Errorf("eloDiff(0.5) = %v, want 0", got)
	}
	if got := eloDiff(0.75); math.Abs(got-190.848) > 0.01 {
		t.Errorf("eloDiff(0.75) = %v, want 190.85", got)
	}
	if got := eloDiff(0.25); math.Abs(got+190.848) > 0.01 {
		t.Errorf("eloDiff(0.25) = %v, want -190.85", got)
	}
	if got := eloDiff(0); !math.IsInf(got, -1) {
		t.Errorf("eloDiff(0) = %v, want -Inf", got)
	}
	if got := eloDiff(1); !math.IsInf(got, 1) {
		t.Errorf("eloDiff(1) = %v, want +Inf", got)
	}
}

func TestOurScore(t *testing.T) {
	cases := []struct {
		result   string
		ourWhite bool
		want     float64
	}{
		{"1-0", true, 1},
		{"1-0", false, 0},
		{"0-1", true, 0},
		{"0-1", false, 1},
		{"1/2-1/2", true, 0.5},
		{"1/2-1/2", false, 0.5},
	}
	for _, tc := range cases {
		if got := ourScore(tc.result, tc.ourWhite); got != tc.want {
			t.Errorf("ourScore(%q, %v) = %v, want %v", tc.result, tc.ourWhite, got, tc.want)
		}
	}
}

func TestBootstrapEloEven(t *testing.T) {
	elo, low, high := bootstrapElo([]float64{1, 1, 1, 1}, 7)
	if elo != 0 || low != 0 || high != 0 {
		t.Errorf("bootstrapElo(even) = %v [%v, %v], want 0 [0, 0]", elo, low, high)
	}
}

func writeBook(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "openings.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write book: %v", err)
	}
	return path
}

func TestLoadBook(t *testing.T) {
	path := writeBook(t, "name,moves\nruy,e2e4 e7e5 g1f3 b8c6\nitalian,e2e4 e7e5 g1f3 b8c6 f1c4 f8c5\n")
	book, err := loadBook(path)
	if err != nil {
		t.Fatalf("loadBook: %v", err)
	}
	if len(book) != 2 {
		t.Fatalf("book has %d lines, want 2", len(book))
	}
	if book[0].Name != "ruy" || len(book[0].Moves) != 4 {
		t.Errorf("first line = %+v", book[0])
	}
}

func TestLoadBookDropsTranspositions(t *testing.T) {
	path := writeBook(t, "name,moves\nfirst,e2e4 e7e5 g1f3 b8c6\nsecond,e2e4 e7e5 g1f3 b8c6\n")
	book, err := loadBook(path)
	if err != nil {
		t.Fatalf("loadBook: %v", err)
	}
	if len(book) != 1 {
		t.Fatalf("book has %d lines, want 1", len(book))
	}
}

func TestLoadBookRejectsIllegalMove(t *testing.T) {
	path := writeBook(t, "name,moves\nbad,e2e5 e7e5\n")
	if _, err := loadBook(path); err == nil {
		t.Fatal("loadBook accepted an illegal book line")
	}
}
