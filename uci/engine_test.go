package uci

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

type testSession struct {
	t      *testing.T
	in     *io.PipeWriter
	lines  chan string
	done   chan int
	seen   []string
	exited bool
}

func newTestSession(t *testing.T) *testSession {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan int, 1)

	go func() {
		code := Run(inR, outW, io.Discard)
		outW.Close()
		done <- code
	}()

	lines := make(chan string, 4096)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	session := &testSession{t: t, in: inW, lines: lines, done: done}
	t.Cleanup(func() {
		session.in.Close()
		if !session.exited {
			select {
			case <-done:
				session.exited = true
			case <-time.After(5 * time.Second):
				t.Errorf("engine did not exit")
			}
		}
	})
	return session
}

func (s *testSession) send(line string) {
	s.t.Helper()
	if _, err := fmt.Fprintln(s.in, line); err != nil {
		s.t.Fatalf("send %q: %v", line, err)
	}
}

func (s *testSession) next() (string, bool) {
	s.t.Helper()
	select {
	case line, ok := <-s.lines:
		if ok {
			s.seen = append(s.seen, line)
		}
		return line, ok
	case <-time.After(5 * time.Second):
		s.t.Fatal("timed out waiting for engine output")
		return "", false
	}
}

func (s *testSession) expectPrefix(prefix string) string {
	s.t.Helper()
	line, ok := s.next()
	if !ok {
		s.t.Fatalf("engine output ended while waiting for %q", prefix)
	}
	if !strings.HasPrefix(line, prefix) {
		s.t.Fatalf("line = %q, want prefix %q", line, prefix)
	}
	return line
}

func (s *testSession) waitForPrefix(prefix string) string {
	s.t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case line, ok := <-s.lines:
			if !ok {
				s.t.Fatalf("engine output ended while waiting for %q", prefix)
			}
			s.seen = append(s.seen, line)
			if strings.HasPrefix(line, prefix) {
				return line
			}
		case <-deadline:
			s.t.Fatalf("timed out waiting for %q", prefix)
		}
	}
}

func (s *testSession) assertNoBestmoveSince(start int) {
	s.t.Helper()
	for _, line := range s.seen[start:] {
		if strings.HasPrefix(line, "bestmove ") {
			s.t.Fatalf("unexpected bestmove: %q", line)
		}
	}
}

func (s *testSession) bestmoveLine() (move, ponder string) {
	s.t.Helper()
	line := s.waitForPrefix("bestmove ")
	fields := strings.Fields(line)
	if len(fields) != 2 && len(fields) != 4 {
		s.t.Fatalf("bestmove line = %q", line)
	}
	if len(fields) == 4 && fields[2] != "ponder" {
		s.t.Fatalf("bestmove line = %q, want an optional ponder token", line)
	}
	move = fields[1]
	if len(fields) == 4 {
		ponder = fields[3]
	}
	return move, ponder
}

func (s *testSession) bestmove() string {
	s.t.Helper()
	move, _ := s.bestmoveLine()
	return move
}

func (s *testSession) waitExit() {
	s.t.Helper()
	if s.exited {
		return
	}
	select {
	case <-s.done:
		s.exited = true
	case <-time.After(5 * time.Second):
		s.t.Fatal("engine did not exit")
	}
}

func (s *testSession) drain() {
	s.t.Helper()
	for line := range s.lines {
		s.seen = append(s.seen, line)
	}
}

func (s *testSession) handshake() {
	s.t.Helper()
	s.send("uci")
	s.expectPrefix("id name gochess")
	s.expectPrefix("id author ")
	s.expectPrefix("option name Hash")
	s.expectPrefix("option name Clear Hash")
	s.expectPrefix("option name Threads")
	s.expectPrefix("option name Move Overhead")
	s.expectPrefix("option name Ponder type check default false")
	if metricsSupported {
		s.expectPrefix("option name Analysis type check default false")
	}
	s.expectPrefix("uciok")
	s.send("isready")
	s.expectPrefix("readyok")
}

func (s *testSession) assertBestmoveLegal(fen string) {
	s.t.Helper()
	position, err := chess.ParseFEN(fen)
	if err != nil {
		s.t.Fatalf("ParseFEN: %v", err)
	}
	move, ponder := s.bestmoveLine()
	if move == "0000" {
		s.t.Fatal("bestmove was 0000 in a non-terminal position")
	}
	next, _, err := position.ApplyUCIMove(move)
	if err != nil {
		s.t.Fatalf("bestmove %q is not legal in %s: %v", move, fen, err)
	}
	if ponder != "" {
		parsed, err := chess.ParseUCIMove(ponder)
		if err != nil {
			s.t.Fatalf("ponder move %q: %v", ponder, err)
		}
		if _, ok := next.FindLegalMove(*parsed); !ok {
			s.t.Fatalf("ponder move %q is not legal after bestmove %q", ponder, move)
		}
	}
}

