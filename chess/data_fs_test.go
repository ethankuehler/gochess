package chess

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/ethankuehler/gochess/data"
)

func TestBuildAllAttacksFromEmbeddedData(t *testing.T) {
	if err := BuildAllAttacksFromFS(data.FS); err != nil {
		t.Fatalf("BuildAllAttacksFromFS: %v", err)
	}

	position := NewStartingPosition()
	if got := len(position.LegalMoves()); got != 20 {
		t.Fatalf("starting position has %d legal moves, want 20", got)
	}
	if got, want := position.Perft(2), uint64(400); got != want {
		t.Fatalf("perft(2) = %d, want %d", got, want)
	}

	occupancy := BitBoard(0x00FF000000FF0000)
	for square := Square(0); square < squareCount; square++ {
		if got, want := RookAttack(square, occupancy), SlidingAttacks(square, occupancy, rookDirections); got != want {
			t.Fatalf("rook attack mismatch at square %d", square)
		}
		if got, want := BishopAttack(square, occupancy), SlidingAttacks(square, occupancy, bishopDirections); got != want {
			t.Fatalf("bishop attack mismatch at square %d", square)
		}
	}
}

func TestBuildAllAttacksFromFSRejectsMissingFiles(t *testing.T) {
	if err := BuildAllAttacksFromFS(data.FS); err != nil {
		t.Fatalf("initial BuildAllAttacksFromFS: %v", err)
	}

	empty := fstest.MapFS{}
	if err := BuildAllAttacksFromFS(empty); err == nil {
		t.Fatal("BuildAllAttacksFromFS succeeded with an empty filesystem")
	}

	// A failed build must not clobber the tables.
	if got := len(NewStartingPosition().LegalMoves()); got != 20 {
		t.Fatalf("starting position has %d legal moves after failed init, want 20", got)
	}
}

func TestEmbeddedDataContainsRuntimeFiles(t *testing.T) {
	for _, name := range []string{
		"knight_attacks.csv",
		"king_attacks.csv",
		"white_pawn_move.csv",
		"black_pawn_move.csv",
		"white_pawn_attacks.csv",
		"black_pawn_attacks.csv",
		"rook_magics.csv",
		"bishop_magics.csv",
	} {
		if _, err := fs.Stat(data.FS, name); err != nil {
			t.Errorf("embedded %s: %v", name, err)
		}
	}
}

func TestLoadMagicEntriesFromFS(t *testing.T) {
	entries, err := LoadMagicEntriesFS(data.FS, "rook_magics.csv")
	if err != nil {
		t.Fatalf("LoadMagicEntriesFS: %v", err)
	}
	if len(entries) != squareCount {
		t.Fatalf("loaded %d entries, want %d", len(entries), squareCount)
	}
	if _, err := LoadMagicEntriesFS(data.FS, "missing.csv"); err == nil {
		t.Fatal("LoadMagicEntriesFS succeeded for a missing file")
	}
}
