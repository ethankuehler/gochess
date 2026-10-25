package main

import (
	"fmt"
	"io"
	"time"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/uci"
)

type gameConfig struct {
	movetime   time.Duration
	sfMovetime time.Duration
	sfNodes    uint64
	maxPlies   int
	verbose    bool
}

type gameRecord struct {
	Opening  string   `json:"opening"`
	OurWhite bool     `json:"our_white"`
	Result   string   `json:"result"`
	Reason   string   `json:"reason"`
	Plies    int      `json:"plies"`
	Moves    []string `json:"moves"`
}

// playGame plays one game from a book line. The colours are assigned by the
// caller; everything after the book is decided by the two engines.
func playGame(cfg gameConfig, line opening, ourWhite bool, our *uci.Client, sf *sfClient, out io.Writer) (gameRecord, error) {
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
			return gameRecord{}, fmt.Errorf("book %q move %s: %w", line.Name, move, err)
		}
		history = append(history, move)
		board = next
	}

	if err := our.NewGame(); err != nil {
		return gameRecord{}, fmt.Errorf("gochess ucinewgame: %w", err)
	}
	if err := sf.newGame(); err != nil {
		return gameRecord{}, err
	}

	rec := gameRecord{Opening: line.Name, OurWhite: ourWhite}
	for ply := len(history); ; ply++ {
		if len(board.LegalMoves()) == 0 {
			if board.InCheck() {
				rec.Result = "1-0"
				if board.SideToMove() == chess.White {
					rec.Result = "0-1"
				}
				rec.Reason = "checkmate"
			} else {
				rec.Result = "1/2-1/2"
				rec.Reason = "stalemate"
			}
			rec.Plies = ply
			rec.Moves = append([]string(nil), history...)
			return rec, nil
		}
		if reason := drawReason(board, keys); reason != "" {
			rec.Result = "1/2-1/2"
			rec.Reason = reason
			rec.Plies = ply
			rec.Moves = append([]string(nil), history...)
			return rec, nil
		}
		if ply >= cfg.maxPlies {
			rec.Result = "1/2-1/2"
			rec.Reason = "max-plies"
			rec.Plies = ply
			rec.Moves = append([]string(nil), history...)
			return rec, nil
		}

		ourTurn := (board.SideToMove() == chess.White) == ourWhite
		var (
			move    string
			score   int
			elapsed time.Duration
			engine  string
		)
		if ourTurn {
			engine = "gochess"
			if err := our.SetPosition(startFEN, history...); err != nil {
				return rec, fmt.Errorf("gochess position: %w", err)
			}
			start := time.Now()
			res, err := our.GoMovetime(cfg.movetime)
			elapsed = time.Since(start)
			if err != nil {
				return rec, fmt.Errorf("gochess search: %w", err)
			}
			move, score = res.BestMove, res.Score
		} else {
			engine = "Stockfish"
			if err := sf.setPosition(startFEN, history); err != nil {
				return rec, err
			}
			start := time.Now()
			var err error
			if cfg.sfNodes > 0 {
				move, score, err = sf.goNodes(cfg.sfNodes)
			} else {
				budget := cfg.sfMovetime
				if budget <= 0 {
					budget = cfg.movetime
				}
				move, score, err = sf.goMovetime(budget)
			}
			elapsed = time.Since(start)
			if err != nil {
				return rec, err
			}
		}

		if move == "" {
			return rec, fmt.Errorf("%s returned no move in %s", engine, board.FEN())
		}
		parsed, err := chess.ParseUCIMove(move)
		if err != nil {
			return rec, fmt.Errorf("%s returned malformed move %q: %w", engine, move, err)
		}
		if _, ok := board.FindLegalMove(*parsed); !ok {
			return rec, fmt.Errorf("%s returned illegal move %q in %s", engine, move, board.FEN())
		}
		if cfg.verbose {
			fmt.Fprintf(out, "  %-9s %-6s %+5d cp  %s\n",
				engine, move, score, elapsed.Round(time.Millisecond))
		}

		keys = append(keys, board.Key())
		next, _, err := board.ApplyUCIMove(move)
		if err != nil {
			return rec, fmt.Errorf("%s move %q: %w", engine, move, err)
		}
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

// ourScore converts a PGN result into gochess's score for that game.
func ourScore(result string, ourWhite bool) float64 {
	switch result {
	case "1-0":
		if ourWhite {
			return 1
		}
		return 0
	case "0-1":
		if ourWhite {
			return 0
		}
		return 1
	default:
		return 0.5
	}
}
