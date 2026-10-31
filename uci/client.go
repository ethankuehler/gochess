package uci

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

// EngineResult scores are from the side to move's point of view.
type EngineResult struct {
	BestMove   string
	Score      int
	MateIn     int
	HasMate    bool
	Depth      int
	SelDepth   int
	Nodes      uint64
	NPS        uint64
	TimeMS     int64
	PV         []string
	Iterations []EngineIteration
	Metrics    *chess.SearchMetrics
}

type EngineIteration struct {
	Depth    int    `json:"depth"`
	SelDepth int    `json:"seldepth"`
	Nodes    uint64 `json:"nodes"`
	NPS      uint64 `json:"nps"`
	TimeMS   int64  `json:"timeMs"`
}

// Client commands must be serialized.
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

func (c *Client) NewGame() error {
	if err := c.send("ucinewgame"); err != nil {
		return fmt.Errorf("send ucinewgame: %w", err)
	}
	return c.ready()
}

func (c *Client) SetOption(name, value string) error {
	command := "setoption name " + name
	if value != "" {
		command += " value " + value
	}
	if err := c.send(command); err != nil {
		return fmt.Errorf("send setoption: %w", err)
	}
	return c.ready()
}

// SetPosition includes prior moves for repetition detection.
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

func (c *Client) GoDepth(depth int) (EngineResult, error) {
	if depth < 1 {
		depth = 1
	}
	return c.goCommand(fmt.Sprintf("go depth %d", depth))
}

func (c *Client) GoMovetime(budget time.Duration) (EngineResult, error) {
	ms := budget.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	return c.goCommand(fmt.Sprintf("go movetime %d", ms))
}

func (c *Client) GoNodes(nodes uint64) (EngineResult, error) {
	if nodes < 1 {
		nodes = 1
	}
	return c.goCommand(fmt.Sprintf("go nodes %d", nodes))
}

func (c *Client) goCommand(command string) (EngineResult, error) {
	if err := c.send(command); err != nil {
		return EngineResult{}, fmt.Errorf("send go: %w", err)
	}

	start := time.Now()
	var result EngineResult
	var analysisErr error
	for {
		line, err := c.readLine()
		if err != nil {
			return result, fmt.Errorf("read search: %w", err)
		}
		if strings.HasPrefix(line, "info string analysis ") {
			var metrics chess.SearchMetrics
			payload := strings.TrimPrefix(line, "info string analysis ")
			if err := json.Unmarshal([]byte(payload), &metrics); err != nil {
				// Drain through bestmove to keep the next command synchronized.
				if analysisErr == nil {
					analysisErr = fmt.Errorf("invalid analysis info: %w", err)
				}
			} else {
				result.Metrics = &metrics
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "info":
			if len(fields) >= 2 && fields[1] != "string" {
				parseInfo(fields, &result)
				recordIteration(&result)
			}
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
			if analysisErr != nil {
				return result, analysisErr
			}
			return result, nil
		}
	}
}

func recordIteration(result *EngineResult) {
	if result.Depth <= 0 {
		return
	}
	iteration := EngineIteration{
		Depth:    result.Depth,
		SelDepth: result.SelDepth,
		Nodes:    result.Nodes,
		NPS:      result.NPS,
		TimeMS:   result.TimeMS,
	}
	if len(result.Iterations) > 0 && result.Iterations[len(result.Iterations)-1].Depth == iteration.Depth {
		result.Iterations[len(result.Iterations)-1] = iteration
		return
	}
	result.Iterations = append(result.Iterations, iteration)
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
		case "seldepth":
			if value, ok := fieldInt(fields, i+1); ok && value >= 0 {
				result.SelDepth = value
			}
		case "nodes":
			if value, ok := fieldUint(fields, i+1); ok {
				result.Nodes = value
			}
		case "nps":
			if value, ok := fieldUint(fields, i+1); ok {
				result.NPS = value
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
