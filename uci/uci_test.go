package uci

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

func TestParseSetOption(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantName string
		wantVal  string
		hasVal   bool
		ok       bool
	}{
		{"hash", []string{"name", "Hash", "value", "256"}, "Hash", "256", true, true},
		{"spaces in name", []string{"name", "Move", "Overhead", "value", "100"}, "Move Overhead", "100", true, true},
		{"button", []string{"name", "Clear", "Hash"}, "Clear Hash", "", false, true},
		{"case", []string{"NAME", "hash", "VALUE", "32"}, "hash", "32", true, true},
		{"missing name", []string{"foo", "bar"}, "", "", false, false},
		{"empty", nil, "", "", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, value, hasValue, ok := parseSetOption(tc.args)
			if name != tc.wantName || value != tc.wantVal || hasValue != tc.hasVal || ok != tc.ok {
				t.Fatalf("parseSetOption(%v) = %q, %q, %v, %v; want %q, %q, %v, %v",
					tc.args, name, value, hasValue, ok, tc.wantName, tc.wantVal, tc.hasVal, tc.ok)
			}
		})
	}
}

func TestOptions(t *testing.T) {
	options := defaultOptions()
	lines := options.lines()
	wantLines := 5
	if metricsSupported {
		wantLines = 6
	}
	if len(lines) != wantLines {
		t.Fatalf("got %d option lines, want %d: %v", len(lines), wantLines, lines)
	}
	if !strings.Contains(lines[0], "option name Hash type spin default 16 min 1 max 1024") {
		t.Errorf("unexpected Hash line: %s", lines[0])
	}
	if lines[1] != "option name Clear Hash type button" {
		t.Errorf("unexpected Clear Hash line: %s", lines[1])
	}
	if !strings.Contains(lines[2], "option name Threads type spin default 1 min 1 max 1") {
		t.Errorf("unexpected Threads line: %s", lines[2])
	}
	if !strings.Contains(lines[3], "option name Move Overhead type spin default 50 min 0 max 5000") {
		t.Errorf("unexpected Move Overhead line: %s", lines[3])
	}
	if metricsSupported && lines[5] != "option name Analysis type check default false" {
		t.Errorf("unexpected Analysis line: %s", lines[5])
	}
	if options.analysis {
		t.Error("Analysis defaults to true")
	}
	if lines[4] != "option name Ponder type check default false" {
		t.Errorf("unexpected Ponder line: %s", lines[4])
	}
	if options.ponder {
		t.Error("Ponder defaults to true")
	}

	if msg := options.set("Move Overhead", "100", true); msg != "" {
		t.Errorf("Move Overhead message = %q", msg)
	}
	if options.moveOverhead != 100*time.Millisecond {
		t.Errorf("moveOverhead = %v, want 100ms", options.moveOverhead)
	}

	options.set("Move Overhead", "-5", true)
	if options.moveOverhead != 0 {
		t.Errorf("negative overhead = %v, want 0", options.moveOverhead)
	}
	options.set("Move Overhead", "99999", true)
	if options.moveOverhead != maxMoveOverheadMS*time.Millisecond {
		t.Errorf("large overhead = %v, want clamp", options.moveOverhead)
	}

	if msg := options.set("Threads", "4", true); !strings.Contains(msg, "fixed at 1") {
		t.Errorf("Threads message = %q", msg)
	}
	if options.threads != 1 {
		t.Errorf("threads = %d, want 1", options.threads)
	}

	if msg := options.set("Analysis", "TRUE", true); msg != "" || !options.analysis {
		t.Errorf("Analysis true message = %q, enabled = %v", msg, options.analysis)
	}
	if msg := options.set("Analysis", "yes", true); !strings.Contains(msg, "invalid Analysis value") || !options.analysis {
		t.Errorf("invalid Analysis message = %q, enabled = %v", msg, options.analysis)
	}
	if msg := options.set("Analysis", "false", true); msg != "" || options.analysis {
		t.Errorf("Analysis false message = %q, enabled = %v", msg, options.analysis)
	}
	if msg := options.set("Analysis", "", false); msg != "" || options.analysis {
		t.Errorf("missing Analysis value message = %q, enabled = %v", msg, options.analysis)
	}

	if msg := options.set("Ponder", "TRUE", true); msg != "" || !options.ponder {
		t.Errorf("Ponder true message = %q, enabled = %v", msg, options.ponder)
	}
	if msg := options.set("Ponder", "yes", true); !strings.Contains(msg, "invalid Ponder value") || !options.ponder {
		t.Errorf("invalid Ponder message = %q, enabled = %v", msg, options.ponder)
	}
	if msg := options.set("Ponder", "false", true); msg != "" || options.ponder {
		t.Errorf("Ponder false message = %q, enabled = %v", msg, options.ponder)
	}
	if msg := options.set("Ponder", "", false); msg != "" || options.ponder {
		t.Errorf("missing Ponder value message = %q, enabled = %v", msg, options.ponder)
	}

	if msg := options.set("Hash", "256", true); msg != "" {
		t.Errorf("Hash message = %q, want empty", msg)
	}
	if options.hashMB != 256 {
		t.Errorf("hashMB = %d, want 256", options.hashMB)
	}
	options.set("Hash", "0", true)
	if options.hashMB != chess.MinHashMB {
		t.Errorf("hashMB = %d after 0, want %d", options.hashMB, chess.MinHashMB)
	}
	options.set("Hash", "99999", true)
	if options.hashMB != chess.MaxHashMB {
		t.Errorf("hashMB = %d after a huge value, want %d", options.hashMB, chess.MaxHashMB)
	}
	if msg := options.set("Hash", "not-a-number", true); !strings.Contains(msg, "invalid Hash value") {
		t.Errorf("invalid Hash message = %q", msg)
	}
	if msg := options.set("Clear Hash", "", false); msg != "" {
		t.Errorf("Clear Hash message = %q, want empty", msg)
	}
	if msg := options.set("Ponder", "true", true); msg != "" {
		t.Errorf("Ponder message = %q, want empty", msg)
	}
	if msg := options.set("UCI_Chess960", "true", true); !strings.Contains(msg, "Chess960") {
		t.Errorf("UCI_Chess960 message = %q", msg)
	}
	if msg := options.set("Unknown", "value", true); msg != "" {
		t.Errorf("unknown option message = %q, want empty", msg)
	}
	if msg := options.set("Move Overhead", "not-a-number", true); msg == "" {
		t.Error("invalid Move Overhead value was accepted without a message")
	}
}

