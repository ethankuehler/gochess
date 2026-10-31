// Command match validates gochess games against Stockfish.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/data"
	"github.com/ethankuehler/gochess/uci"
)

type matchConfig struct {
	depth         int
	maxMoves      int
	randomPlies   int
	seed          uint64
	validateDepth int
	checkDraws    bool
	verbose       bool
}

type gameResult struct {
	result string // "1-0", "0-1", "1/2-1/2", or "*" when unfinished
	reason string
	plies  int
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "match:", err)
		os.Exit(1)
	}
}

func run(args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("match", flag.ContinueOnError)
	flags.SetOutput(errOut)
	stockfishPath := flags.String("stockfish", "stockfish", "Stockfish binary")
	games := flags.Int("games", 1, "number of games to play")
	depth := flags.Int("depth", 4, "search depth for both engines")
	maxMoves := flags.Int("max-moves", 300, "stop an unfinished game after this many plies")
	fen := flags.String("fen", "", "starting FEN (default: starting position)")
	moves := flags.String("moves", "", "space-separated UCI moves to apply after the starting FEN")
	randomPlies := flags.Int("random-plies", 0, "random legal opening plies before engine play")
	seed := flags.Uint64("seed", 1, "random seed for opening plies")
	validateDepth := flags.Int("validate-depth", 1, "perft depth compared with Stockfish at every position (0 disables)")
	checkDraws := flags.Bool("check-draws", true, "verify gochess recognizes each drawn position")
	verbose := flags.Bool("verbose", false, "print every move and score")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *games < 1 {
		return fmt.Errorf("games must be at least 1")
	}
	if *depth < 1 {
		return fmt.Errorf("depth must be at least 1")
	}
	if *maxMoves < 1 {
		return fmt.Errorf("max-moves must be at least 1")
	}
	if *randomPlies < 0 {
		return fmt.Errorf("random-plies must not be negative")
	}
	if *validateDepth < 0 {
		return fmt.Errorf("validate-depth must not be negative")
	}

	if err := chess.BuildAllAttacksFromFS(data.FS); err != nil {
		return fmt.Errorf("initialize attack tables: %w", err)
	}

	rootFEN := *fen
	if rootFEN == "" {
		rootFEN = chess.NewStartingPosition().FEN()
	}
	if _, err := chess.ParseFEN(rootFEN); err != nil {
		return fmt.Errorf("parse starting FEN: %w", err)
	}
	initialMoves := strings.Fields(*moves)

	stockfish, err := startStockfish(*stockfishPath, errOut)
	if err != nil {
		return err
	}
	defer stockfish.Close()

	ourClient, err := uci.StartInProcess(errOut)
	if err != nil {
		return fmt.Errorf("start gochess: %w", err)
	}
	defer ourClient.Close()

	config := matchConfig{
		depth:         *depth,
		maxMoves:      *maxMoves,
		randomPlies:   *randomPlies,
		seed:          *seed,
		validateDepth: *validateDepth,
		checkDraws:    *checkDraws,
		verbose:       *verbose,
	}

	fmt.Fprintf(out, "gochess vs Stockfish: %d game(s), depth %d, perft check depth %d\n",
		*games, config.depth, config.validateDepth)

	start := time.Now()
	var wins, losses, draws, unfinished int
	drawReasons := make(map[string]int)
	for gameIndex := 0; gameIndex < *games; gameIndex++ {
		ourColour := chess.White
		if gameIndex%2 == 1 {
			ourColour = chess.Black
		}
		result, err := playGame(config, gameIndex, rootFEN, initialMoves, ourColour, ourClient, stockfish, out)
		if err != nil {
			return fmt.Errorf("game %d: %w", gameIndex+1, err)
		}

		fmt.Fprintf(out, "game %d: gochess %s vs Stockfish %s — %s (%s) after %d plies\n",
			gameIndex+1, colourName(ourColour), colourName(ourColour.Opposite()),
			result.result, result.reason, result.plies)

		switch result.result {
		case "1-0":
			if ourColour == chess.White {
				wins++
			} else {
				losses++
			}
		case "0-1":
			if ourColour == chess.Black {
				wins++
			} else {
				losses++
			}
		case "1/2-1/2":
			draws++
			drawReasons[result.reason]++
		default:
			unfinished++
		}
	}

	fmt.Fprintf(out, "summary: gochess %d win(s), %d loss(es), %d draw(s), %d unfinished in %s\n",
		wins, losses, draws, unfinished, time.Since(start).Round(time.Millisecond))
	if len(drawReasons) > 0 {
		reasons := make([]string, 0, len(drawReasons))
		for reason := range drawReasons {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		parts := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			parts = append(parts, fmt.Sprintf("%s=%d", reason, drawReasons[reason]))
		}
		fmt.Fprintf(out, "draws by reason: %s\n", strings.Join(parts, ", "))
	}
	return nil
}

