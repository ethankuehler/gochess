package uci

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ethankuehler/gochess/chess"
)

func TestProcessClientHelper(t *testing.T) {
	if len(os.Args) == 0 || os.Args[len(os.Args)-1] != "process-helper" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "uci":
			fmt.Println("id name process-test-engine")
			fmt.Println("uciok")
		case line == "isready":
			fmt.Println("readyok")
		case line == "go depth 2":
			fmt.Println("info depth 2 seldepth 4 score cp 42 nodes 100 nps 10000 time 10 hashfull 12 tbhits 3 pv e2e4 e7e5")
			fmt.Println("bestmove e2e4 ponder e7e5")
		case line == "go perft 1":
			fmt.Println("a2a3: 1")
			fmt.Println("Nodes searched: 1")
		case line == "quit":
			return
		}
	}
}

func TestProcessClient(t *testing.T) {
	var errOut bytes.Buffer
	client, err := StartProcess(os.Args[0], []ProcessOption{{Name: "Threads", Value: "1"}}, &errOut,
		"-test.run=^TestProcessClientHelper$", "--", "process-helper")
	if err != nil {
		t.Fatalf("StartProcess: %v\nstderr: %s", err, errOut.String())
	}
	t.Cleanup(func() { _ = client.Close() })
	if got := client.Label(); got != "process-test-engine" {
		t.Fatalf("Label = %q, want process-test-engine", got)
	}
	if err := client.NewGame(); err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := client.SetPosition(chess.NewStartingPosition().FEN()); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	result, err := client.GoDepth(2)
	if err != nil {
		t.Fatalf("GoDepth: %v", err)
	}
	if result.BestMove != "e2e4" || result.Score != 42 || result.Depth != 2 || result.SelDepth != 4 {
		t.Errorf("search result = %+v", result)
	}
	if result.Nodes != 100 || result.NPS != 10000 || result.TimeMS != 10 || result.HashFull != 12 || result.TBHits != 3 {
		t.Errorf("search details = %+v", result)
	}
	if len(result.PV) != 2 || result.PV[0] != "e2e4" || result.PV[1] != "e7e5" {
		t.Errorf("PV = %v", result.PV)
	}
	divide, total, err := client.Perft(1)
	if err != nil {
		t.Fatalf("Perft: %v", err)
	}
	if total != 1 || divide["a2a3"] != 1 || len(divide) != 1 {
		t.Errorf("perft = %v, total %d", divide, total)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