func TestParseGoParams(t *testing.T) {
	params := parseGoParams([]string{
		"wtime", "1000", "btime", "2000", "winc", "10", "binc", "20",
		"movestogo", "40", "depth", "5", "nodes", "1000", "movetime", "50",
		"infinite",
	})

	if params.wtime != time.Second || params.btime != 2*time.Second {
		t.Errorf("clocks = %v, %v", params.wtime, params.btime)
	}
	if params.winc != 10*time.Millisecond || params.binc != 20*time.Millisecond {
		t.Errorf("increments = %v, %v", params.winc, params.binc)
	}
	if params.movestogo != 40 || params.depth != 5 || params.nodes != 1000 {
		t.Errorf("movestogo=%d depth=%d nodes=%d", params.movestogo, params.depth, params.nodes)
	}
	if params.movetime != 50*time.Millisecond || !params.movetimeSet {
		t.Errorf("movetime = %v, set=%v", params.movetime, params.movetimeSet)
	}
	if !params.infinite || !params.hasClock {
		t.Errorf("infinite=%v hasClock=%v", params.infinite, params.hasClock)
	}

	search := parseGoParams([]string{"searchmoves", "e2e4", "d7d5"})
	if !search.searchSet || len(search.searchMoves) != 2 {
		t.Fatalf("searchmoves = %v, set=%v", search.searchMoves, search.searchSet)
	}
	if search.searchMoves[0] != "e2e4" || search.searchMoves[1] != "d7d5" {
		t.Fatalf("searchmoves = %v", search.searchMoves)
	}

	invalid := parseGoParams([]string{"depth", "0", "nodes", "-1", "movetime", "x", "movestogo", "0"})
	if invalid.depth != 0 || invalid.nodes != 0 || invalid.movetime != 0 || invalid.movestogo != 0 {
		t.Fatalf("invalid values were accepted: %+v", invalid)
	}
	if !invalid.movetimeSet {
		t.Fatal("movetime token should be recorded even with an invalid value")
	}

	overhead := 50 * time.Millisecond
	zeroMoveTime := allocateBudget(goParams{movetimeSet: true}, chess.White, overhead)
	if !zeroMoveTime.ok || zeroMoveTime.soft != time.Millisecond || zeroMoveTime.hard != time.Millisecond {
		t.Fatalf("zero movetime budget = %+v", zeroMoveTime)
	}

	empty := parseGoParams(nil)
	if empty.infinite || empty.hasClock || empty.searchSet {
		t.Fatalf("empty go parsed as %+v", empty)
	}
}

