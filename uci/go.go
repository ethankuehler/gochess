package uci

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

// goParams is the parsed form of the go command. Zero values mean "not
// supplied" for every field.
type goParams struct {
	depth       int
	nodes       uint64
	movetime    time.Duration
	movetimeSet bool
	wtime       time.Duration
	btime       time.Duration
	winc        time.Duration
	binc        time.Duration
	movestogo   int
	hasClock    bool
	infinite    bool
	ponder      bool
	searchMoves []string
	searchSet   bool
}

// searchBudget is the soft and hard time allowance for one move.
type searchBudget struct {
	soft time.Duration
	hard time.Duration
	ok   bool
}

// handleGo implements the go command. It snapshots the current position,
// computes a time budget, and starts one search goroutine.
func (e *Engine) handleGo(args []string) {
	if e.isSearching() {
		fmt.Fprintln(e.errOut, "go ignored: a search is already running")
		return
	}
	if err := e.ensureInitialized(); err != nil {
		e.out.line("info string initialization failed: %v", err)
		e.out.line("bestmove 0000")
		return
	}

	position, ok := e.positionForGo()
	if !ok {
		e.out.line("info string no valid position")
		e.out.line("bestmove 0000")
		return
	}

	params := parseGoParams(args)
	if params.ponder {
		// Pondering is not advertised. If a GUI sends go ponder anyway, search
		// without a clock and wait for stop/quit.
		params.infinite = true
	}
	rootMoves := e.resolveRootMoves(position, params)
	budget := allocateBudget(params, position.SideToMove(), e.options.moveOverhead)

	limits := chess.SearchLimits{
		MaxDepth:  params.depth,
		MaxNodes:  params.nodes,
		RootMoves: rootMoves,
	}
	if budget.ok {
		now := time.Now()
		limits.SoftDeadline = now.Add(budget.soft)
		limits.Deadline = now.Add(budget.hard)
	}

	e.startSearch(position, limits, params.infinite, params.ponder)
}

func parseGoParams(args []string) goParams {
	var params goParams
	for i := 0; i < len(args); i++ {
		switch strings.ToLower(args[i]) {
		case "searchmoves":
			params.searchSet = true
			params.searchMoves = append(params.searchMoves, args[i+1:]...)
			return params
		case "ponder":
			params.ponder = true
		case "infinite":
			params.infinite = true
		case "wtime":
			if i+1 < len(args) {
				params.wtime = parseMilliseconds(args[i+1])
				params.hasClock = true
				i++
			}
		case "btime":
			if i+1 < len(args) {
				params.btime = parseMilliseconds(args[i+1])
				params.hasClock = true
				i++
			}
		case "winc":
			if i+1 < len(args) {
				params.winc = parseMilliseconds(args[i+1])
				i++
			}
		case "binc":
			if i+1 < len(args) {
				params.binc = parseMilliseconds(args[i+1])
				i++
			}
		case "movestogo":
			if i+1 < len(args) {
				params.movestogo = parsePositiveInt(args[i+1])
				i++
			}
		case "depth":
			if i+1 < len(args) {
				params.depth = parsePositiveInt(args[i+1])
				i++
			}
		case "nodes":
			if i+1 < len(args) {
				params.nodes = parseNodes(args[i+1])
				i++
			}
		case "movetime":
			if i+1 < len(args) {
				params.movetime = parseMilliseconds(args[i+1])
				params.movetimeSet = true
				i++
			}
		case "mate":
			// Optional. Ignore the value and search normally.
			if i+1 < len(args) {
				i++
			}
		}
	}
	return params
}

func parseMilliseconds(token string) time.Duration {
	const maxMilliseconds = int64(24 * 60 * 60 * 1000)
	value, err := strconv.ParseInt(token, 10, 64)
	if err != nil || value < 0 {
		return 0
	}
	if value > maxMilliseconds {
		value = maxMilliseconds
	}
	return time.Duration(value) * time.Millisecond
}

func parsePositiveInt(token string) int {
	value, err := strconv.Atoi(token)
	if err != nil || value < 1 {
		return 0
	}
	return value
}

func parseNodes(token string) uint64 {
	value, err := strconv.ParseUint(token, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

// resolveRootMoves filters the searchmoves list to legal generated moves.
// A non-nil empty result is meaningful: it means the GUI supplied a
// searchmoves list with no legal entries, so there is no move to search.
func (e *Engine) resolveRootMoves(position *chess.Position, params goParams) []chess.Move {
	if !params.searchSet {
		return nil
	}
	rootMoves := make([]chess.Move, 0, len(params.searchMoves))
	seen := make(map[chess.Move]bool, len(params.searchMoves))
	for _, token := range params.searchMoves {
		parsed, err := chess.ParseUCIMove(token)
		if err != nil {
			continue
		}
		legal, ok := position.FindLegalMove(*parsed)
		if !ok || seen[legal] {
			continue
		}
		seen[legal] = true
		rootMoves = append(rootMoves, legal)
	}
	return rootMoves
}

// allocateBudget turns UCI clock fields into a soft and hard time allowance.
// It is deliberately conservative: losing on time is worse than playing a
// slightly weaker move.
func allocateBudget(params goParams, side chess.Colour, overhead time.Duration) searchBudget {
	if params.infinite || params.ponder {
		return searchBudget{}
	}
	if params.movetimeSet {
		budget := params.movetime - overhead
		if budget < time.Millisecond {
			budget = time.Millisecond
		}
		return searchBudget{soft: budget, hard: budget, ok: true}
	}
	if !params.hasClock {
		return searchBudget{}
	}

	remaining := params.wtime
	increment := params.winc
	if side == chess.Black {
		remaining = params.btime
		increment = params.binc
	}
	if remaining <= 0 {
		return searchBudget{soft: time.Millisecond, hard: time.Millisecond, ok: true}
	}

	movesToGo := params.movestogo
	if movesToGo < 1 {
		movesToGo = 30
	}
	if movesToGo > 50 {
		movesToGo = 50
	}

	soft := remaining/time.Duration(movesToGo) + 3*increment/4
	cap := remaining / 4
	if params.movestogo > 0 {
		cap = remaining / 2
	}
	if cap < time.Millisecond {
		cap = time.Millisecond
	}
	if soft > cap {
		soft = cap
	}
	if soft < time.Millisecond {
		soft = time.Millisecond
	}

	hard := soft * 2
	limit := remaining - overhead
	if limit < time.Millisecond {
		limit = time.Millisecond
	}
	if hard > limit {
		hard = limit
	}
	if hard < soft {
		hard = soft
	}

	// With almost no time left, do not start a meaningful search.
	if remaining <= 2*overhead+10*time.Millisecond {
		soft = time.Millisecond
		hard = time.Millisecond
	}

	return searchBudget{soft: soft, hard: hard, ok: true}
}
