// Command search runs a fixed-depth search on one position.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "search:", err)
		os.Exit(1)
	}
}

func run(args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("search", flag.ContinueOnError)
	flags.SetOutput(errOut)
	fen := flags.String("fen", "", "position in FEN (default: starting position)")
	moves := flags.String("moves", "", "space-separated UCI moves to apply after the FEN")
	depth := flags.Int("depth", 4, "search depth in plies")
	evalOnly := flags.Bool("eval", false, "print the static evaluation and exit")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	fenValue := *fen
	if fenValue == "" {
		fenValue = chess.NewStartingPosition().FEN()
	}
	position, err := chess.ParseFEN(fenValue)
	if err != nil {
		return fmt.Errorf("parse FEN: %w", err)
	}

	// Evaluation and move application both need the attack tables.
	chess.BuildAllAttacks()

	history := make([]chess.PositionKey, 0, len(strings.Fields(*moves)))
	if *moves != "" {
		for _, uci := range strings.Fields(*moves) {
			history = append(history, position.Key())
			next, _, err := position.ApplyUCIMove(uci)
			if err != nil {
				return fmt.Errorf("apply move %q: %w", uci, err)
			}
			position = next
		}
	}

	if *evalOnly {
		score := position.Evaluate()
		fmt.Fprintf(out, "evaluation %+.2f (white)\n", float64(score)/100)
		if position.SideToMove() == chess.Black {
			score = -score
		}
		fmt.Fprintf(out, "evaluation %+.2f (side to move)\n", float64(score)/100)
		return nil
	}

	if *depth < 1 {
		return fmt.Errorf("depth must be at least 1")
	}

	start := time.Now()
	result := position.SearchWithLimits(context.Background(), chess.SearchLimits{
		MaxDepth:    *depth,
		GameHistory: history,
		TT:          chess.NewTranspositionTable(chess.DefaultHashMB),
	}, nil)
	elapsed := time.Since(start)

	fmt.Fprintf(out, "fen        %s\n", position.FEN())
	fmt.Fprintf(out, "depth      %d\n", result.Depth)
	if result.Draw {
		fmt.Fprintln(out, "draw       fifty-move, threefold repetition, or insufficient material")
	}
	if !result.HasMove {
		fmt.Fprintln(out, "bestmove   (none)")
		if position.InCheck() {
			fmt.Fprintln(out, "score      checkmate")
		} else {
			fmt.Fprintln(out, "score      stalemate")
		}
	} else {
		fmt.Fprintf(out, "bestmove   %s\n", result.BestMove)
		if moves, ok := chess.ScoreToMate(result.Score); ok {
			if moves >= 0 {
				fmt.Fprintf(out, "score      mate in %d (side to move)\n", moves)
			} else {
				fmt.Fprintf(out, "score      mated in %d (side to move)\n", -moves)
			}
		} else {
			fmt.Fprintf(out, "score      %+.2f (side to move)\n", float64(result.Score)/100)
		}
	}
	fmt.Fprintf(out, "nodes      %d\n", result.Nodes)
	fmt.Fprintf(out, "hash       %d probes, %d hits, %d cutoffs, %d stores\n",
		result.TT.Probes, result.TT.Hits, result.TT.Cutoffs, result.TT.Stores)
	fmt.Fprintf(out, "time       %s\n", elapsed.Round(time.Millisecond))
	if elapsed > 0 {
		fmt.Fprintf(out, "nps        %d\n", int64(float64(result.Nodes)/elapsed.Seconds()))
	}
	return nil
}