func TestAllocateBudget(t *testing.T) {
	overhead := 50 * time.Millisecond

	movetime := allocateBudget(goParams{movetime: 200 * time.Millisecond, movetimeSet: true}, chess.White, overhead)
	if !movetime.ok || movetime.soft != 150*time.Millisecond || movetime.hard != 150*time.Millisecond {
		t.Fatalf("movetime budget = %+v", movetime)
	}

	tinyMovetime := allocateBudget(goParams{movetime: 10 * time.Millisecond, movetimeSet: true}, chess.White, overhead)
	if !tinyMovetime.ok || tinyMovetime.soft != time.Millisecond || tinyMovetime.hard != time.Millisecond {
		t.Fatalf("tiny movetime budget = %+v", tinyMovetime)
	}

	infinite := allocateBudget(goParams{infinite: true}, chess.White, overhead)
	if infinite.ok {
		t.Fatalf("infinite budget = %+v, want no deadline", infinite)
	}

	noClock := allocateBudget(goParams{depth: 3}, chess.White, overhead)
	if noClock.ok {
		t.Fatalf("depth-only budget = %+v, want no deadline", noClock)
	}

	suddenDeath := allocateBudget(goParams{hasClock: true, wtime: time.Second, btime: time.Second}, chess.White, overhead)
	if !suddenDeath.ok {
		t.Fatal("sudden death produced no budget")
	}
	if suddenDeath.soft <= 0 || suddenDeath.soft > suddenDeath.hard || suddenDeath.hard > time.Second-overhead {
		t.Fatalf("sudden death budget = %+v", suddenDeath)
	}

	white := allocateBudget(goParams{
		hasClock: true,
		wtime:    10 * time.Second,
		btime:    time.Second,
	}, chess.White, overhead)
	black := allocateBudget(goParams{
		hasClock: true,
		wtime:    10 * time.Second,
		btime:    time.Second,
	}, chess.Black, overhead)
	if !white.ok || !black.ok || white.soft <= black.soft*2 {
		t.Fatalf("White budget = %+v, Black budget = %+v", white, black)
	}

	repeating := allocateBudget(goParams{
		hasClock:  true,
		wtime:     time.Second,
		btime:     time.Second,
		movestogo: 40,
	}, chess.White, overhead)
	if !repeating.ok || repeating.soft <= 0 || repeating.soft > repeating.hard {
		t.Fatalf("repeating budget = %+v", repeating)
	}

	lowTime := allocateBudget(goParams{hasClock: true, wtime: 100 * time.Millisecond}, chess.White, overhead)
	if !lowTime.ok || lowTime.soft != time.Millisecond || lowTime.hard != time.Millisecond {
		t.Fatalf("low-time budget = %+v", lowTime)
	}

	// Movetime caps the budget derived from the clock.
	clockCapped := allocateBudget(goParams{
		hasClock:    true,
		wtime:       3 * time.Second,
		btime:       3 * time.Second,
		winc:        2 * time.Second,
		binc:        2 * time.Second,
		movetime:    5 * time.Second,
		movetimeSet: true,
	}, chess.White, overhead)
	if !clockCapped.ok || clockCapped.soft != 750*time.Millisecond || clockCapped.hard != 1500*time.Millisecond {
		t.Fatalf("clock-capped movetime = %+v", clockCapped)
	}

	// With plenty of clock, the movetime ceiling wins.
	movetimeCeiling := allocateBudget(goParams{
		hasClock:    true,
		wtime:       5 * time.Minute,
		btime:       5 * time.Minute,
		movetime:    500 * time.Millisecond,
		movetimeSet: true,
	}, chess.White, overhead)
	if !movetimeCeiling.ok || movetimeCeiling.soft != 450*time.Millisecond || movetimeCeiling.hard != 450*time.Millisecond {
		t.Fatalf("movetime ceiling = %+v", movetimeCeiling)
	}

	// A low clock overrides a large movetime completely.
	lowClockWithMovetime := allocateBudget(goParams{
		hasClock:    true,
		wtime:       100 * time.Millisecond,
		movetime:    5 * time.Second,
		movetimeSet: true,
	}, chess.White, overhead)
	if !lowClockWithMovetime.ok || lowClockWithMovetime.soft != time.Millisecond || lowClockWithMovetime.hard != time.Millisecond {
		t.Fatalf("low clock with movetime = %+v", lowClockWithMovetime)
	}

	zeroClock := allocateBudget(goParams{hasClock: true}, chess.White, overhead)
	if !zeroClock.ok || zeroClock.soft != time.Millisecond {
		t.Fatalf("zero-clock budget = %+v", zeroClock)
	}
}

