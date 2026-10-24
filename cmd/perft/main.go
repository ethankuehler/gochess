// Command perft runs move-generation benchmarks and compares the engine's
// perft divide with Stockfish.
package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

var divideLine = regexp.MustCompile(`^([a-h][1-8][a-h][1-8][qrbn]?): (\d+)$`)

type suiteCase struct {
	fen   string
	depth int
	nodes uint64
	deep  bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "perft:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		fen           = flag.String("fen", "", "position in FEN (default: starting position)")
		depth         = flag.Int("depth", 5, "perft depth")
		moves         = flag.Bool("moves", false, "print all legal moves in UCI and exit")
		divide        = flag.Bool("divide", false, "print per-root-move node counts")
		suite         = flag.String("suite", "", "run a perft fixture CSV")
		deep          = flag.Bool("deep", false, "include deep suite rows")
		compare       = flag.Bool("compare", false, "compare the divide with Stockfish")
		stockfishPath = flag.String("stockfish", "stockfish", "Stockfish binary for -compare")
	)
	flag.Parse()

	chess.BuildAllAttacks()

	if *suite != "" {
		return runSuite(*suite, *deep)
	}

	fenValue := *fen
	if fenValue == "" {
		fenValue = chess.NewStartingPosition().FEN()
	}
	b, err := chess.ParseFEN(fenValue)
	if err != nil {
		return err
	}

	if *depth < 0 {
		return fmt.Errorf("depth must not be negative")
	}

	if *moves {
		printMoves(b)
		return nil
	}

	if *compare {
		if *depth < 1 {
			return fmt.Errorf("compare requires depth >= 1")
		}
		return compareWithStockfish(b, fenValue, *depth, *stockfishPath)
	}

	start := time.Now()
	if *divide {
		if *depth < 1 {
			return fmt.Errorf("divide requires depth >= 1")
		}
		results := b.PerftDivide(*depth)
		var total uint64
		for _, result := range results {
			fmt.Printf("%s: %d\n", result.Move, result.Nodes)
			total += result.Nodes
		}
		elapsed := time.Since(start)
		fmt.Printf("\nNodes searched: %d\n", total)
		fmt.Printf("Time: %s\n", elapsed)
		if elapsed > 0 {
			fmt.Printf("NPS: %.0f\n", float64(total)/elapsed.Seconds())
		}
		return nil
	}

	nodes := b.Perft(*depth)
	elapsed := time.Since(start)
	fmt.Printf("Nodes searched: %d\n", nodes)
	fmt.Printf("Time: %s\n", elapsed)
	if elapsed > 0 {
		fmt.Printf("NPS: %.0f\n", float64(nodes)/elapsed.Seconds())
	}
	return nil
}

func printMoves(b *chess.Position) {
	moves := b.LegalMoves()
	names := make([]string, len(moves))
	for i, move := range moves {
		names[i] = move.String()
	}
	slices.Sort(names)
	for _, name := range names {
		fmt.Println(name)
	}
	fmt.Fprintf(os.Stderr, "%d legal moves\n", len(names))
}

func compareWithStockfish(b *chess.Position, fen string, depth int, stockfishPath string) error {
	results := b.PerftDivide(depth)
	ours := make(map[string]uint64, len(results))
	var ourTotal uint64
	for _, result := range results {
		uci := result.Move.String()
		ours[uci] = result.Nodes
		ourTotal += result.Nodes
	}

	theirs, theirTotal, err := stockfishDivide(stockfishPath, fen, depth)
	if err != nil {
		return err
	}

	mismatches := 0
	for _, result := range results {
		uci := result.Move.String()
		want, ok := theirs[uci]
		if !ok {
			fmt.Printf("missing in Stockfish: %s (%d nodes)\n", uci, result.Nodes)
			mismatches++
		} else if want != result.Nodes {
			fmt.Printf("%s: ours %d, Stockfish %d\n", uci, result.Nodes, want)
			mismatches++
		}
	}
	for uci, nodes := range theirs {
		if _, ok := ours[uci]; !ok {
			fmt.Printf("extra in Stockfish: %s (%d nodes)\n", uci, nodes)
			mismatches++
		}
	}
	if ourTotal != theirTotal {
		fmt.Printf("total: ours %d, Stockfish %d\n", ourTotal, theirTotal)
		mismatches++
	}
	if mismatches != 0 {
		return fmt.Errorf("%d divide mismatches", mismatches)
	}
	fmt.Printf("OK: %d root moves and %d nodes match Stockfish\n", len(ours), ourTotal)
	return nil
}

