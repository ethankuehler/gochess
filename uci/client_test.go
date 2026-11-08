package uci

import (
	"io"
	"testing"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

func startTestClient(t *testing.T) *Client {
	t.Helper()
	client, err := StartInProcess(io.Discard)
	if err != nil {
		t.Fatalf("StartInProcess: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestClientSearch(t *testing.T) {
	client := startTestClient(t)
	position := chess.NewStartingPosition()

	if err := client.SetPosition(position.FEN()); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	result, err := client.GoDepth(2)
	if err != nil {
		t.Fatalf("GoDepth: %v", err)
	}
	if result.BestMove == "" {
		t.Fatal("engine returned no best move")
	}
	if result.Depth != 2 {
		t.Errorf("depth = %d, want 2", result.Depth)
	}
	if result.Nodes == 0 {
		t.Error("nodes = 0")
	}
	if result.TimeMS < 1 {
		t.Errorf("time = %dms, want at least 1", result.TimeMS)
	}
	if result.NPS == 0 || result.SelDepth < result.Depth {
		t.Errorf("NPS/seldepth = %d/%d, depth = %d", result.NPS, result.SelDepth, result.Depth)
	}

	parsed, err := chess.ParseUCIMove(result.BestMove)
	if err != nil {
		t.Fatalf("bestmove %q: %v", result.BestMove, err)
	}
	if _, ok := position.FindLegalMove(*parsed); !ok {
		t.Fatalf("bestmove %q is not legal", result.BestMove)
	}

	current := position
	for i, move := range result.PV {
		next, _, err := current.ApplyUCIMove(move)
		if err != nil {
			t.Fatalf("PV move %d (%s): %v", i+1, move, err)
		}
		current = next
	}
}

func TestClientTerminalPositions(t *testing.T) {
	client := startTestClient(t)

	checkmate, err := chess.ParseFEN("7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	if err != nil {
		t.Fatalf("ParseFEN checkmate: %v", err)
	}
	if err := client.SetPosition(checkmate.FEN()); err != nil {
		t.Fatalf("SetPosition checkmate: %v", err)
	}
	result, err := client.GoDepth(2)
	if err != nil {
		t.Fatalf("GoDepth checkmate: %v", err)
	}
	if result.BestMove != "" {
		t.Errorf("checkmate bestmove = %q, want empty", result.BestMove)
	}
	if !result.HasMate || result.MateIn != 0 {
		t.Errorf("checkmate result = mate %v %d, want mate 0", result.HasMate, result.MateIn)
	}

	stalemate, err := chess.ParseFEN("7k/5Q2/6K1/8/8/8/8/8 b - - 0 1")
	if err != nil {
		t.Fatalf("ParseFEN stalemate: %v", err)
	}
	if err := client.SetPosition(stalemate.FEN()); err != nil {
		t.Fatalf("SetPosition stalemate: %v", err)
	}
	result, err = client.GoDepth(2)
	if err != nil {
		t.Fatalf("GoDepth stalemate: %v", err)
	}
	if result.BestMove != "" {
		t.Errorf("stalemate bestmove = %q, want empty", result.BestMove)
	}
	if result.HasMate {
		t.Errorf("stalemate reported mate %d", result.MateIn)
	}
}

func TestClientSetPositionErrorRecovers(t *testing.T) {
	client := startTestClient(t)

	if err := client.SetPosition("not-a-fen"); err == nil {
		t.Fatal("SetPosition accepted an invalid FEN")
	}
	if err := client.SetPosition(chess.NewStartingPosition().FEN()); err != nil {
		t.Fatalf("SetPosition after error: %v", err)
	}
	result, err := client.GoDepth(1)
	if err != nil {
		t.Fatalf("GoDepth after error: %v", err)
	}
	if result.BestMove == "" {
		t.Fatal("engine returned no move after recovering from a position error")
	}
}

func TestClientNewGameAndClose(t *testing.T) {
	client := startTestClient(t)
	if err := client.NewGame(); err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := client.SetPosition(chess.NewStartingPosition().FEN()); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	if _, err := client.GoDepth(1); err != nil {
		t.Fatalf("GoDepth: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestParseInfo(t *testing.T) {
	var result EngineResult
	parseInfo([]string{
		"info", "depth", "5", "seldepth", "7", "score", "cp", "-34",
		"nodes", "12345", "nps", "400000", "time", "31", "hashfull", "12", "tbhits", "3",
		"pv", "e2e4", "e7e5", "g1f3",
	}, &result)

	if result.Depth != 5 || result.SelDepth != 7 || result.Score != -34 || result.HasMate {
		t.Errorf("parsed result = %+v", result)
	}
	if result.Nodes != 12345 || result.NPS != 400000 || result.TimeMS != 31 {
		t.Errorf("nodes/NPS/time = %d/%d/%d", result.Nodes, result.NPS, result.TimeMS)
	}
	if result.HashFull != 12 || result.TBHits != 3 {
		t.Errorf("hashfull/tbhits = %d/%d", result.HashFull, result.TBHits)
	}
	if len(result.PV) != 3 || result.PV[0] != "e2e4" || result.PV[2] != "g1f3" {
		t.Errorf("PV = %v", result.PV)
	}

	parseInfo([]string{"info", "depth", "4", "score", "mate", "-2", "nodes", "99"}, &result)
	if !result.HasMate || result.MateIn != -2 {
		t.Errorf("mate result = %+v", result)
	}

	parseInfo([]string{"info", "string", "depth", "100", "score", "cp", "999"}, &result)
	if result.Depth != 4 || result.Score != -34 {
		t.Errorf("info string mutated the result: %+v", result)
	}
}

func TestClientSetOptionAndLimits(t *testing.T) {
	client := startTestClient(t)
	if err := client.SetOption("Move Overhead", "0"); err != nil {
		t.Fatalf("SetOption: %v", err)
	}
	position := chess.NewStartingPosition()
	if err := client.SetPosition(position.FEN()); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}

	result, err := client.GoMovetime(60 * time.Millisecond)
	if err != nil {
		t.Fatalf("GoMovetime: %v", err)
	}
	if result.BestMove == "" {
		t.Fatal("GoMovetime returned no best move")
	}
	if result.Nodes == 0 {
		t.Error("GoMovetime nodes = 0")
	}

	result, err = client.GoNodes(5000)
	if err != nil {
		t.Fatalf("GoNodes: %v", err)
	}
	if result.BestMove == "" {
		t.Fatal("GoNodes returned no best move")
	}
	if result.Nodes == 0 || result.Nodes > 50_000 {
		t.Errorf("GoNodes nodes = %d, want a bounded positive count", result.Nodes)
	}
}

type clientTestWriteCloser struct {
	io.Writer
}

func (clientTestWriteCloser) Close() error { return nil }

func clientWithOutput(lines ...string) *Client {
	output := make(chan string, len(lines))
	for _, line := range lines {
		output <- line
	}
	close(output)
	return &Client{in: clientTestWriteCloser{Writer: io.Discard}, lines: output}
}

func TestGoCommand(t *testing.T) {
	client := clientWithOutput(
		"info depth 1 seldepth 2 score cp 10 nodes 10 nps 1000 time 1 pv e2e4",
		"info string ignored",
		"info depth 1 seldepth 3 score cp 20 nodes 20 nps 2000 time 2 pv d2d4 e7e5",
		"bestmove d2d4",
	)
	result, err := client.goCommand("go depth 1")
	if err != nil {
		t.Fatalf("goCommand: %v", err)
	}
	if result.BestMove != "d2d4" || result.Depth != 1 || result.SelDepth != 3 || result.Nodes != 20 || result.NPS != 2000 || result.TimeMS != 2 {
		t.Fatalf("result = %+v", result)
	}
	if len(result.PV) != 2 || result.PV[0] != "d2d4" {
		t.Fatalf("PV = %v", result.PV)
	}
}
