package uci

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

// startSearch launches the one search goroutine. The command loop remains free
// to answer isready, stop, and quit while it runs.
func (e *Engine) startSearch(position *chess.Position, limits chess.SearchLimits, infinite, pondering bool) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	e.mu.Lock()
	if e.searching {
		e.mu.Unlock()
		cancel()
		return
	}
	e.searching = true
	e.pondering = pondering
	e.cancel = cancel
	e.done = done
	e.mu.Unlock()

	start := time.Now()
	go e.searchRoutine(ctx, position, limits, infinite, start, done)
}

func (e *Engine) searchRoutine(ctx context.Context, position *chess.Position, limits chess.SearchLimits, infinite bool, start time.Time, done chan struct{}) {
	reported := false
	finished := false
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(e.errOut, "search panic: %v\n", r)
			if !reported && !e.isQuitting() {
				e.out.line("info string internal search error")
				e.out.line("bestmove 0000")
			}
		}
		if !finished {
			e.finishSearch(done)
		}
	}()

	result := position.SearchWithLimits(ctx, limits, func(info chess.SearchInfo) {
		if !e.isQuitting() {
			e.writeInfo(info)
		}
	})
	elapsed := time.Since(start)

	// In infinite mode the search may reach a depth/node limit before the GUI
	// sends stop. Hold the result until stop or quit, as the specification
	// requires. Terminal positions are the exception: there is nothing to
	// search, so bestmove goes out immediately.
	if infinite && result.HasMove {
		<-ctx.Done()
	}

	// Mark the session idle before bestmove reaches the GUI. A GUI may react
	// immediately with position/go, and the command loop must not reject that
	// next command just because this goroutine is still writing its last line.
	e.finishSearch(done)
	finished = true

	if e.isQuitting() {
		return
	}
	e.writeFinal(result, elapsed)
	reported = true
}

func (e *Engine) finishSearch(done chan struct{}) {
	e.mu.Lock()
	e.searching = false
	e.pondering = false
	e.cancel = nil
	if e.done == done {
		e.done = nil
	}
	e.mu.Unlock()
	close(done)
}

func (e *Engine) isQuitting() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.quitting
}

// writeInfo formats one UCI info line. Scores are reported from the side to
// move's point of view, which is the engine's point of view at the root.
func (e *Engine) writeInfo(info chess.SearchInfo) {
	elapsedMS := info.Time.Milliseconds()
	if elapsedMS < 1 {
		elapsedMS = 1
	}
	var nps uint64
	if info.Nodes > 0 {
		nps = info.Nodes * 1000 / uint64(elapsedMS)
	}

	score := fmt.Sprintf("cp %d", info.Score)
	if moves, ok := chess.ScoreToMate(info.Score); ok {
		score = fmt.Sprintf("mate %d", moves)
	}

	line := fmt.Sprintf(
		"info depth %d seldepth %d score %s nodes %d nps %d time %d",
		info.Depth, info.SelDepth, score, info.Nodes, nps, elapsedMS,
	)
	if len(info.PV) > 0 {
		line += " pv " + formatPV(info.PV)
	}
	e.out.line("%s", line)
}

func (e *Engine) writeFinal(result chess.SearchResult, elapsed time.Duration) {
	if !result.HasMove {
		// A terminal checkmate has Score == -MateScore. An empty searchmoves
		// list also produces HasMove=false but Score 0, so do not infer mate
		// from "no move returned".
		if moves, ok := chess.ScoreToMate(result.Score); ok && result.Score < 0 && moves == 0 {
			e.out.line("info depth 0 score mate 0")
		} else {
			e.out.line("info depth 0 score cp 0")
		}
		e.out.line("bestmove 0000")
		return
	}

	e.writeInfo(chess.SearchInfo{
		Depth:    result.Depth,
		SelDepth: result.SelDepth,
		Score:    result.Score,
		Nodes:    result.Nodes,
		Time:     elapsed,
		PV:       result.PV,
	})
	e.out.line("bestmove %s", result.BestMove)
}

func formatPV(moves []chess.Move) string {
	var builder strings.Builder
	for i, move := range moves {
		if i > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(move.String())
	}
	return builder.String()
}