func TestHandshakeAndQuit(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("quit")
	s.waitExit()
}

func TestGoDepth(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go depth 2")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestBestmovePonderToken(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go depth 4")
	move, ponder := s.bestmoveLine()
	if move == "0000" || ponder == "" {
		t.Fatalf("bestmove = %q, ponder = %q; want a move and a ponder reply", move, ponder)
	}
	position := chess.NewStartingPosition()
	next, _, err := position.ApplyUCIMove(move)
	if err != nil {
		t.Fatalf("bestmove %q is not legal: %v", move, err)
	}
	parsed, err := chess.ParseUCIMove(ponder)
	if err != nil {
		t.Fatalf("ponder move %q: %v", ponder, err)
	}
	if _, ok := next.FindLegalMove(*parsed); !ok {
		t.Fatalf("ponder move %q is not legal after bestmove %q", ponder, move)
	}
	s.send("quit")
	s.waitExit()
}

func TestAnalysisOutput(t *testing.T) {
	if !metricsSupported {
		t.Skip("metrics build tag not enabled")
	}
	s := newTestSession(t)
	s.handshake()
	analysisCount := func(lines []string) int {
		count := 0
		for _, line := range lines {
			if strings.HasPrefix(line, "info string analysis ") {
				count++
			}
		}
		return count
	}
	parseLastAnalysis := func(lines []string) chess.SearchMetrics {
		s.t.Helper()
		if len(lines) < 2 || !strings.HasPrefix(lines[len(lines)-1], "bestmove ") {
			s.t.Fatalf("last output is not bestmove: %v", lines)
		}
		line := lines[len(lines)-2]
		if !strings.HasPrefix(line, "info string analysis ") {
			s.t.Fatalf("line before bestmove = %q", line)
		}
		payload := strings.TrimPrefix(line, "info string analysis ")
		if strings.Contains(payload, " ") {
			s.t.Fatalf("analysis JSON is not compact: %q", payload)
		}
		var metrics chess.SearchMetrics
		if err := json.Unmarshal([]byte(payload), &metrics); err != nil {
			s.t.Fatalf("invalid analysis JSON: %v", err)
		}
		return metrics
	}

	s.send("position startpos")
	start := len(s.seen)
	s.send("go depth 2")
	s.bestmove()
	if count := analysisCount(s.seen[start:]); count != 0 {
		t.Fatalf("disabled search emitted %d analysis lines", count)
	}

	s.send("setoption name Analysis value true")
	start = len(s.seen)
	s.send("go depth 2")
	s.bestmove()
	lines := s.seen[start:]
	if count := analysisCount(lines); count != 1 {
		t.Fatalf("enabled search emitted %d analysis lines: %v", count, lines)
	}
	metrics := parseLastAnalysis(lines)
	if metrics.MoveGenCalls == 0 || metrics.PseudoMoves == 0 || metrics.LegalMoves == 0 || metrics.EvaluationCalls == 0 || metrics.MakeMoveCalls == 0 || metrics.SearchMakeMoveCalls == 0 || metrics.LegalFilterNS < 0 || metrics.SearchMakeMoveNS == 0 {
		t.Fatalf("incomplete analysis metrics: %+v", metrics)
	}

	s.send("position fen 7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	start = len(s.seen)
	s.send("go depth 2")
	if move := s.bestmove(); move != "0000" {
		t.Fatalf("terminal bestmove = %s", move)
	}
	lines = s.seen[start:]
	if count := analysisCount(lines); count != 1 {
		t.Fatalf("terminal search emitted %d analysis lines: %v", count, lines)
	}
	metrics = parseLastAnalysis(lines)
	if metrics.MoveGenCalls != 1 || metrics.PseudoMoves == 0 || metrics.LegalMoves != 0 || metrics.MakeMoveCalls != metrics.PseudoMoves || metrics.SearchMakeMoveCalls != 0 {
		t.Fatalf("terminal analysis metrics: %+v", metrics)
	}

	s.send("setoption name Analysis value false")
	s.send("position startpos")
	start = len(s.seen)
	s.send("go depth 1")
	s.bestmove()
	if count := analysisCount(s.seen[start:]); count != 0 {
		t.Fatalf("re-disabled search emitted %d analysis lines", count)
	}

	s.send("quit")
	s.waitExit()
}

func TestPositionMoves(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos moves e2e4 e7e5 g1f3 b8c6 f1b5 a7a6 b5c6 d7c6 e1g1")
	s.send("go depth 2")
	const fen = "r1bqkbnr/1pp2ppp/p1p5/4p3/4P3/5N2/PPPP1PPP/RNBQ1RK1 b kq - 1 5"
	s.assertBestmoveLegal(fen)
	s.send("quit")
	s.waitExit()
}

func TestNewGameResetsPosition(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position fen 4k3/8/8/8/8/8/8/4K3 b - - 0 1")
	s.send("ucinewgame")
	s.send("go depth 1")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestTerminalPositions(t *testing.T) {
	s := newTestSession(t)
	s.handshake()

	s.send("position fen 7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	s.send("go depth 2")
	s.waitForPrefix("info depth 0 score mate 0")
	if move := s.bestmove(); move != "0000" {
		t.Fatalf("checkmate bestmove = %s, want 0000", move)
	}

	s.send("position fen 7k/5Q2/6K1/8/8/8/8/8 b - - 0 1")
	s.send("go depth 2")
	s.waitForPrefix("info depth 0 score cp 0")
	if move := s.bestmove(); move != "0000" {
		t.Fatalf("stalemate bestmove = %s, want 0000", move)
	}

	s.send("quit")
	s.waitExit()
}

func TestInfiniteStopAndIsReady(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go infinite")
	s.waitForPrefix("info depth 1")

	s.send("isready")
	s.waitForPrefix("readyok")
	s.send("stop")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestStopImmediatelyAfterGo(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go infinite")
	s.send("stop")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestPonderHitIgnoredDuringNormalSearch(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go infinite")
	s.waitForPrefix("info depth 1")
	s.send("ponderhit")
	s.send("isready")
	s.waitForPrefix("readyok")
	s.send("stop")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestPonderWaitsForPonderHit(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go ponder movetime 200")
	s.waitForPrefix("info depth 2")

	start := len(s.seen)
	s.send("isready")
	s.waitForPrefix("readyok")
	s.assertNoBestmoveSince(start)

	s.send("ponderhit")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestPonderHitStartsClockLate(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go ponder movetime 400")
	s.waitForPrefix("info depth 2")

	// The ponder movetime must not start ticking until ponderhit. If it did,
	// the engine would have stopped (and sent bestmove) before this point.
	time.Sleep(500 * time.Millisecond)
	start := len(s.seen)
	s.send("isready")
	s.waitForPrefix("readyok")
	s.assertNoBestmoveSince(start)

	hit := time.Now()
	s.send("ponderhit")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	if elapsed := time.Since(hit); elapsed < 100*time.Millisecond {
		t.Fatalf("bestmove arrived %v after ponderhit; the move budget did not start at the hit", elapsed)
	}
	s.send("quit")
	s.waitExit()
}

func TestPonderHitWithoutClockIsInfinite(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go ponder")
	s.waitForPrefix("info depth 1")
	s.send("ponderhit")

	start := len(s.seen)
	s.send("isready")
	s.waitForPrefix("readyok")
	s.assertNoBestmoveSince(start)

	s.send("stop")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestPonderHitAfterDepthLimit(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go ponder depth 2")
	s.waitForPrefix("info depth 2")

	start := len(s.seen)
	s.send("isready")
	s.waitForPrefix("readyok")
	s.assertNoBestmoveSince(start)

	s.send("ponderhit")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestStopDuringPonder(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go ponder movetime 1000")
	s.waitForPrefix("info depth 1")
	s.send("stop")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())

	s.send("position startpos")
	s.send("go depth 1")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestPonderTerminalPosition(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position fen 7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	s.send("go ponder")

	start := len(s.seen)
	s.send("isready")
	s.waitForPrefix("readyok")
	s.assertNoBestmoveSince(start)

	s.send("stop")
	if move := s.bestmove(); move != "0000" {
		t.Fatalf("terminal ponder bestmove = %s, want 0000", move)
	}
	s.send("quit")
	s.waitExit()
}

func TestPonderHitTwice(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go ponder movetime 300")
	s.waitForPrefix("info depth 2")
	s.send("ponderhit")
	s.send("ponderhit")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())

	start := len(s.seen)
	s.send("isready")
	s.waitForPrefix("readyok")
	s.assertNoBestmoveSince(start)
	s.send("quit")
	s.waitExit()
}

func TestPonderQuit(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go ponder")
	s.waitForPrefix("info depth 1")
	s.send("quit")
	s.waitExit()
}

func TestCRLFInput(t *testing.T) {
	s := newTestSession(t)
	if _, err := io.WriteString(s.in, "uci\r\nisready\r\n"); err != nil {
		t.Fatalf("write CRLF input: %v", err)
	}
	s.waitForPrefix("uciok")
	s.waitForPrefix("readyok")
	s.send("quit")
	s.waitExit()
}

func TestQuitDuringSearch(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go infinite")
	s.waitForPrefix("info depth 1")
	s.send("quit")
	s.waitExit()
}

func TestEOFExits(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.in.Close()
	s.waitExit()
}

func TestSearchMoves(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go depth 2 searchmoves e2e4")
	if move := s.bestmove(); move != "e2e4" {
		t.Fatalf("searchmoves bestmove = %s, want e2e4", move)
	}

	s.send("go depth 1 searchmoves e2e5")
	s.waitForPrefix("info depth 0 score cp 0")
	if move := s.bestmove(); move != "0000" {
		t.Fatalf("illegal searchmoves bestmove = %s, want 0000", move)
	}

	s.send("quit")
	s.waitExit()
}

func TestZeroMovetimeAndClock(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go movetime 0")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("go wtime 300 btime 300")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestNodesLimit(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go nodes 100")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestFourFieldFEN(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position fen rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -")
	s.send("go depth 1")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	for _, line := range s.seen {
		if strings.Contains(line, "position error") {
			t.Fatalf("four-field FEN produced an error: %s", line)
		}
	}
	s.send("quit")
	s.waitExit()
}

func TestInvalidPositionAndMove(t *testing.T) {
	s := newTestSession(t)
	s.handshake()

	s.send("position fen not-a-fen")
	s.waitForPrefix("info string position error")
	s.send("go depth 1")
	s.waitForPrefix("info string no valid position")
	if move := s.bestmove(); move != "0000" {
		t.Fatalf("invalid position bestmove = %s, want 0000", move)
	}

	s.send("position startpos moves e2e5")
	s.waitForPrefix("info string position error")
	s.send("go depth 1")
	s.waitForPrefix("info string no valid position")
	if move := s.bestmove(); move != "0000" {
		t.Fatalf("invalid move bestmove = %s, want 0000", move)
	}

	s.send("quit")
	s.waitExit()
}

func TestGoWithoutPositionUsesStartpos(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("go depth 1")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())
	s.send("quit")
	s.waitExit()
}

func TestSetOptionAndBench(t *testing.T) {
	s := newTestSession(t)
	s.handshake()

	s.send("setoption name Move Overhead value 100")
	s.send("setoption name Threads value 4")
	s.waitForPrefix("info string Threads is fixed at 1")
	s.send("setoption name Hash value 32")
	s.send("setoption name Clear Hash")
	s.send("setoption name Unknown value 1")

	s.send("position startpos")
	s.send("go movetime 20")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())

	s.send("bench 3")
	line := s.waitForPrefix("")
	fields := strings.Fields(line)
	if len(fields) != 4 || fields[1] != "nodes" || fields[3] != "nps" {
		t.Fatalf("bench line = %q", line)
	}
	if _, err := strconv.ParseUint(fields[0], 10, 64); err != nil {
		t.Fatalf("bench nodes = %q: %v", fields[0], err)
	}

	s.send("quit")
	s.waitExit()
}

func TestDebugEcho(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("debug on")
	s.send("isready")
	s.waitForPrefix("info string isready")
	s.waitForPrefix("readyok")
	s.send("debug off")
	s.send("isready")
	s.waitForPrefix("readyok")
	s.send("quit")
	s.waitExit()
}

func TestHashOptionResizeAndClear(t *testing.T) {
	s := newTestSession(t)
	s.handshake()

	s.send("setoption name Hash value 1")
	s.send("position startpos")
	s.send("go depth 3")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())

	s.send("setoption name Clear Hash")
	s.send("go depth 3")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())

	s.send("setoption name Hash value 32")
	s.send("go depth 3")
	s.assertBestmoveLegal(chess.NewStartingPosition().FEN())

	s.send("setoption name Hash value nope")
	s.waitForPrefix("info string invalid Hash value")

	s.send("quit")
	s.waitExit()
}

func TestStdoutContainsOnlyProtocol(t *testing.T) {
	s := newTestSession(t)
	s.handshake()
	s.send("position startpos")
	s.send("go depth 2")
	s.expectBestmovePrefix()
	s.send("quit")
	s.waitExit()
	s.drain()

	allowed := []string{"id ", "option ", "uciok", "readyok", "info ", "bestmove "}
	for _, line := range s.seen {
		ok := false
		for _, prefix := range allowed {
			if strings.HasPrefix(line, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("non-protocol stdout line: %q", line)
		}
	}
}

func (s *testSession) expectBestmovePrefix() {
	s.t.Helper()
	s.waitForPrefix("bestmove ")
}
