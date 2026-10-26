// Command arena plays gochess against Stockfish or another UCI opponent and
// reports an Elo estimate. It is deliberately separate from cmd/match: match is
// a rules verifier, while arena is a rating harness with time controls, an
// opening book, paired colours, and a bootstrap confidence interval. The
// "versions" subcommand compares two local gochess builds at fixed depth from a
// random opening sample.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/data"
	"github.com/ethankuehler/gochess/uci"
)

type report struct {
	Engine   string       `json:"engine"`
	Book     string       `json:"book"`
	Opponent string       `json:"opponent"`
	TC       string       `json:"tc"`
	Games    int          `json:"games"`
	Wins     int          `json:"wins"`
	Draws    int          `json:"draws"`
	Losses   int          `json:"losses"`
	Score    float64      `json:"score"`
	Elo      *float64     `json:"elo"`
	EloLow   *float64     `json:"elo_low"`
	EloHigh  *float64     `json:"elo_high"`
	Elapsed  string       `json:"elapsed"`
	GamesLog []gameRecord `json:"games_log"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "arena:", err)
		os.Exit(1)
	}
}

func run(args []string, out, errOut io.Writer) error {
	if len(args) > 0 && args[0] == "versions" {
		return runVersions(args[1:], out, errOut)
	}

	flags := flag.NewFlagSet("arena", flag.ContinueOnError)
	flags.SetOutput(errOut)

	enginePath := flags.String("engine", "./gochess", "gochess UCI binary")
	stockfishPath := flags.String("stockfish", "stockfish", "Stockfish binary")
	bookPath := flags.String("book", "data/openings.csv", "opening book CSV")
	pairs := flags.Int("pairs", 20, "opening pairs to play (two games per pair)")
	skip := flags.Int("skip", 0, "book lines to skip before playing")
	movetime := flags.Duration("movetime", 250*time.Millisecond, "gochess time per move")
	sfMovetime := flags.Duration("sf-movetime", 0, "Stockfish time per move (0 uses -movetime)")
	sfNodes := flags.Uint64("sf-nodes", 0, "Stockfish node limit (overrides time)")
	sfElo := flags.Int("sf-elo", 0, "Stockfish UCI_Elo (0 leaves full strength)")
	sfSkill := flags.Int("sf-skill", -1, "Stockfish Skill Level (-1 leaves it unset)")
	maxPlies := flags.Int("max-plies", 300, "maximum game length in plies")
	seed := flags.Uint64("seed", 1, "bootstrap seed")
	jsonPath := flags.String("json", "", "write a JSON report here")
	verbose := flags.Bool("verbose", false, "print every move")
	checkBook := flags.Bool("check-book", false, "validate the book and exit")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *pairs < 1 {
		return fmt.Errorf("pairs must be at least 1")
	}
	if *skip < 0 {
		return fmt.Errorf("skip must not be negative")
	}
	if *movetime < time.Millisecond {
		return fmt.Errorf("movetime must be at least 1ms")
	}
	if *maxPlies < 1 {
		return fmt.Errorf("max-plies must be at least 1")
	}
	if _, err := os.Stat(*enginePath); err != nil {
		return fmt.Errorf("engine binary %q: %w", *enginePath, err)
	}
	// loadBook replays the lines, which needs legal move generation.
	if err := chess.BuildAllAttacksFromFS(data.FS); err != nil {
		return fmt.Errorf("initialize attack tables: %w", err)
	}

	book, err := loadBook(*bookPath)
	if err != nil {
		return err
	}
	if *checkBook {
		fmt.Fprintf(out, "book ok: %d lines\n", len(book))
		return nil
	}
	if *skip >= len(book) {
		return fmt.Errorf("skip %d >= book lines %d", *skip, len(book))
	}
	book = book[*skip:]
	if *pairs > len(book) {
		fmt.Fprintf(out, "note: book has %d lines after skip; playing %d pairs\n", len(book), len(book))
		*pairs = len(book)
	}

	var options []engineOption
	if *sfElo > 0 {
		options = append(options,
			engineOption{name: "UCI_LimitStrength", value: "true"},
			engineOption{name: "UCI_Elo", value: strconv.Itoa(*sfElo)},
		)
	}
	if *sfSkill >= 0 {
		options = append(options, engineOption{name: "Skill Level", value: strconv.Itoa(*sfSkill)})
	}

	sf, err := startEngine(*stockfishPath, options, errOut)
	if err != nil {
		return err
	}
	defer sf.Close()

	our, err := uci.StartInProcess(errOut)
	if err != nil {
		return fmt.Errorf("start gochess: %w", err)
	}
	defer our.Close()
	if err := our.SetOption("Move Overhead", "0"); err != nil {
		return fmt.Errorf("set Move Overhead: %w", err)
	}

	opponent := *stockfishPath
	switch {
	case *sfNodes > 0:
		opponent = fmt.Sprintf("Stockfish nodes=%d", *sfNodes)
	case *sfElo > 0:
		opponent = fmt.Sprintf("Stockfish UCI_Elo=%d", *sfElo)
	default:
		opponent = "Stockfish full strength"
	}
	sfTC := fmt.Sprintf("%s/move", *movetime)
	if *sfNodes > 0 {
		sfTC = fmt.Sprintf("nodes=%d", *sfNodes)
	} else if *sfMovetime > 0 {
		sfTC = fmt.Sprintf("%s/move", *sfMovetime)
	}

	fmt.Fprintf(out, "arena: gochess vs %s\n", opponent)
	fmt.Fprintf(out, "  book:     %s (%d lines, %d games)\n", *bookPath, *pairs, *pairs*2)
	fmt.Fprintf(out, "  gochess:  %s/move (Move Overhead 0)\n", *movetime)
	fmt.Fprintf(out, "  opponent: %s\n", sfTC)
	fmt.Fprintf(out, "  max plies: %d\n\n", *maxPlies)

	cfg := gameConfig{
		movetime:   *movetime,
		sfMovetime: *sfMovetime,
		sfNodes:    *sfNodes,
		maxPlies:   *maxPlies,
		verbose:    *verbose,
	}

	var (
		records    []gameRecord
		pairScores []float64
		wins       int
		draws      int
		losses     int
		endings    = make(map[string]int)
	)
	started := time.Now()

	for i := 0; i < *pairs; i++ {
		pair := 0.0
		for _, ourWhite := range []bool{true, false} {
			rec, err := playGame(cfg, book[i], ourWhite, our, sf, out)
			if err != nil {
				return fmt.Errorf("game %d: %w", len(records)+1, err)
			}
			records = append(records, rec)
			endings[rec.Reason]++

			gameScore := ourScore(rec.Result, rec.OurWhite)
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
			if rec.OurWhite {
				colour = "White"
			}
			fmt.Fprintf(out, "game %3d  %-24s  gochess %-5s  %-7s  %-20s  %3d plies\n",
				len(records), rec.Opening, colour, rec.Result, rec.Reason, rec.Plies)
		}
		pairScores = append(pairScores, pair)
	}

	elapsed := time.Since(started)
	elo, low, high := bootstrapElo(pairScores, *seed)
	fmt.Fprintf(out, "\nsummary: %d games, %d wins, %d draws, %d losses, score %.3f\n",
		len(records), wins, draws, losses, score(wins, draws, losses))
	fmt.Fprintf(out, "Elo difference vs %s: %s (95%% CI %s..%s)\n",
		opponent, formatElo(elo), formatElo(low), formatElo(high))
	if *sfElo > 0 {
		fmt.Fprintf(out, "implied gochess rating: %.0f (95%% CI %.0f..%.0f) on the Stockfish scale\n",
			float64(*sfElo)+elo, float64(*sfElo)+low, float64(*sfElo)+high)
	}
	fmt.Fprintf(out, "elapsed: %s\n", elapsed.Round(time.Second))

	if len(endings) > 0 {
		fmt.Fprintf(out, "endings:")
		for _, reason := range sortedKeys(endings) {
			fmt.Fprintf(out, " %s=%d", reason, endings[reason])
		}
		fmt.Fprintln(out)
	}

	if *jsonPath != "" {
		rep := report{
			Engine:   "gochess",
			Book:     *bookPath,
			Opponent: opponent,
			TC:       fmt.Sprintf("gochess %s/move, opponent %s", *movetime, sfTC),
			Games:    len(records),
			Wins:     wins,
			Draws:    draws,
			Losses:   losses,
			Score:    score(wins, draws, losses),
			Elo:      jsonFloat(elo),
			EloLow:   jsonFloat(low),
			EloHigh:  jsonFloat(high),
			Elapsed:  elapsed.Round(time.Second).String(),
			GamesLog: records,
		}
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal report: %w", err)
		}
		if dir := filepath.Dir(*jsonPath); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create report directory: %w", err)
			}
		}
		if err := os.WriteFile(*jsonPath, append(data, '\n'), 0o644); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
		fmt.Fprintf(out, "report: %s\n", *jsonPath)
	}

	return nil
}

func formatElo(v float64) string {
	switch {
	case math.IsInf(v, 1):
		return "+inf"
	case math.IsInf(v, -1):
		return "-inf"
	case math.Abs(v) < 0.5:
		return "+0"
	default:
		return fmt.Sprintf("%+.0f", v)
	}
}

func jsonFloat(v float64) *float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return nil
	}
	return &v
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
