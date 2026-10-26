package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethankuehler/gochess/chess"
)

func TestSelectOpenings(t *testing.T) {
	book := make([]opening, 12)
	for i := range book {
		book[i] = opening{Name: fmt.Sprintf("line-%02d", i), Moves: []string{"e2e4"}}
	}

	first := selectOpenings(book, 5, 42)
	second := selectOpenings(book, 5, 42)
	if len(first) != 5 {
		t.Fatalf("selected %d openings, want 5", len(first))
	}
	seen := make(map[string]bool)
	for i, line := range first {
		if line.Name != second[i].Name {
			t.Fatalf("selection is not deterministic: %v vs %v", openingNames(first), openingNames(second))
		}
		if seen[line.Name] {
			t.Fatalf("selection contains duplicate %q", line.Name)
		}
		seen[line.Name] = true
	}

	if all := selectOpenings(book, 99, 42); len(all) != len(book) {
		t.Fatalf("over-large selection has %d lines, want %d", len(all), len(book))
	}
	if all := selectOpenings(book, 0, 42); len(all) != len(book) {
		t.Fatalf("zero selection has %d lines, want all %d", len(all), len(book))
	}
}

func TestEngineAccumulator(t *testing.T) {
	var acc engineAccumulator
	acc.games = 2
	acc.addSearch(engineSearchResult{Nodes: 1000, TimeMS: 100, Depth: 5, SelDepth: 7}, 0)
	acc.addSearch(engineSearchResult{Nodes: 3000, TimeMS: 300, Depth: 6, SelDepth: 8}, 0)

	stats := acc.snapshot()
	if stats.Games != 2 || stats.Moves != 2 {
		t.Errorf("games/moves = %d/%d, want 2/2", stats.Games, stats.Moves)
	}
	if stats.Nodes != 4000 {
		t.Errorf("nodes = %d, want 4000", stats.Nodes)
	}
	if stats.NPS != 10000 {
		t.Errorf("nps = %d, want 10000", stats.NPS)
	}
	if stats.AvgDepth != 5.5 {
		t.Errorf("avg depth = %v, want 5.5", stats.AvgDepth)
	}
	if stats.MaxDepth != 6 {
		t.Errorf("max depth = %d, want 6", stats.MaxDepth)
	}
	if stats.AvgSelDepth != 7.5 {
		t.Errorf("avg seldepth = %v, want 7.5", stats.AvgSelDepth)
	}
	if stats.AvgMS != 200 {
		t.Errorf("avg ms/move = %v, want 200", stats.AvgMS)
	}
	if stats.NodesPerMove != 2000 {
		t.Errorf("nodes/move = %v, want 2000", stats.NodesPerMove)
	}
	if stats.P50MS != 100 || stats.P95MS != 300 {
		t.Errorf("p50/p95 = %d/%d, want 100/300", stats.P50MS, stats.P95MS)
	}
}

func TestParseEngineInfo(t *testing.T) {
	fields := strings.Fields("info depth 6 seldepth 9 score cp 42 nodes 12345 nps 67890 time 181 hashfull 12 tbhits 3 pv e2e4 e7e5")
	var res engineSearchResult
	parseEngineInfo(fields, &res)
	if res.Depth != 6 || res.SelDepth != 9 || res.Nodes != 12345 || res.NPS != 67890 || res.TimeMS != 181 {
		t.Errorf("parsed result = %+v", res)
	}
	if res.Score != 42 || res.HasMate {
		t.Errorf("score = %+v, want cp 42", res)
	}
	if res.HashFull != 12 || res.TBHits != 3 {
		t.Errorf("hashfull/tbhits = %d/%d", res.HashFull, res.TBHits)
	}
	if len(res.PV) != 2 || res.PV[0] != "e2e4" || res.PV[1] != "e7e5" {
		t.Errorf("pv = %v", res.PV)
	}

	var mate engineSearchResult
	parseEngineInfo(strings.Fields("info depth 4 score mate -3 nodes 100 time 10"), &mate)
	if !mate.HasMate || mate.MateIn != -3 {
		t.Errorf("mate result = %+v", mate)
	}

	var ignored engineSearchResult
	parseEngineInfo(strings.Fields("info string initialized"), &ignored)
	if ignored.Depth != 0 || ignored.Nodes != 0 || len(ignored.PV) != 0 {
		t.Errorf("info string changed result: %+v", ignored)
	}
}

