package uci

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EngineResult is the information a client collects from one `go` command.
// Score is from the side to move's point of view, matching UCI. When HasMate
// is true, MateIn is in moves (negative means the engine is being mated) and
// Score is not meaningful.
type EngineResult struct {
	BestMove string
	Score    int
	MateIn   int
	HasMate  bool
	Depth    int
	Nodes    uint64
	TimeMS   int64
	PV       []string
}

// Client is a small UCI client for a single engine session. It is not safe for
// concurrent use; callers should serialize commands. StartInProcess runs the
// engine in this process over io.Pipe, which is convenient for the web demo
// and for tests because no external binary or working directory is needed.
type Client struct {
	in         io.WriteCloser
	out        *bufio.Reader
	outCloser  io.Closer
	done       chan struct{}
	lines      chan string
	stop       chan struct{}
	readerDone chan struct{}
	readErr    error

	closeOnce sync.Once
	closeErr  error
}

// StartInProcess starts a gochess UCI engine in the current process and
// completes the uci/isready handshake. Diagnostics from the engine are written
// to errOut; pass io.Discard to silence them.
func StartInProcess(errOut io.Writer) (*Client, error) {
	if errOut == nil {
		errOut = io.Discard
	}

	commandReader, commandWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()

	client := &Client{
		in:         commandWriter,
		out:        bufio.NewReader(outputReader),
		outCloser:  outputReader,
		done:       make(chan struct{}),
		lines:      make(chan string, 64),
		stop:       make(chan struct{}),
		readerDone: make(chan struct{}),
	}

	go func() {
		_ = Run(commandReader, outputWriter, errOut)
		_ = outputWriter.Close()
		_ = commandReader.Close()
		close(client.done)
	}()
	go client.readLoop()

	if err := client.handshake(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *Client) handshake() error {
	if err := c.send("uci"); err != nil {
		return fmt.Errorf("send uci: %w", err)
	}
	for {
		line, err := c.readLine()
		if err != nil {
			return fmt.Errorf("read uci handshake: %w", err)
		}
		if line == "uciok" {
			break
		}
		if isInitializationError(line) {
			return errors.New(strings.TrimSpace(strings.TrimPrefix(line, "info string")))
		}
	}

	if err := c.send("isready"); err != nil {
		return fmt.Errorf("send isready: %w", err)
	}
	for {
		line, err := c.readLine()
		if err != nil {
			return fmt.Errorf("read readyok: %w", err)
		}
		if line == "readyok" {
			return nil
		}
		if isInitializationError(line) {
			return errors.New(strings.TrimSpace(strings.TrimPrefix(line, "info string")))
		}
	}
}

// NewGame tells the engine that the next search is from a different game.
func (c *Client) NewGame() error {
	if err := c.send("ucinewgame"); err != nil {
		return fmt.Errorf("send ucinewgame: %w", err)
	}
	return c.ready()
}

// SetPosition sets the current FEN, optionally followed by the moves that led
// to it. Passing the move list lets the engine see the game history it needs
// for threefold-repetition detection; a bare FEN has no history. It waits for
// isready and reports an engine position error rather than silently searching a
// stale board.
func (c *Client) SetPosition(fen string, moves ...string) error {
	command := "position fen " + fen
	if len(moves) > 0 {
		command += " moves " + strings.Join(moves, " ")
	}
	if err := c.send(command); err != nil {
		return fmt.Errorf("send position: %w", err)
	}
	return c.ready()
}

// GoDepth runs a fixed-depth search and returns the final info and best move.
// A null or missing best move is returned as an empty BestMove.
func (c *Client) GoDepth(depth int) (EngineResult, error) {
	if depth < 1 {
		depth = 1
	}
	if err := c.send(fmt.Sprintf("go depth %d", depth)); err != nil {
		return EngineResult{}, fmt.Errorf("send go: %w", err)
	}

	start := time.Now()
	var result EngineResult
	for {
		line, err := c.readLine()
		if err != nil {
			return result, fmt.Errorf("read search: %w", err)
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "info":
			parseInfo(fields, &result)
		case "bestmove":
			if len(fields) >= 2 && fields[1] != "0000" && fields[1] != "(none)" {
				result.BestMove = fields[1]
			}
			if result.TimeMS < 1 {
				result.TimeMS = time.Since(start).Milliseconds()
				if result.TimeMS < 1 {
					result.TimeMS = 1
				}
			}
			return result, nil
		}
	}
}

func (c *Client) ready() error {
	if err := c.send("isready"); err != nil {
		return fmt.Errorf("send isready: %w", err)
	}
	var firstErr error
	for {
		line, err := c.readLine()
		if err != nil {
			return fmt.Errorf("read readyok: %w", err)
		}
		if line == "readyok" {
			return firstErr
		}
		if strings.HasPrefix(line, "info string") && strings.Contains(line, "error") && firstErr == nil {
			firstErr = errors.New(strings.TrimSpace(strings.TrimPrefix(line, "info string")))
		}
	}
}

func (c *Client) send(command string) error {
	if _, err := fmt.Fprintln(c.in, command); err != nil {
		return err
	}
	return nil
}

func (c *Client) readLoop() {
	defer close(c.readerDone)
	defer close(c.lines)
	for {
		line, err := c.out.ReadString('\n')
		if len(line) > 0 {
			text := strings.TrimSpace(line)
			select {
			case c.lines <- text:
			case <-c.stop:
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				c.readErr = err
			}
			return
		}
	}
}

func (c *Client) readLine() (string, error) {
	line, ok := <-c.lines
	if !ok {
		if c.readErr != nil {
			return "", c.readErr
		}
		return "", io.EOF
	}
	return line, nil
}

// Close sends quit, closes the pipes, and waits for the engine goroutine.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		_ = c.send("quit")
		_ = c.in.Close()
		if c.outCloser != nil {
			_ = c.outCloser.Close()
		}
		close(c.stop)
		<-c.done
		<-c.readerDone
	})
	return c.closeErr
}

func isInitializationError(line string) bool {
	return strings.HasPrefix(line, "info string") && strings.Contains(line, "initialization failed")
}

// parseInfo extracts the fields the web UI needs from one info line. Unknown
// fields and `info string` messages are ignored.
func parseInfo(fields []string, result *EngineResult) {
	if len(fields) < 2 || fields[1] == "string" {
		return
	}
	for i := 1; i < len(fields); i++ {
		switch fields[i] {
		case "depth":
			if value, ok := fieldInt(fields, i+1); ok && value >= 0 {
				result.Depth = value
			}
		case "nodes":
			if value, ok := fieldUint(fields, i+1); ok {
				result.Nodes = value
			}
		case "time":
			if value, ok := fieldInt(fields, i+1); ok && value >= 0 {
				result.TimeMS = int64(value)
			}
		case "score":
			if i+2 >= len(fields) {
				continue
			}
			value, ok := fieldInt(fields, i+2)
			if !ok {
				continue
			}
			switch fields[i+1] {
			case "cp":
				result.Score = value
				result.HasMate = false
				result.MateIn = 0
			case "mate":
				result.MateIn = value
				result.HasMate = true
			}
			i += 2
		case "pv":
			result.PV = append([]string(nil), fields[i+1:]...)
			return
		}
	}
}

func fieldInt(fields []string, index int) (int, bool) {
	if index >= len(fields) {
		return 0, false
	}
	value, err := strconv.Atoi(fields[index])
	return value, err == nil
}

func fieldUint(fields []string, index int) (uint64, bool) {
	if index >= len(fields) {
		return 0, false
	}
	value, err := strconv.ParseUint(fields[index], 10, 64)
	return value, err == nil
}
