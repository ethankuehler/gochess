package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/data"
)

// Command arena versions pits two gochess builds against each other from a
// random sample of opening-book lines, at a fixed search depth. It is a
// regression harness rather than a rating harness: equal depth makes the
// comparison cheap and reproducible, while the reported Elo difference is
// anchored to the baseline build's known rating.

type engineIdentity struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	SHA256    string `json:"sha256,omitempty"`
	BuildRef  string `json:"build_ref,omitempty"`
	AutoBuilt bool   `json:"auto_built"`
}

type engineStats struct {
	Games        int     `json:"games"`
	Moves        int     `json:"moves"`
	Nodes        uint64  `json:"nodes"`
	SearchMS     int64   `json:"search_ms"`
	NPS          uint64  `json:"nps"`
	AvgDepth     float64 `json:"avg_depth"`
	MaxDepth     int     `json:"max_depth"`
	AvgSelDepth  float64 `json:"avg_seldepth"`
	AvgMS        float64 `json:"avg_ms_per_move"`
	P50MS        int64   `json:"p50_ms"`
	P95MS        int64   `json:"p95_ms"`
	NodesPerMove float64 `json:"nodes_per_move"`
	MissingNodes int     `json:"searches_missing_nodes"`
}

type engineAccumulator struct {
	games        int
	moves        int
	nodes        uint64
	searchMS     int64
	depths       []int
	selDepths    []int
	times        []int64
	maxDepth     int
	missingNodes int
}

func (a *engineAccumulator) addSearch(res engineSearchResult, fallbackMS int64) {
	a.moves++
	if res.Nodes == 0 {
		a.missingNodes++
	}
	a.nodes += res.Nodes

	ms := res.TimeMS
	if ms <= 0 {
		ms = fallbackMS
	}
	if ms < 0 {
		ms = 0
	}
	a.searchMS += ms
	a.times = append(a.times, ms)

	if res.Depth > 0 {
		a.depths = append(a.depths, res.Depth)
		if res.Depth > a.maxDepth {
			a.maxDepth = res.Depth
		}
	}
	if res.SelDepth > 0 {
		a.selDepths = append(a.selDepths, res.SelDepth)
	}
}

func (a *engineAccumulator) merge(other engineAccumulator) {
	a.games += other.games
	a.moves += other.moves
	a.nodes += other.nodes
	a.searchMS += other.searchMS
	a.depths = append(a.depths, other.depths...)
	a.selDepths = append(a.selDepths, other.selDepths...)
	a.times = append(a.times, other.times...)
	if other.maxDepth > a.maxDepth {
		a.maxDepth = other.maxDepth
	}
	a.missingNodes += other.missingNodes
}

func (a *engineAccumulator) snapshot() engineStats {
	stats := engineStats{
		Games:        a.games,
		Moves:        a.moves,
		Nodes:        a.nodes,
		SearchMS:     a.searchMS,
		MaxDepth:     a.maxDepth,
		MissingNodes: a.missingNodes,
	}
	if a.searchMS > 0 {
		stats.NPS = a.nodes * 1000 / uint64(a.searchMS)
	}
	if a.moves > 0 {
		stats.AvgMS = float64(a.searchMS) / float64(a.moves)
		stats.NodesPerMove = float64(a.nodes) / float64(a.moves)
	}
	if len(a.depths) > 0 {
		var total int
		for _, depth := range a.depths {
			total += depth
		}
		stats.AvgDepth = float64(total) / float64(len(a.depths))
	}
	if len(a.selDepths) > 0 {
		var total int
		for _, depth := range a.selDepths {
			total += depth
		}
		stats.AvgSelDepth = float64(total) / float64(len(a.selDepths))
	}
	if len(a.times) > 0 {
		times := append([]int64(nil), a.times...)
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		stats.P50MS = percentile(times, 0.50)
		stats.P95MS = percentile(times, 0.95)
	}
	return stats
}

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

type colorSplit struct {
	Games  int     `json:"games"`
	Wins   int     `json:"wins"`
	Draws  int     `json:"draws"`
	Losses int     `json:"losses"`
	Score  float64 `json:"score"`
}

