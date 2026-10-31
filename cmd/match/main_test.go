package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/data"
)

func TestMain(m *testing.M) {
	if err := chess.BuildAllAttacksFromFS(data.FS); err != nil {
		fmt.Fprintf(os.Stderr, "initialize attack tables: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestDrawReason(t *testing.T) {
	cases := []struct {
		name  string
		fen   string
		moves []string
		want  string
	}{
		{
			name: "playing",
			fen:  chess.NewStartingPosition().FEN(),
			want: "",
		},
		{
			name: "insufficient material",
			fen:  "4k3/8/8/8/8/8/8/4K3 w - - 0 1",
			want: "insufficient material",
		},
		{
			name: "fifty move",
			fen:  "4k3/8/8/8/8/8/8/R3K3 w - - 100 1",
			want: "fifty-move rule",
		},
		{
			name: "threefold repetition",
			fen:  "4k3/8/8/8/8/8/8/R3K3 w - - 0 1",
			moves: []string{
				"e1d1", "e8d8", "d1e1", "d8e8",
				"e1d1", "e8d8", "d1e1", "d8e8",
			},
			want: "threefold repetition",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			board, err := chess.ParseFEN(tc.fen)
			if err != nil {
				t.Fatalf("ParseFEN: %v", err)
			}
			var history []chess.PositionKey
			for _, move := range tc.moves {
				history = append(history, board.Key())
				next, _, err := board.ApplyUCIMove(move)
				if err != nil {
					t.Fatalf("ApplyUCIMove(%q): %v", move, err)
				}
				board = next
			}
			if got := drawReason(board, history); got != tc.want {
				t.Errorf("drawReason = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestComparePerft(t *testing.T) {
	ours := map[string]uint64{"e2e4": 20, "d2d4": 20}
	theirs := map[string]uint64{"e2e4": 20, "d2d4": 20}
	if err := comparePerft(ours, 40, theirs, 40, "fen"); err != nil {
		t.Fatalf("matching perft: %v", err)
	}

	theirs["e2e4"] = 21
	err := comparePerft(ours, 40, theirs, 41, "fen")
	if err == nil {
		t.Fatal("mismatched perft did not produce an error")
	}
	for _, want := range []string{"e2e4 ours=20 Stockfish=21", "total ours=40 Stockfish=41"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}

	delete(theirs, "e2e4")
	theirs["e2e5"] = 20
	err = comparePerft(ours, 40, theirs, 40, "fen")
	if err == nil {
		t.Fatal("missing/extra moves did not produce an error")
	}
	for _, want := range []string{"e2e4 missing from Stockfish", "e2e5 extra in Stockfish"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestMatchAgainstStockfish(t *testing.T) {
	path := os.Getenv("STOCKFISH_BIN")
	if path == "" {
		t.Skip("set STOCKFISH_BIN to run the Stockfish match test")
	}

	var out, errOut bytes.Buffer
	err := run([]string{
		"-stockfish", path,
		"-games", "2",
		"-depth", "1",
		"-max-moves", "30",
		"-validate-depth", "1",
		"-seed", "11",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("run: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}
	for _, want := range []string{"game 1:", "game 2:", "summary:"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output does not contain %q:\n%s", want, out.String())
		}
	}
}
