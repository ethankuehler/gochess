package chess

import (
	"encoding/csv"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

type perftCase struct {
	fen   string
	depth int
	nodes uint64
	deep  bool
}

func loadPerftCases(t *testing.T) []perftCase {
	t.Helper()

	file, err := os.Open("data/perft.csv")
	if err != nil {
		t.Fatalf("open perft fixtures: %v", err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read perft fixtures: %v", err)
	}
	if len(records) < 2 {
		t.Fatal("perft fixture file has no data rows")
	}
	wantHeader := []string{"fen", "depth", "nodes", "deep"}
	if len(records[0]) != len(wantHeader) {
		t.Fatalf("perft fixture header = %v, want %v", records[0], wantHeader)
	}
	for i, column := range wantHeader {
		if records[0][i] != column {
			t.Fatalf("perft fixture header = %v, want %v", records[0], wantHeader)
		}
	}

	cases := make([]perftCase, 0, len(records)-1)
	for line, record := range records[1:] {
		if len(record) != len(wantHeader) {
			t.Fatalf("perft fixture line %d has %d columns, want %d", line+2, len(record), len(wantHeader))
		}
		depth, err := strconv.Atoi(record[1])
		if err != nil || depth < 0 {
			t.Fatalf("perft fixture line %d: invalid depth %q", line+2, record[1])
		}
		nodes, err := strconv.ParseUint(record[2], 10, 64)
		if err != nil {
			t.Fatalf("perft fixture line %d: invalid node count %q", line+2, record[2])
		}
		deep, err := strconv.ParseBool(record[3])
		if err != nil {
			t.Fatalf("perft fixture line %d: invalid deep flag %q", line+2, record[3])
		}
		cases = append(cases, perftCase{
			fen:   record[0],
			depth: depth,
			nodes: nodes,
			deep:  deep,
		})
	}
	return cases
}

func TestPerft(t *testing.T) {
	cases := loadPerftCases(t)
	includeDeep := os.Getenv("PERFT_DEEP") == "1" && !testing.Short()

	ran := 0
	for i, tc := range cases {
		if tc.deep && !includeDeep {
			continue
		}
		ran++
		t.Run(fmt.Sprintf("case_%03d_d%d", i, tc.depth), func(t *testing.T) {
			b := mustFEN(t, tc.fen)
			if got := b.Perft(tc.depth); got != tc.nodes {
				t.Errorf("Perft(%d) = %d, want %d (fen %s)", tc.depth, got, tc.nodes, tc.fen)
			}
		})
	}
	if ran == 0 {
		t.Fatal("no perft cases ran")
	}
}

func TestPerftDivideStartPosition(t *testing.T) {
	results := NewStartingPosition().PerftDivide(2)
	if len(results) != 20 {
		t.Fatalf("got %d root moves, want 20", len(results))
	}
	for _, result := range results {
		if result.Nodes != 20 {
			t.Errorf("%s: nodes = %d, want 20", result.Move, result.Nodes)
		}
	}
	for i := 1; i < len(results); i++ {
		if results[i-1].Move.String() >= results[i].Move.String() {
			t.Errorf("divide results are not sorted: %s before %s", results[i-1].Move, results[i].Move)
		}
	}
}

func TestPerftDepthZero(t *testing.T) {
	if got := NewStartingPosition().Perft(0); got != 1 {
		t.Errorf("Perft(0) = %d, want 1", got)
	}
}

func stockfishPerft(t *testing.T, path, fen string, depth int) uint64 {
	t.Helper()

	cmd := exec.Command(path)
	cmd.Stdin = strings.NewReader(fmt.Sprintf("position fen %s\ngo perft %d\nquit\n", fen, depth))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run Stockfish: %v\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "Nodes searched:"); ok {
			nodes, err := strconv.ParseUint(strings.TrimSpace(rest), 10, 64)
			if err != nil {
				t.Fatalf("parse Stockfish nodes %q: %v", line, err)
			}
			return nodes
		}
	}
	t.Fatalf("no perft output from Stockfish for %s:\n%s", fen, out)
	return 0
}

func TestPerftAgainstStockfish(t *testing.T) {
	path := os.Getenv("STOCKFISH_BIN")
	if path == "" {
		t.Skip("set STOCKFISH_BIN to run the Stockfish differential test")
	}
	if _, err := exec.LookPath(path); err != nil {
		t.Fatalf("STOCKFISH_BIN %q not found: %v", path, err)
	}

	b := NewStartingPosition()
	rng := rand.New(rand.NewPCG(0x9e3779b97f4a7c15, 0xbf58476d1ce4e5b9))
	for ply := 0; ply < 30; ply++ {
		got := b.Perft(3)
		want := stockfishPerft(t, path, b.FEN(), 3)
		if got != want {
			t.Fatalf("ply %d: Perft(3) = %d, Stockfish = %d (fen %s)", ply, got, want, b.FEN())
		}
		moves := b.LegalMoves()
		if len(moves) == 0 {
			return
		}
		next := b.makeMove(moves[rng.IntN(len(moves))])
		b = &next
	}
}