type versionGameRecord struct {
	Opening        string      `json:"opening"`
	CandidateWhite bool        `json:"candidate_white"`
	Result         string      `json:"result"`
	Reason         string      `json:"reason"`
	Plies          int         `json:"plies"`
	Moves          []string    `json:"moves"`
	Candidate      engineStats `json:"candidate_stats"`
	Baseline       engineStats `json:"baseline_stats"`
}

type versionReport struct {
	Schema      int            `json:"schema"`
	Candidate   engineIdentity `json:"candidate"`
	Baseline    engineIdentity `json:"baseline"`
	AnchorElo   *float64       `json:"anchor_elo,omitempty"`
	Depth       int            `json:"depth"`
	Book        string         `json:"book"`
	BookLines   int            `json:"book_lines"`
	Openings    []string       `json:"openings"`
	Seed        uint64         `json:"seed"`
	Games       int            `json:"games"`
	Wins        int            `json:"wins"`
	Draws       int            `json:"draws"`
	Losses      int            `json:"losses"`
	Score       float64        `json:"score"`
	DrawRate    float64        `json:"draw_rate"`
	AvgPlies    float64        `json:"avg_plies"`
	NodeRatio   float64        `json:"node_ratio,omitempty"`
	NPSRatio    float64        `json:"nps_ratio,omitempty"`
	TimeRatio   float64        `json:"time_ratio,omitempty"`
	Elo         *float64       `json:"elo"`
	EloLow      *float64       `json:"elo_low"`
	EloHigh     *float64       `json:"elo_high"`
	ImpliedElo  *float64       `json:"implied_elo,omitempty"`
	ImpliedLow  *float64       `json:"implied_elo_low,omitempty"`
	ImpliedHigh *float64       `json:"implied_elo_high,omitempty"`
	Elapsed     string         `json:"elapsed"`
	Endings     map[string]int `json:"endings"`

	CandidateWhite colorSplit          `json:"candidate_white"`
	CandidateBlack colorSplit          `json:"candidate_black"`
	CandidateStats engineStats         `json:"candidate_stats"`
	BaselineStats  engineStats         `json:"baseline_stats"`
	GamesLog       []versionGameRecord `json:"games_log"`
}

type builtEngine struct {
	path    string
	source  string
	ref     string
	cleanup func()
}

type versionGameConfig struct {
	depth    int
	maxPlies int
	verbose  bool
}

// enginePlayer is the part of an engine the game loop needs. It keeps the
// fixed-depth version games testable with a fake engine.
type enginePlayer interface {
	newGame() error
	setPosition(fen string, moves []string) error
	goDepth(depth int) (engineSearchResult, error)
}

