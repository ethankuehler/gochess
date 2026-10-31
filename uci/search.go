package uci

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

func (e *Engine) startSearch(position *chess.Position, limits chess.SearchLimits, infinite bool, pondering *ponderState) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	e.mu.Lock()
	if e.searching {
		e.mu.Unlock()
		cancel()
		return
	}
	e.searching = true
	e.ponder = pondering
	e.cancel = cancel
	e.done = done
	e.mu.Unlock()

	start := time.Now()
	go e.searchRoutine(ctx, position, limits, infinite, pondering, start, done)
}

func (e *Engine) searchRoutine(ctx context.Context, position *chess.Position, limits chess.SearchLimits, infinite bool, pondering *ponderState, start time.Time, done chan struct{}) {
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
		if pondering != nil {
			pondering.depth.Store(int64(info.Depth))
		}
		if !e.isQuitting() {
			e.writeInfo(info)
		}
	})
	elapsed := time.Since(start)

	// Ponder results wait for ponderhit or stop.
	if pondering != nil {
		select {
		case <-pondering.release:
		case <-ctx.Done():
		}
	}

	// Infinite results wait for stop unless the position is terminal.
	if infinite && result.HasMove {
		<-ctx.Done()
	}

	// Go idle before bestmove so the GUI can immediately send position/go.
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
	e.ponder = nil
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
		// Empty searchmoves also has no move, but is not checkmate.
		if moves, ok := chess.ScoreToMate(result.Score); ok && result.Score < 0 && moves == 0 {
			e.out.line("info depth 0 score mate 0")
		} else {
			e.out.line("info depth 0 score cp 0")
		}
		e.writeAnalysis(result.Metrics)
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
	e.writeAnalysis(result.Metrics)
	if result.PonderMove != (chess.Move{}) {
		e.out.line("bestmove %s ponder %s", result.BestMove, result.PonderMove)
		return
	}
	e.out.line("bestmove %s", result.BestMove)
}

func (e *Engine) writeAnalysis(metrics *chess.SearchMetrics) {
	if metrics == nil {
		return
	}
	encoded, err := json.Marshal(metrics)
	if err != nil {
		fmt.Fprintf(e.errOut, "analysis metrics: %v\n", err)
		return
	}
	e.out.line("info string analysis %s", encoded)
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
