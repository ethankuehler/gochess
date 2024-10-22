package chess

import (
	"testing"

	"github.com/ethankuehler/gochess/chess"
)

func TestKnightAttacks(t *testing.T) {
	chess.BuildKnightAttacks()
	if chess.KNIGHT_ATTACKS == nil {
		t.Error("nil map")
	}

	if len(chess.KNIGHT_ATTACKS) != 64 {
		t.Errorf("map incorrect size, 64 != %d", len(chess.KNIGHT_ATTACKS))
	}

	//TODO: more testing
}

func TestPawnAttacks(t *testing.T) {
	chess.BuildPawnAttacks()
	if chess.PAWN_ATTACKS == nil {
		t.Error("nil map")
	}

	if len(chess.PAWN_ATTACKS) != 48 {
		t.Errorf("map incorrect size, 64 != %d", len(chess.PAWN_ATTACKS))
	}
	//TODO: more testing
}

func TestPawnMoves(t *testing.T) {
	chess.BuildPawnMoves()
	if chess.PAWN_MOVES == nil {
		t.Error("nil map")
	}

	if len(chess.PAWN_MOVES) != 48 {
		t.Errorf("map incorrect size, 64 != %d", len(chess.PAWN_ATTACKS))
	}
	//TODO: more testing
}