func playGame(config matchConfig, gameIndex int, rootFEN string, initialMoves []string, ourColour chess.Colour, ourClient *uci.Client, stockfish *stockfishClient, out io.Writer) (gameResult, error) {
	board, err := chess.ParseFEN(rootFEN)
	if err != nil {
		return gameResult{}, err
	}

	var (
		history    []string
		historyKey []chess.PositionKey
		rng        *rand.Rand
	)
	for _, move := range initialMoves {
		historyKey = append(historyKey, board.Key())
		next, _, err := board.ApplyUCIMove(move)
		if err != nil {
			return gameResult{}, fmt.Errorf("initial move %q: %w", move, err)
		}
		history = append(history, move)
		board = next
	}
	if config.randomPlies > 0 {
		rng = rand.New(rand.NewPCG(config.seed, uint64(gameIndex+1)))
	}

	if err := ourClient.NewGame(); err != nil {
		return gameResult{}, fmt.Errorf("gochess ucinewgame: %w", err)
	}
	if err := stockfish.newGame(); err != nil {
		return gameResult{}, err
	}

	for ply := len(history); ; ply++ {
		moves := board.LegalMoves()
		if len(moves) == 0 {
			if err := checkTerminalAwareness(board, historyKey, config.depth); err != nil {
				return gameResult{}, err
			}
			if board.InCheck() {
				result := "1-0"
				if board.SideToMove() == chess.White {
					result = "0-1"
				}
				return gameResult{result: result, reason: "checkmate", plies: ply}, nil
			}
			return gameResult{result: "1/2-1/2", reason: "stalemate", plies: ply}, nil
		}

		if reason := drawReason(board, historyKey); reason != "" {
			if config.checkDraws {
				if err := checkDrawAwareness(board, historyKey, config.depth, reason); err != nil {
					return gameResult{}, err
				}
			}
			return gameResult{result: "1/2-1/2", reason: reason, plies: ply}, nil
		}
		if ply >= config.maxMoves {
			return gameResult{result: "*", reason: "max-moves", plies: ply}, nil
		}

		if config.validateDepth > 0 {
			if err := validatePosition(board, rootFEN, history, stockfish, config.validateDepth); err != nil {
				return gameResult{}, err
			}
		}

		var (
			move       string
			engineName string
			score      int
			hasMate    bool
		)
		switch {
		case ply < config.randomPlies:
			engineName = "random"
			move = moves[rng.IntN(len(moves))].String()
		case board.SideToMove() == ourColour:
			engineName = "gochess"
			if err := ourClient.SetPosition(rootFEN, history...); err != nil {
				return gameResult{}, fmt.Errorf("gochess position: %w", err)
			}
			result, err := ourClient.GoDepth(config.depth)
			if err != nil {
				return gameResult{}, fmt.Errorf("gochess search: %w", err)
			}
			move = result.BestMove
			if result.HasMate {
				score, hasMate = result.MateIn, true
			} else {
				score = result.Score
			}
		default:
			engineName = "Stockfish"
			if config.validateDepth == 0 {
				if err := stockfish.setPosition(rootFEN, history); err != nil {
					return gameResult{}, err
				}
			}
			move, score, hasMate, err = stockfish.goDepth(config.depth)
			if err != nil {
				return gameResult{}, err
			}
		}
		if move == "" {
			return gameResult{}, fmt.Errorf("%s returned no move at %s", engineName, board.FEN())
		}

		parsed, err := chess.ParseUCIMove(move)
		if err != nil {
			return gameResult{}, fmt.Errorf("%s returned malformed move %q: %w", engineName, move, err)
		}
		if _, ok := board.FindLegalMove(*parsed); !ok {
			return gameResult{}, fmt.Errorf("%s returned illegal move %q in %s", engineName, move, board.FEN())
		}

		if config.verbose {
			if hasMate {
				fmt.Fprintf(out, "game %d ply %d %s %s (mate %d)\n", gameIndex+1, ply+1, engineName, move, score)
			} else {
				fmt.Fprintf(out, "game %d ply %d %s %s (%+d cp)\n", gameIndex+1, ply+1, engineName, move, score)
			}
		}

		next, _, err := board.ApplyUCIMove(move)
		if err != nil {
			return gameResult{}, fmt.Errorf("%s move %q: %w", engineName, move, err)
		}
		historyKey = append(historyKey, board.Key())
		history = append(history, move)
		board = next
	}
}