func stockfishDivide(stockfishPath, fen string, depth int) (map[string]uint64, uint64, error) {
	cmd := exec.Command(stockfishPath)
	cmd.Stdin = strings.NewReader(fmt.Sprintf("position fen %s\ngo perft %d\nquit\n", fen, depth))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, 0, fmt.Errorf("run %s: %w", stockfishPath, err)
	}

	divide := make(map[string]uint64)
	var total uint64
	foundNodes := false
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if rest, ok := strings.CutPrefix(line, "Nodes searched:"); ok {
			nodes, err := strconv.ParseUint(strings.TrimSpace(rest), 10, 64)
			if err != nil {
				return nil, 0, fmt.Errorf("parse Stockfish node count %q: %w", line, err)
			}
			total = nodes
			foundNodes = true
			continue
		}
		match := divideLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		nodes, err := strconv.ParseUint(match[2], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("parse Stockfish divide %q: %w", line, err)
		}
		divide[match[1]] = nodes
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("read Stockfish output: %w", err)
	}
	if !foundNodes {
		return nil, 0, fmt.Errorf("%s produced no perft output", stockfishPath)
	}
	return divide, total, nil
}

func runSuite(path string, includeDeep bool) error {
	cases, err := readSuite(path)
	if err != nil {
		return err
	}

	start := time.Now()
	var ran int
	var totalNodes uint64
	var failures int
	for _, tc := range cases {
		if tc.deep && !includeDeep {
			continue
		}
		ran++

		b, err := chess.ParseFEN(tc.fen)
		if err != nil {
			fmt.Printf("FAIL d%d %s: %v\n", tc.depth, tc.fen, err)
			failures++
			continue
		}
		got := b.Perft(tc.depth)
		totalNodes += got
		if got != tc.nodes {
			fmt.Printf("FAIL d%d got %d want %d  %s\n", tc.depth, got, tc.nodes, tc.fen)
			failures++
			continue
		}
		fmt.Printf("PASS d%d %d  %s\n", tc.depth, got, tc.fen)
	}

	elapsed := time.Since(start)
	if ran == 0 {
		return fmt.Errorf("no suite cases ran")
	}
	fmt.Printf("\n%d/%d cases passed in %s", ran-failures, ran, elapsed)
	if elapsed > 0 {
		fmt.Printf(" (%.0f nodes/s)", float64(totalNodes)/elapsed.Seconds())
	}
	fmt.Println()
	if failures != 0 {
		return fmt.Errorf("%d cases failed", failures)
	}
	return nil
}

func readSuite(path string) ([]suiteCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("%s: no data rows", path)
	}
	wantHeader := []string{"fen", "depth", "nodes", "deep"}
	if len(records[0]) != len(wantHeader) {
		return nil, fmt.Errorf("%s: header = %v, want %v", path, records[0], wantHeader)
	}
	for i, column := range wantHeader {
		if records[0][i] != column {
			return nil, fmt.Errorf("%s: header = %v, want %v", path, records[0], wantHeader)
		}
	}

	cases := make([]suiteCase, 0, len(records)-1)
	for line, record := range records[1:] {
		if len(record) != len(wantHeader) {
			return nil, fmt.Errorf("%s line %d: got %d columns, want %d", path, line+2, len(record), len(wantHeader))
		}
		depth, err := strconv.Atoi(record[1])
		if err != nil || depth < 0 {
			return nil, fmt.Errorf("%s line %d: invalid depth %q", path, line+2, record[1])
		}
		nodes, err := strconv.ParseUint(record[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: invalid node count %q", path, line+2, record[2])
		}
		deep, err := strconv.ParseBool(record[3])
		if err != nil {
			return nil, fmt.Errorf("%s line %d: invalid deep flag %q", path, line+2, record[3])
		}
		cases = append(cases, suiteCase{fen: record[0], depth: depth, nodes: nodes, deep: deep})
	}
	return cases, nil
}
