package uci

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/data"
)

const (
	defaultBenchDepth = 5
	maxBenchDepth     = 8
)

// benchPositions is a fixed, self-contained set of positions. The bench must
// not depend on the working directory or on test data files.
var benchPositions = []string{
	"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
	"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
	"r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
	"rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8",
	"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
	"4k3/8/8/3q4/8/8/3R4/4K3 w - - 0 1",
	"6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 0 1",
	"8/8/8/4k3/8/8/3Q4/4K3 w - - 0 1",
}

// Bench runs the deterministic benchmark and prints its final
// "<nodes> nodes <nps> nps" line to out. It returns a process exit code.
func Bench(out, errOut io.Writer, depth int) int {
	if depth < 1 {
		depth = defaultBenchDepth
	}
	if err := chess.BuildAllAttacksFromFS(data.FS); err != nil {
		fmt.Fprintf(errOut, "bench: initialization failed: %v\n", err)
		return 1
	}
	return runBench(newLineWriter(out), errOut, depth)
}

func runBench(out *lineWriter, errOut io.Writer, depth int) int {
	if depth < 1 {
		depth = defaultBenchDepth
	}
	if depth > maxBenchDepth {
		depth = maxBenchDepth
	}
	start := time.Now()
	var total uint64

	for i, fen := range benchPositions {
		position, err := chess.ParseFEN(fen)
		if err != nil {
			fmt.Fprintf(errOut, "bench: position %d: %v\n", i+1, err)
			return 1
		}
		result := position.SearchWithLimits(context.Background(), chess.SearchLimits{MaxDepth: depth}, nil)
		total += result.Nodes
		fmt.Fprintf(errOut, "bench %d/%d nodes %d\n", i+1, len(benchPositions), result.Nodes)
	}

	elapsed := time.Since(start)
	var nps uint64
	if elapsed > 0 {
		nps = total * uint64(time.Second) / uint64(elapsed)
	}
	out.line("%d nodes %d nps", total, nps)
	return 0
}

func (e *Engine) handleBench(args []string) {
	if e.isSearching() {
		fmt.Fprintln(e.errOut, "bench ignored while searching")
		return
	}
	if err := e.ensureInitialized(); err != nil {
		e.out.line("info string initialization failed: %v", err)
		return
	}
	depth := defaultBenchDepth
	if len(args) > 0 {
		if value, err := strconv.Atoi(args[0]); err == nil && value > 0 {
			depth = value
		}
	}
	runBench(e.out, e.errOut, depth)
}