func drawReason(board *chess.Position, history []chess.PositionKey) string {
	switch {
	case board.IsFiftyMoveDraw():
		return "fifty-move rule"
	case board.IsThreefoldRepetition(history):
		return "threefold repetition"
	case board.IsInsufficientMaterial():
		return "insufficient material"
	default:
		return ""
	}
}

func checkTerminalAwareness(board *chess.Position, history []chess.PositionKey, depth int) error {
	result := board.SearchWithLimits(context.Background(), chess.SearchLimits{
		MaxDepth:    depth,
		GameHistory: history,
	}, nil)
	if result.HasMove {
		return fmt.Errorf("gochess returned a move in a terminal position (%s)", board.FEN())
	}
	if board.InCheck() {
		if result.Score != -chess.MateScore {
			return fmt.Errorf("gochess scored checkmate as %d, want %d (%s)", result.Score, -chess.MateScore, board.FEN())
		}
		return nil
	}
	if result.Score != 0 {
		return fmt.Errorf("gochess scored stalemate as %d, want 0 (%s)", result.Score, board.FEN())
	}
	return nil
}

func checkDrawAwareness(board *chess.Position, history []chess.PositionKey, depth int, reason string) error {
	result := board.SearchWithLimits(context.Background(), chess.SearchLimits{
		MaxDepth:    depth,
		GameHistory: history,
	}, nil)
	if !result.HasMove {
		return fmt.Errorf("gochess returned no move in a %s position (%s)", reason, board.FEN())
	}
	if !result.Draw {
		return fmt.Errorf("gochess did not recognize %s (score %d, %s)", reason, result.Score, board.FEN())
	}
	if result.Score != 0 {
		return fmt.Errorf("gochess reported score %d for a %s draw (%s)", result.Score, reason, board.FEN())
	}
	return nil
}

func validatePosition(board *chess.Position, rootFEN string, history []string, stockfish *stockfishClient, depth int) error {
	if err := stockfish.setPosition(rootFEN, history); err != nil {
		return err
	}
	theirs, theirTotal, err := stockfish.perft(depth)
	if err != nil {
		return err
	}

	ours := make(map[string]uint64)
	var ourTotal uint64
	for _, result := range board.PerftDivide(depth) {
		ours[result.Move.String()] = result.Nodes
		ourTotal += result.Nodes
	}

	if err := comparePerft(ours, ourTotal, theirs, theirTotal, board.FEN()); err != nil {
		if len(history) > 0 {
			return fmt.Errorf("%w (moves: %s)", err, strings.Join(history, " "))
		}
		return err
	}
	return nil
}

func comparePerft(ours map[string]uint64, ourTotal uint64, theirs map[string]uint64, theirTotal uint64, fen string) error {
	var problems []string
	for move, nodes := range ours {
		want, ok := theirs[move]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s missing from Stockfish", move))
		} else if want != nodes {
			problems = append(problems, fmt.Sprintf("%s ours=%d Stockfish=%d", move, nodes, want))
		}
	}
	for move, nodes := range theirs {
		if _, ok := ours[move]; !ok {
			problems = append(problems, fmt.Sprintf("%s extra in Stockfish (%d nodes)", move, nodes))
		}
	}
	if ourTotal != theirTotal {
		problems = append(problems, fmt.Sprintf("total ours=%d Stockfish=%d", ourTotal, theirTotal))
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	const limit = 8
	if len(problems) > limit {
		problems = append(problems[:limit], fmt.Sprintf("and %d more", len(problems)-limit))
	}
	return fmt.Errorf("perft mismatch at %s: %s", fen, strings.Join(problems, "; "))
}

func colourName(colour chess.Colour) string {
	if colour == chess.White {
		return "White"
	}
	return "Black"
}