func runVersions(args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("arena versions", flag.ContinueOnError)
	flags.SetOutput(errOut)

	enginePath := flags.String("engine", "", "candidate UCI binary (default: build the working tree)")
	baselinePath := flags.String("baseline", "", "baseline UCI binary (default: build -baseline-ref)")
	baselineRef := flags.String("baseline-ref", "HEAD", "git ref to build when -baseline is empty")
	bookPath := flags.String("book", "data/openings.csv", "opening book CSV")
	openings := flags.Int("openings", 20, "random opening lines to play (two games each)")
	seed := flags.Uint64("seed", 1, "random seed for opening selection")
	depth := flags.Int("depth", 5, "fixed search depth in plies for both engines")
	anchorElo := flags.Float64("anchor-elo", 1640, "baseline rating used for implied Elo (0 disables)")
	maxPlies := flags.Int("max-plies", 300, "maximum game length in plies")
	jsonPath := flags.String("json", "", "write a JSON report here")
	verbose := flags.Bool("verbose", false, "print every move")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *openings < 1 {
		return fmt.Errorf("openings must be at least 1")
	}
	if *depth < 1 || *depth > 64 {
		return fmt.Errorf("depth must be between 1 and 64")
	}
	if *anchorElo < 0 {
		return fmt.Errorf("anchor-elo must not be negative")
	}
	if *maxPlies < 1 {
		return fmt.Errorf("max-plies must be at least 1")
	}

	// loadBook replays the lines, which needs legal move generation.
	if err := chess.BuildAllAttacksFromFS(data.FS); err != nil {
		return fmt.Errorf("initialize attack tables: %w", err)
	}
	book, err := loadBook(*bookPath)
	if err != nil {
		return err
	}
	bookLines := len(book)
	if *openings > bookLines {
		fmt.Fprintf(out, "note: book has %d lines; playing all of them\n", bookLines)
		*openings = bookLines
	}
	selected := selectOpenings(book, *openings, *seed)

	candidatePath := *enginePath
	var candidateBuild *builtEngine
	if candidatePath == "" {
		fmt.Fprintln(out, "building candidate from working tree...")
		candidateBuild, err = buildWorkingTree()
		if err != nil {
			return err
		}
		candidatePath = candidateBuild.path
		defer candidateBuild.cleanup()
	}

	baselinePathValue := *baselinePath
	var baselineBuild *builtEngine
	if baselinePathValue == "" {
		fmt.Fprintf(out, "building baseline from %s...\n", *baselineRef)
		baselineBuild, err = buildGitRef(*baselineRef)
		if err != nil {
			return err
		}
		baselinePathValue = baselineBuild.path
		defer baselineBuild.cleanup()
	}

	candidate, err := startEngine(candidatePath, nil, errOut)
	if err != nil {
		return err
	}
	defer candidate.Close()
	baseline, err := startEngine(baselinePathValue, nil, errOut)
	if err != nil {
		return err
	}
	defer baseline.Close()

	candidateSource := candidatePath
	if candidateBuild != nil {
		candidateSource = candidateBuild.source
	}
	baselineSource := baselinePathValue
	if baselineBuild != nil {
		baselineSource = baselineBuild.source
	}

	candidateInfo := engineIdentity{
		Name:      candidate.label(),
		Source:    candidateSource,
		SHA256:    hashFile(candidatePath),
		AutoBuilt: candidateBuild != nil,
	}
	if candidateBuild != nil {
		candidateInfo.BuildRef = candidateBuild.ref
	}
	baselineInfo := engineIdentity{
		Name:      baseline.label(),
		Source:    baselineSource,
		SHA256:    hashFile(baselinePathValue),
		AutoBuilt: baselineBuild != nil,
	}
	if baselineBuild != nil {
		baselineInfo.BuildRef = baselineBuild.ref
	}

	anchor := "none"
	if *anchorElo > 0 {
		anchor = fmt.Sprintf("%.0f", *anchorElo)
	}
	fmt.Fprintf(out, "arena versions: candidate vs baseline\n")
	fmt.Fprintf(out, "  candidate: %s (%s)\n", candidateInfo.Name, candidateSource)
	fmt.Fprintf(out, "  baseline:  %s (%s)\n", baselineInfo.Name, baselineSource)
	fmt.Fprintf(out, "  anchor:    %s\n", anchor)
	fmt.Fprintf(out, "  depth:     %d plies (both engines)\n", *depth)
	fmt.Fprintf(out, "  book:      %s (%d random of %d lines, seed %d)\n", *bookPath, len(selected), bookLines, *seed)
	fmt.Fprintf(out, "  games:     %d (each opening played twice, colours swapped)\n", len(selected)*2)
	fmt.Fprintf(out, "  max plies: %d\n", *maxPlies)
	if candidateInfo.SHA256 != "" && candidateInfo.SHA256 == baselineInfo.SHA256 {
		fmt.Fprintln(out, "  note: candidate and baseline binaries are identical (self-play)")
	}
	fmt.Fprintln(out)

	cfg := versionGameConfig{
		depth:    *depth,
		maxPlies: *maxPlies,
		verbose:  *verbose,
	}

	var (
		records        []versionGameRecord
		pairScores     []float64
		wins           int
		draws          int
		losses         int
		totalPlies     int
		endings        = make(map[string]int)
		candidateTotal engineAccumulator
		baselineTotal  engineAccumulator
	)
	started := time.Now()

	for _, line := range selected {
		pair := 0.0
		for _, candidateWhite := range []bool{true, false} {
			rec, candidateGame, baselineGame, err := playVersionGame(cfg, line, candidateWhite, candidate, baseline, out)
			if err != nil {
				return fmt.Errorf("game %d: %w", len(records)+1, err)
			}
			records = append(records, rec)
			candidateTotal.merge(candidateGame)
			baselineTotal.merge(baselineGame)
			endings[rec.Reason]++
			totalPlies += rec.Plies

			gameScore := ourScore(rec.Result, rec.CandidateWhite)
			pair += gameScore
			switch gameScore {
			case 1:
				wins++
			case 0.5:
				draws++
			case 0:
				losses++
			}

			colour := "Black"
			if rec.CandidateWhite {
				colour = "White"
			}
			fmt.Fprintf(out, "game %3d  %-28s  candidate %-5s  %-7s  %-20s  %3d plies\n",
				len(records), rec.Opening, colour, rec.Result, rec.Reason, rec.Plies)
		}
		pairScores = append(pairScores, pair)
	}

	elapsed := time.Since(started)
	elo, low, high := bootstrapElo(pairScores, *seed)
	candidateStats := candidateTotal.snapshot()
	baselineStats := baselineTotal.snapshot()
	candidateWhite := candidateColorSplit(records, true)
	candidateBlack := candidateColorSplit(records, false)
	nodeRatio := ratio(candidateStats.Nodes, baselineStats.Nodes)
	npsRatio := ratio(candidateStats.NPS, baselineStats.NPS)
	timeRatio := ratio(uint64(candidateStats.SearchMS), uint64(baselineStats.SearchMS))
	overallScore := score(wins, draws, losses)
	drawRate := 0.0
	avgPlies := 0.0
	if len(records) > 0 {
		drawRate = float64(draws) / float64(len(records))
		avgPlies = float64(totalPlies) / float64(len(records))
	}

	var (
		anchorValue    *float64
		implied        *float64
		impliedLow     *float64
		impliedHigh    *float64
		impliedValue   float64
		impliedLowVal  float64
		impliedHighVal float64
	)
	if *anchorElo > 0 {
		value := *anchorElo
		anchorValue = &value
		impliedValue = value + elo
		impliedLowVal = value + low
		impliedHighVal = value + high
		implied = jsonFloat(impliedValue)
		impliedLow = jsonFloat(impliedLowVal)
		impliedHigh = jsonFloat(impliedHighVal)
	}

	fmt.Fprintf(out, "\nsummary: %d games, %d wins, %d draws, %d losses, score %.3f\n",
		len(records), wins, draws, losses, overallScore)
	fmt.Fprintf(out, "colour split (candidate): White %.1f/%d points (score %.3f), Black %.1f/%d points (score %.3f)\n",
		candidatePoints(candidateWhite), candidateWhite.Games, candidateWhite.Score,
		candidatePoints(candidateBlack), candidateBlack.Games, candidateBlack.Score)
	fmt.Fprintf(out, "average game length: %.1f plies, draw rate %.1f%%\n", avgPlies, drawRate*100)
	fmt.Fprintf(out, "Elo difference (candidate - baseline): %s (95%% CI %s..%s)\n",
		formatElo(elo), formatElo(low), formatElo(high))
	if math.IsInf(elo, 0) {
		fmt.Fprintln(out, "note: every game had the same result, so the Elo difference is unbounded; play more openings")
	}
	if anchorValue != nil {
		fmt.Fprintf(out, "implied candidate rating: %s (95%% CI %s..%s), assuming baseline = %.0f\n",
			formatRating(impliedValue), formatRating(impliedLowVal), formatRating(impliedHighVal), *anchorValue)
		fmt.Fprintln(out, "anchor note: this is an anchor-based estimate, not a new Lichess measurement")
	}
	fmt.Fprintf(out, "wall time: %s\n", elapsed.Round(time.Second))
	if len(endings) > 0 {
		fmt.Fprintf(out, "endings:")
		for _, reason := range sortedKeys(endings) {
			fmt.Fprintf(out, " %s=%d", reason, endings[reason])
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintf(out, "\nmetrics (fixed depth %d):\n", *depth)
	fmt.Fprintf(out, "  %-10s %6s %12s %12s %10s %8s %10s %8s\n",
		"engine", "moves", "nodes", "search time", "nps", "avg d", "ms/move", "p95")
	printEngineStats(out, "candidate", candidateStats)
	printEngineStats(out, "baseline", baselineStats)
	if nodeRatio > 0 && npsRatio > 0 && timeRatio > 0 {
		fmt.Fprintf(out, "  candidate/baseline resource ratio: nodes %.2fx, nps %.2fx, search time %.2fx\n",
			nodeRatio, npsRatio, timeRatio)
	}
	if candidateStats.MissingNodes > 0 || baselineStats.MissingNodes > 0 {
		fmt.Fprintf(out, "  note: missing node counts: candidate=%d baseline=%d\n",
			candidateStats.MissingNodes, baselineStats.MissingNodes)
	}

	if *jsonPath != "" {
		rep := versionReport{
			Schema:         1,
			Candidate:      candidateInfo,
			Baseline:       baselineInfo,
			AnchorElo:      anchorValue,
			Depth:          *depth,
			Book:           *bookPath,
			BookLines:      bookLines,
			Openings:       openingNames(selected),
			Seed:           *seed,
			Games:          len(records),
			Wins:           wins,
			Draws:          draws,
			Losses:         losses,
			Score:          overallScore,
			DrawRate:       drawRate,
			AvgPlies:       avgPlies,
			NodeRatio:      nodeRatio,
			NPSRatio:       npsRatio,
			TimeRatio:      timeRatio,
			Elo:            jsonFloat(elo),
			EloLow:         jsonFloat(low),
			EloHigh:        jsonFloat(high),
			ImpliedElo:     implied,
			ImpliedLow:     impliedLow,
			ImpliedHigh:    impliedHigh,
			Elapsed:        elapsed.Round(time.Second).String(),
			Endings:        endings,
			CandidateWhite: candidateWhite,
			CandidateBlack: candidateBlack,
			CandidateStats: candidateStats,
			BaselineStats:  baselineStats,
			GamesLog:       records,
		}
		if err := writeReport(*jsonPath, rep); err != nil {
			return err
		}
		fmt.Fprintf(out, "report: %s\n", *jsonPath)
	}

	return nil
}

func playVersionGame(cfg versionGameConfig, line opening, candidateWhite bool, candidate, baseline enginePlayer, out io.Writer) (versionGameRecord, engineAccumulator, engineAccumulator, error) {
	startFEN := chess.NewStartingPosition().FEN()
	board := chess.NewStartingPosition()

	var (
		history []string
		keys    []chess.PositionKey
	)
	for _, move := range line.Moves {
		keys = append(keys, board.Key())
		next, _, err := board.ApplyUCIMove(move)
		if err != nil {
			return versionGameRecord{}, engineAccumulator{}, engineAccumulator{}, fmt.Errorf("book %q move %s: %w", line.Name, move, err)
		}
		history = append(history, move)
		board = next
	}

	if err := candidate.newGame(); err != nil {
		return versionGameRecord{}, engineAccumulator{}, engineAccumulator{}, fmt.Errorf("candidate ucinewgame: %w", err)
	}
	if err := baseline.newGame(); err != nil {
		return versionGameRecord{}, engineAccumulator{}, engineAccumulator{}, fmt.Errorf("baseline ucinewgame: %w", err)
	}

	var (
		candidateMoves engineAccumulator
		baselineMoves  engineAccumulator
	)
	candidateMoves.games = 1
	baselineMoves.games = 1
	rec := versionGameRecord{Opening: line.Name, CandidateWhite: candidateWhite}

	finish := func(result, reason string, plies int) (versionGameRecord, engineAccumulator, engineAccumulator, error) {
		rec.Result = result
		rec.Reason = reason
		rec.Plies = plies
		rec.Moves = append([]string(nil), history...)
		rec.Candidate = candidateMoves.snapshot()
		rec.Baseline = baselineMoves.snapshot()
		return rec, candidateMoves, baselineMoves, nil
	}

	for ply := len(history); ; ply++ {
		if len(board.LegalMoves()) == 0 {
			if board.InCheck() {
				result := "1-0"
				if board.SideToMove() == chess.White {
					result = "0-1"
				}
				return finish(result, "checkmate", ply)
			}
			return finish("1/2-1/2", "stalemate", ply)
		}
		if reason := drawReason(board, keys); reason != "" {
			return finish("1/2-1/2", reason, ply)
		}
		if ply >= cfg.maxPlies {
			return finish("1/2-1/2", "max-plies", ply)
		}

		candidateTurn := (board.SideToMove() == chess.White) == candidateWhite
		player := baseline
		label := "baseline"
		stats := &baselineMoves
		if candidateTurn {
			player = candidate
			label = "candidate"
			stats = &candidateMoves
		}

		if err := player.setPosition(startFEN, history); err != nil {
			return versionGameRecord{}, engineAccumulator{}, engineAccumulator{}, fmt.Errorf("%s position: %w", label, err)
		}
		start := time.Now()
		res, err := player.goDepth(cfg.depth)
		elapsed := time.Since(start)
		if err != nil {
			return versionGameRecord{}, engineAccumulator{}, engineAccumulator{}, fmt.Errorf("%s search: %w", label, err)
		}
		stats.addSearch(res, elapsed.Milliseconds())

		if res.BestMove == "" {
			return versionGameRecord{}, engineAccumulator{}, engineAccumulator{}, fmt.Errorf("%s returned no move in %s", label, board.FEN())
		}
		parsed, err := chess.ParseUCIMove(res.BestMove)
		if err != nil {
			return versionGameRecord{}, engineAccumulator{}, engineAccumulator{}, fmt.Errorf("%s returned malformed move %q: %w", label, res.BestMove, err)
		}
		if _, ok := board.FindLegalMove(*parsed); !ok {
			return versionGameRecord{}, engineAccumulator{}, engineAccumulator{}, fmt.Errorf("%s returned illegal move %q in %s", label, res.BestMove, board.FEN())
		}
		if cfg.verbose {
			fmt.Fprintf(out, "  %-9s %-6s %-10s depth %2d  nodes %8d  nps %8d  %s\n",
				label, res.BestMove, formatSearchScore(res), res.Depth, res.Nodes, res.NPS, formatMS(res.TimeMS))
		}

		keys = append(keys, board.Key())
		next, _, err := board.ApplyUCIMove(res.BestMove)
		if err != nil {
			return versionGameRecord{}, engineAccumulator{}, engineAccumulator{}, fmt.Errorf("%s move %q: %w", label, res.BestMove, err)
		}
		history = append(history, res.BestMove)
		board = next
	}
}

func selectOpenings(book []opening, n int, seed uint64) []opening {
	if n <= 0 || n > len(book) {
		n = len(book)
	}
	rng := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	perm := rng.Perm(len(book))
	selected := make([]opening, n)
	for i := range selected {
		selected[i] = book[perm[i]]
	}
	return selected
}

func openingNames(openings []opening) []string {
	names := make([]string, len(openings))
	for i, line := range openings {
		names[i] = line.Name
	}
	return names
}

func candidateColorSplit(records []versionGameRecord, white bool) colorSplit {
	var split colorSplit
	for _, rec := range records {
		if rec.CandidateWhite != white {
			continue
		}
		split.Games++
		switch ourScore(rec.Result, rec.CandidateWhite) {
		case 1:
			split.Wins++
		case 0.5:
			split.Draws++
		case 0:
			split.Losses++
		}
	}
	split.Score = score(split.Wins, split.Draws, split.Losses)
	return split
}

func ratio(a, b uint64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func candidatePoints(split colorSplit) float64 {
	return float64(split.Wins) + 0.5*float64(split.Draws)
}

func formatSearchScore(res engineSearchResult) string {
	if res.HasMate {
		return fmt.Sprintf("mate %+d", res.MateIn)
	}
	return fmt.Sprintf("cp %+d", res.Score)
}

func printEngineStats(out io.Writer, label string, stats engineStats) {
	fmt.Fprintf(out, "  %-10s %6d %12s %12s %10s %8.1f %10s %8s\n",
		label,
		stats.Moves,
		formatCount(stats.Nodes),
		formatMS(stats.SearchMS),
		formatCount(stats.NPS),
		stats.AvgDepth,
		formatAvgMS(stats.AvgMS),
		formatMS(stats.P95MS),
	)
}

func formatCount(value uint64) string {
	switch {
	case value >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", float64(value)/1_000_000_000)
	case value >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(value)/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("%.1fK", float64(value)/1_000)
	default:
		return fmt.Sprintf("%d", value)
	}
}

func formatAvgMS(ms float64) string {
	if ms < 1000 {
		return fmt.Sprintf("%.1fms", ms)
	}
	return formatMS(int64(ms))
}

func formatMS(ms int64) string {
	if ms <= 0 {
		return "0ms"
	}
	duration := time.Duration(ms) * time.Millisecond
	if duration < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	return duration.Round(time.Millisecond).String()
}

func formatRating(value float64) string {
	switch {
	case math.IsInf(value, 1):
		return "+inf"
	case math.IsInf(value, -1):
		return "-inf"
	case math.IsNaN(value):
		return "n/a"
	default:
		return fmt.Sprintf("%.0f", value)
	}
}

func writeReport(path string, rep versionReport) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create report directory: %w", err)
		}
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

func buildWorkingTree() (*builtEngine, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	version := gitVersion(root)
	tmp, err := os.MkdirTemp("", "gochess-arena-worktree-*")
	if err != nil {
		return nil, fmt.Errorf("create build directory: %w", err)
	}
	bin := engineBinaryPath(tmp)
	if err := goBuild(root, bin, version); err != nil {
		_ = os.RemoveAll(tmp)
		return nil, err
	}
	return &builtEngine{
		path:    bin,
		source:  "working tree",
		ref:     version,
		cleanup: func() { _ = os.RemoveAll(tmp) },
	}, nil
}

func buildGitRef(ref string) (*builtEngine, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "-") {
		return nil, fmt.Errorf("invalid git ref %q", ref)
	}
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	commit, err := gitOutput(root, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return nil, err
	}
	short, err := gitOutput(root, "rev-parse", "--short", commit)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "gochess-arena-ref-*")
	if err != nil {
		return nil, fmt.Errorf("create build directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }

	archive := filepath.Join(tmp, "src.zip")
	cmd := exec.Command("git", "archive", "--format=zip", "--output="+archive, commit)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return nil, fmt.Errorf("git archive %s: %w\n%s", ref, err, strings.TrimSpace(string(output)))
	}
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		cleanup()
		return nil, fmt.Errorf("create source directory: %w", err)
	}
	if err := extractZip(archive, src); err != nil {
		cleanup()
		return nil, err
	}
	bin := engineBinaryPath(tmp)
	if err := goBuild(src, bin, "arena-"+short); err != nil {
		cleanup()
		return nil, err
	}
	return &builtEngine{
		path:    bin,
		source:  "git:" + ref,
		ref:     short,
		cleanup: cleanup,
	}, nil
}

