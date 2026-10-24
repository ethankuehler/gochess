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

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/data"
)

// Engine is one UCI session. The command loop owns position and option
// mutation; the search goroutine owns the current search and is the only
// writer of bestmove.
type Engine struct {
	in     *bufio.Reader
	out    *lineWriter
	errOut io.Writer
	fsys   fs.FS

	initOnce sync.Once
	initErr  error

	mu          sync.Mutex
	position    *chess.Position
	posValid    bool
	hasPosition bool
	options     options
	debug       bool
	searching   bool
	pondering   bool
	cancel      context.CancelFunc
	done        chan struct{}
	quitting    bool
}

// Run starts a UCI session on the given streams and returns when the GUI sends
// quit or closes stdin. It uses the embedded data files, so the working
// directory does not matter.
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

// lockedWriter serializes diagnostics written by the command loop and the
// search goroutine.
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

// handle processes one line. It returns true when the command loop should
// exit (quit).
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
	if message := e.options.set(name, value, hasValue); message != "" {
		e.out.line("info string %s", message)
	}
}

func (e *Engine) handleNewGame() {
	if e.isSearching() {
		fmt.Fprintln(e.errOut, "ucinewgame ignored while searching")
		return
	}
	e.mu.Lock()
	e.position = chess.NewStartingPosition()
	e.posValid = true
	e.hasPosition = true
	e.mu.Unlock()
}

func (e *Engine) handleStop() {
	e.mu.Lock()
	if e.searching && e.cancel != nil {
		e.cancel()
	}
	e.mu.Unlock()
}

func (e *Engine) handlePonderHit() {
	// Pondering is not advertised and has no clock budget. If a GUI sends
	// go ponder anyway, treat ponderhit as stop so the engine returns a move
	// instead of waiting for a stop that may never arrive.
	e.mu.Lock()
	searching := e.searching
	pondering := e.pondering
	cancel := e.cancel
	e.mu.Unlock()
	if !searching || !pondering || cancel == nil {
		fmt.Fprintln(e.errOut, "ponderhit ignored: not pondering")
		return
	}
	fmt.Fprintln(e.errOut, "ponderhit treated as stop: pondering is not supported")
	cancel()
}

// quit stops a running search and waits for it to finish. It is safe to call
// more than once.
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
	e.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (e *Engine) isSearching() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.searching
}

// positionForGo returns the position to search. If no position has ever been
// set it falls back to the starting position, which is friendlier than
// replying with no move. A failed position command leaves position nil and
// posValid false, in which case go must not reuse a stale board.
func (e *Engine) positionForGo() (*chess.Position, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.position == nil && !e.hasPosition {
		e.position = chess.NewStartingPosition()
		e.posValid = true
		e.hasPosition = true
	}
	if e.position == nil || !e.posValid {
		return nil, false
	}
	return e.position, true
}