func TestParsePosition(t *testing.T) {
	start, _, err := parsePosition([]string{"startpos"})
	if err != nil {
		t.Fatalf("startpos: %v", err)
	}
	if start.FEN() != chess.NewStartingPosition().FEN() {
		t.Fatalf("startpos FEN = %q", start.FEN())
	}

	afterMove, history, err := parsePosition([]string{"startpos", "moves", "e2e4", "e7e5"})
	if err != nil {
		t.Fatalf("startpos moves: %v", err)
	}
	const want = "rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2"
	if afterMove.FEN() != want {
		t.Fatalf("after moves FEN = %q, want %q", afterMove.FEN(), want)
	}
	if len(history) != 2 {
		t.Fatalf("after moves history length = %d, want 2", len(history))
	}
	if got := afterMove.RepetitionCount(history); got != 0 {
		t.Fatalf("after moves RepetitionCount = %d, want 0", got)
	}

	fourFields, _, err := parsePosition([]string{
		"fen", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR", "w", "KQkq", "-",
	})
	if err != nil {
		t.Fatalf("four-field FEN: %v", err)
	}
	if fourFields.FEN() != chess.NewStartingPosition().FEN() {
		t.Fatalf("four-field FEN = %q", fourFields.FEN())
	}

	fiveFields, _, err := parsePosition([]string{
		"fen", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR", "w", "KQkq", "-", "4",
	})
	if err != nil {
		t.Fatalf("five-field FEN: %v", err)
	}
	if !strings.HasSuffix(fiveFields.FEN(), " 4 1") {
		t.Fatalf("five-field FEN = %q", fiveFields.FEN())
	}
}

func TestParsePositionErrors(t *testing.T) {
	cases := [][]string{
		nil,
		{"fen"},
		{"fen", "not-a-fen"},
		{"startpos", "moves", "e2e5"},
		{"startpos", "nonsense"},
		{"startpos", "moves", "e2e4", "e7e5", "g1f3", "b8c6", "f1b5", "a7a6", "b5c6", "d7c6", "e1g1", "e8g8"},
	}
	for _, args := range cases {
		if _, _, err := parsePosition(args); err == nil {
			t.Errorf("parsePosition(%v) succeeded, want an error", args)
		}
	}
}

func TestFormatPV(t *testing.T) {
	position := chess.NewStartingPosition()
	e2e4 := legalMove(t, position, "e2e4")
	d2d4 := legalMove(t, position, "d2d4")
	if got := formatPV([]chess.Move{e2e4, d2d4}); got != "e2e4 d2d4" {
		t.Fatalf("formatPV = %q", got)
	}
	if got := formatPV(nil); got != "" {
		t.Fatalf("formatPV(nil) = %q", got)
	}
}

func TestBenchDeterministic(t *testing.T) {
	var firstOut, firstErr bytes.Buffer
	if code := Bench(&firstOut, &firstErr, 3); code != 0 {
		t.Fatalf("first bench exit code = %d, stderr = %s", code, firstErr.String())
	}
	var secondOut, secondErr bytes.Buffer
	if code := Bench(&secondOut, &secondErr, 3); code != 0 {
		t.Fatalf("second bench exit code = %d, stderr = %s", code, secondErr.String())
	}

	firstNodes := benchNodes(t, firstOut.String())
	secondNodes := benchNodes(t, secondOut.String())
	if firstNodes != secondNodes {
		t.Fatalf("bench node counts differ: %s vs %s", firstNodes, secondNodes)
	}
	if firstNodes == "0" {
		t.Fatal("bench reported zero nodes")
	}
}

func benchNodes(t *testing.T, output string) string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		t.Fatal("bench produced no output")
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) != 4 || fields[1] != "nodes" || fields[3] != "nps" {
		t.Fatalf("bench output line = %q", lines[len(lines)-1])
	}
	return fields[0]
}

func legalMove(t *testing.T, position *chess.Position, uci string) chess.Move {
	t.Helper()
	parsed, err := chess.ParseUCIMove(uci)
	if err != nil {
		t.Fatalf("ParseUCIMove(%q): %v", uci, err)
	}
	legal, ok := position.FindLegalMove(*parsed)
	if !ok {
		t.Fatalf("move %q is not legal", uci)
	}
	return legal
}