func engineBinaryPath(dir string) string {
	name := "gochess"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}

func repoRoot() (string, error) {
	return gitOutput("", "rev-parse", "--show-toplevel")
}

func gitVersion(dir string) string {
	version, err := gitOutput(dir, "describe", "--always", "--dirty")
	if err != nil {
		return "worktree"
	}
	return versionStamp(version)
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func goBuild(dir, out, version string) error {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("find go: %w", err)
	}
	ldflags := "-X github.com/ethankuehler/gochess/uci.Version=" + versionStamp(version)
	cmd := exec.Command(goBin, "build", "-trimpath", "-ldflags", ldflags, "-o", out, "./cmd/uci")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build UCI engine in %s: %w\n%s", dir, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func versionStamp(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "worktree"
	}
	return strings.NewReplacer(" ", "-", "\t", "-", "\n", "-").Replace(value)
}

func extractZip(path, dst string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open source archive: %w", err)
	}
	defer reader.Close()

	cleanDst, err := filepath.Abs(dst)
	if err != nil {
		return fmt.Errorf("resolve source directory: %w", err)
	}
	for _, file := range reader.File {
		name := filepath.Clean(file.Name)
		if name == "." || filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in source archive: %q", file.Name)
		}
		target := filepath.Join(cleanDst, name)
		rel, err := filepath.Rel(cleanDst, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in source archive: %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create source directory %s: %w", target, err)
			}
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsupported symlink in source archive: %q", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create source directory %s: %w", filepath.Dir(target), err)
		}
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("open source file %s: %w", file.Name, err)
		}
		dstFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			_ = src.Close()
			return fmt.Errorf("create source file %s: %w", target, err)
		}
		_, copyErr := io.Copy(dstFile, src)
		closeDstErr := dstFile.Close()
		closeSrcErr := src.Close()
		if copyErr != nil {
			return fmt.Errorf("extract source file %s: %w", file.Name, copyErr)
		}
		if closeDstErr != nil {
			return fmt.Errorf("close source file %s: %w", target, closeDstErr)
		}
		if closeSrcErr != nil {
			return fmt.Errorf("close archive file %s: %w", file.Name, closeSrcErr)
		}
	}
	return nil
}

func hashFile(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}