func TestCandidateColorSplit(t *testing.T) {
	records := []versionGameRecord{
		{CandidateWhite: true, Result: "1-0"},
		{CandidateWhite: true, Result: "1/2-1/2"},
		{CandidateWhite: false, Result: "1-0"},
	}
	white := candidateColorSplit(records, true)
	if white.Games != 2 || white.Wins != 1 || white.Draws != 1 || white.Score != 0.75 {
		t.Errorf("white split = %+v", white)
	}
	black := candidateColorSplit(records, false)
	if black.Games != 1 || black.Losses != 1 || black.Score != 0 {
		t.Errorf("black split = %+v", black)
	}
}

type fakeEngine struct {
	label          string
	requestedDepth int
	fen            string
	moves          []string
}

func (f *fakeEngine) newGame() error { return nil }

func (f *fakeEngine) setPosition(fen string, moves []string) error {
	f.fen = fen
	f.moves = append([]string(nil), moves...)
	return nil
}

func (f *fakeEngine) goDepth(depth int) (engineSearchResult, error) {
	f.requestedDepth = depth
	board, err := chess.ParseFEN(f.fen)
	if err != nil {
		return engineSearchResult{}, err
	}
	for _, move := range f.moves {
		board, _, err = board.ApplyUCIMove(move)
		if err != nil {
			return engineSearchResult{}, err
		}
	}
	legal := board.LegalMoves()
	if len(legal) == 0 {
		return engineSearchResult{}, fmt.Errorf("no legal move")
	}
	return engineSearchResult{
		BestMove: legal[0].String(),
		Depth:    depth,
		SelDepth: depth + 1,
		Nodes:    100,
		NPS:      100000,
		TimeMS:   1,
	}, nil
}

func TestPlayVersionGame(t *testing.T) {
	line := opening{
		Name:  "test-opening",
		Moves: []string{"e2e4", "e7e5", "g1f3", "b8c6"},
	}
	candidate := &fakeEngine{label: "candidate"}
	baseline := &fakeEngine{label: "baseline"}
	cfg := versionGameConfig{depth: 3, maxPlies: 20}

	rec, candidateAcc, baselineAcc, err := playVersionGame(cfg, line, true, candidate, baseline, &strings.Builder{})
	if err != nil {
		t.Fatalf("playVersionGame: %v", err)
	}
	if rec.Opening != line.Name || !rec.CandidateWhite {
		t.Errorf("record header = %+v", rec)
	}
	if rec.Plies < len(line.Moves) || rec.Plies > cfg.maxPlies {
		t.Errorf("plies = %d, want between %d and %d", rec.Plies, len(line.Moves), cfg.maxPlies)
	}
	if rec.Result != "1-0" && rec.Result != "0-1" && rec.Result != "1/2-1/2" {
		t.Errorf("result = %q", rec.Result)
	}
	if rec.Reason == "" {
		t.Error("reason is empty")
	}
	if candidate.requestedDepth != cfg.depth || baseline.requestedDepth != cfg.depth {
		t.Errorf("requested depths = %d/%d, want %d", candidate.requestedDepth, baseline.requestedDepth, cfg.depth)
	}
	candidateStats := candidateAcc.snapshot()
	baselineStats := baselineAcc.snapshot()
	if candidateStats.Games != 1 || baselineStats.Games != 1 {
		t.Errorf("game counts = %d/%d, want 1/1", candidateStats.Games, baselineStats.Games)
	}
	if candidateStats.Moves+baselineStats.Moves != rec.Plies-len(line.Moves) {
		t.Errorf("engine moves = %d+%d, want %d", candidateStats.Moves, baselineStats.Moves, rec.Plies-len(line.Moves))
	}
	if candidateStats.Nodes == 0 || baselineStats.Nodes == 0 {
		t.Errorf("nodes = %d/%d, want non-zero", candidateStats.Nodes, baselineStats.Nodes)
	}
	if len(rec.Moves) != rec.Plies {
		t.Errorf("recorded moves = %d, want %d", len(rec.Moves), rec.Plies)
	}
}

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "src.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	for name, body := range map[string]string{
		"go.mod":          "module example\n",
		"cmd/uci/main.go": "package main\n",
	} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}

	dst := filepath.Join(dir, "src")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := extractZip(archive, dst); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dst, "cmd", "uci", "main.go"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(body) != "package main\n" {
		t.Errorf("extracted body = %q", body)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bad.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../escape")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write([]byte("bad")); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}

	dst := filepath.Join(dir, "src")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := extractZip(archive, dst); err == nil {
		t.Fatal("extractZip accepted a traversal path")
	}
}
