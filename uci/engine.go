package uci

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/data"
)

// Only the search goroutine writes bestmove; the command loop owns the position.
type Engine struct {
	in     *bufio.Reader
	out    *lineWriter
	errOut io.Writer
	fsys   fs.FS

	initOnce sync.Once
	initErr  error

	mu          sync.Mutex
	position    *chess.Position
	history     []chess.PositionKey
	posValid    bool
	hasPosition bool
	options     options
	tt          *chess.TranspositionTable
	debug       bool
	searching   bool
	ponder      *ponderState
	cancel      context.CancelFunc
	done        chan struct{}
	quitting    bool

	ponderHits   int
	ponderMisses int
}

type ponderState struct {
	clock   *chess.SearchClock
	params  goParams
	side    chess.Colour
	started time.Time
	release chan struct{}

	released bool
	depth    atomic.Int64
}

func Run(in io.Reader, out, errOut io.Writer) int {
	engine := newEngine(in, out, errOut, data.FS)
	engine.loop()
	return 0
}

func newEngine(in io.Reader, out, errOut io.Writer, fsys fs.FS) *Engine {
	if errOut == nil {
		errOut = io.Discard
	}
	return &Engine{
		in:      bufio.NewReader(in),
		out:     newLineWriter(out),
		errOut:  &lockedWriter{w: errOut},
		fsys:    fsys,
		options: defaultOptions(),
	}
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

func (e *Engine) loop() {
	for {
		line, err := e.in.ReadString('\n')
		if len(line) > 0 {
			if e.handle(line) {
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Fprintf(e.errOut, "read error: %v\n", err)
			}
			e.quit()
			return
		}
	}
}

func (e *Engine) handle(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}

	command := strings.ToLower(fields[0])
	if e.debug {
		e.out.line("info string %s", line)
	}

	switch command {
	case "uci":
		e.handshake()
	case "isready":
		e.handleIsReady()
	case "debug":
		e.handleDebug(fields[1:])
	case "setoption":
		e.handleSetOption(fields[1:])
	case "ucinewgame":
		e.handleNewGame()
	case "position":
		e.handlePosition(fields[1:])
	case "go":
		e.handleGo(fields[1:])
	case "stop":
		e.handleStop()
	case "ponderhit":
		e.handlePonderHit()
	case "bench":
		e.handleBench(fields[1:])
	case "quit":
		e.quit()
		return true
	default:
		fmt.Fprintf(e.errOut, "unknown command: %s\n", line)
	}
	return false
}

func (e *Engine) handshake() {
	e.out.line("id name %s", VersionString())
	e.out.line("id author %s", Author)
	for _, line := range e.options.lines() {
		e.out.line("%s", line)
	}
	e.out.line("uciok")
}

func (e *Engine) ensureInitialized() error {
	e.initOnce.Do(func() {
		if e.fsys == nil {
			e.initErr = errors.New("no data filesystem")
			return
		}
		e.initErr = chess.BuildAllAttacksFromFS(e.fsys)
	})
	return e.initErr
}

func (e *Engine) handleIsReady() {
	if err := e.ensureInitialized(); err != nil {
		e.out.line("info string initialization failed: %v", err)
	}
	e.out.line("readyok")
}

func (e *Engine) handleDebug(args []string) {
	if len(args) == 0 {
		return
	}
	switch strings.ToLower(args[0]) {
	case "on":
		e.debug = true
	case "off":
		e.debug = false
	default:
		fmt.Fprintf(e.errOut, "debug: expected on or off, got %q\n", args[0])
	}
}

func (e *Engine) handleSetOption(args []string) {
	if e.isSearching() {
		fmt.Fprintln(e.errOut, "setoption ignored while searching")
		return
	}
	name, value, hasValue, ok := parseSetOption(args)
	if !ok {
		fmt.Fprintln(e.errOut, "setoption: missing name")
		return
	}
	oldHash := e.options.hashMB
	if message := e.options.set(name, value, hasValue); message != "" {
		e.out.line("info string %s", message)
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "hash":
		if e.options.hashMB != oldHash || e.tt == nil {
			e.tt = chess.NewTranspositionTable(e.options.hashMB)
		}
	case "clear hash":
		if e.tt != nil {
			e.tt.Clear()
		}
	}
}

// Only the idle command loop may allocate or resize the table.
func (e *Engine) transpositionTable() *chess.TranspositionTable {
	if e.tt == nil {
		e.tt = chess.NewTranspositionTable(e.options.hashMB)
	}
	return e.tt
}

func (e *Engine) handleNewGame() {
	if e.isSearching() {
		fmt.Fprintln(e.errOut, "ucinewgame ignored while searching")
		return
	}
	e.mu.Lock()
	e.position = chess.NewStartingPosition()
	e.history = nil
	e.posValid = true
	e.hasPosition = true
	e.mu.Unlock()
	if e.tt != nil {
		e.tt.Clear()
	}
}

func (e *Engine) handleStop() {
	e.mu.Lock()
	if e.searching && e.cancel != nil {
		e.cancel()
	}
	ps := e.ponder
	if ps != nil {
		e.releasePonderLocked(ps)
		e.ponder = nil
		e.ponderMisses++
	}
	e.mu.Unlock()

	if ps != nil {
		fmt.Fprintf(e.errOut, "ponder miss: %dms depth %d\n",
			time.Since(ps.started).Milliseconds(), ps.depth.Load())
	}
}

func (e *Engine) handlePonderHit() {
	e.mu.Lock()
	ps := e.ponder
	if !e.searching || ps == nil {
		e.mu.Unlock()
		fmt.Fprintln(e.errOut, "ponderhit ignored: not pondering")
		return
	}

	// The move clock starts at ponderhit.
	budget := allocateBudget(ps.params, ps.side, e.options.moveOverhead)
	if budget.ok {
		now := time.Now()
		ps.clock.SetDeadlines(now.Add(budget.soft), now.Add(budget.hard))
	}
	e.releasePonderLocked(ps)
	e.ponder = nil
	e.ponderHits++
	elapsed := time.Since(ps.started)
	depth := ps.depth.Load()
	e.mu.Unlock()

	fmt.Fprintf(e.errOut, "ponder hit: %dms depth %d\n", elapsed.Milliseconds(), depth)
}

// releasePonderLocked requires e.mu.
func (e *Engine) releasePonderLocked(ps *ponderState) {
	if ps.released {
		return
	}
	ps.released = true
	close(ps.release)
}

func (e *Engine) quit() {
	e.mu.Lock()
	if e.quitting {
		done := e.done
		e.mu.Unlock()
		if done != nil {
			<-done
		}
		return
	}
	e.quitting = true
	cancel := e.cancel
	done := e.done
	if ps := e.ponder; ps != nil {
		e.releasePonderLocked(ps)
		e.ponder = nil
	}
	hits, misses := e.ponderHits, e.ponderMisses
	e.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	if hits+misses > 0 {
		fmt.Fprintf(e.errOut, "ponder summary: hits %d misses %d\n", hits, misses)
	}
}

func (e *Engine) isSearching() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.searching
}

// A failed position command must not leave a stale board available to go.
func (e *Engine) positionForGo() (*chess.Position, []chess.PositionKey, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.position == nil && !e.hasPosition {
		e.position = chess.NewStartingPosition()
		e.history = nil
		e.posValid = true
		e.hasPosition = true
	}
	if e.position == nil || !e.posValid {
		return nil, nil, false
	}
	history := append([]chess.PositionKey(nil), e.history...)
	return e.position, history, true
}
