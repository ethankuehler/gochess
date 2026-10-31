package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type uciEngine struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	out       *bufio.Reader
	path      string
	name      string
	closeOnce sync.Once
	err       error
}

type engineOption struct {
	name  string
	value string
}

type engineSearchResult struct {
	BestMove string
	Score    int
	MateIn   int
	HasMate  bool
	Depth    int
	SelDepth int
	Nodes    uint64
	NPS      uint64
	TimeMS   int64
	HashFull int
	TBHits   uint64
	PV       []string
}

func startEngine(path string, options []engineOption, errOut io.Writer) (*uciEngine, error) {
	if errOut == nil {
		errOut = io.Discard
	}
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("engine stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("engine stdout: %w", err)
	}
	cmd.Stderr = errOut
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start engine %q: %w", path, err)
	}

	client := &uciEngine{
		cmd:   cmd,
		stdin: stdin,
		out:   bufio.NewReaderSize(stdout, 64*1024),
		path:  path,
	}
	if err := client.handshake(options); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *uciEngine) handshake(options []engineOption) error {
	if err := c.send("uci"); err != nil {
		return err
	}
	lines, err := c.readUntil("uciok")
	if err != nil {
		return fmt.Errorf("engine uci handshake: %w", err)
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "id name ") {
			c.name = strings.TrimSpace(strings.TrimPrefix(line, "id name "))
		}
	}

	base := []engineOption{
		{name: "Threads", value: "1"},
		{name: "Hash", value: "16"},
		{name: "Ponder", value: "false"},
		{name: "Move Overhead", value: "0"},
	}
	for _, opt := range append(base, options...) {
		if err := c.setOption(opt.name, opt.value); err != nil {
			return err
		}
	}
	return c.ready()
}

func (c *uciEngine) setOption(name, value string) error {
	return c.send("setoption name " + name + " value " + value)
}

func (c *uciEngine) ready() error {
	if err := c.send("isready"); err != nil {
		return err
	}
	if _, err := c.readUntil("readyok"); err != nil {
		return fmt.Errorf("%s isready: %w", c.label(), err)
	}
	return nil
}

func (c *uciEngine) newGame() error {
	if err := c.send("ucinewgame"); err != nil {
		return err
	}
	return c.ready()
}

func (c *uciEngine) send(command string) error {
	if _, err := fmt.Fprintln(c.stdin, command); err != nil {
		return fmt.Errorf("send %s command %q: %w", c.label(), command, err)
	}
	return nil
}

func (c *uciEngine) setPosition(fen string, moves []string) error {
	command := "position fen " + fen
	if len(moves) > 0 {
		command += " moves " + strings.Join(moves, " ")
	}
	if err := c.send(command); err != nil {
		return err
	}
	return c.ready()
}

func (c *uciEngine) goDepth(depth int) (engineSearchResult, error) {
	if depth < 1 {
		depth = 1
	}
	return c.goCommand(fmt.Sprintf("go depth %d", depth))
}

func (c *uciEngine) goMovetime(budget time.Duration) (engineSearchResult, error) {
	ms := budget.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	return c.goCommand(fmt.Sprintf("go movetime %d", ms))
}

func (c *uciEngine) goNodes(nodes uint64) (engineSearchResult, error) {
	if nodes < 1 {
		nodes = 1
	}
	return c.goCommand(fmt.Sprintf("go nodes %d", nodes))
}

func (c *uciEngine) goCommand(command string) (engineSearchResult, error) {
	if err := c.send(command); err != nil {
		return engineSearchResult{}, err
	}
	started := time.Now()
	lines, err := c.readUntilPrefix("bestmove")
	if err != nil {
		return engineSearchResult{}, engineReadError(c.label(), "search", lines, err)
	}

	var result engineSearchResult
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "info":
			parseEngineInfo(fields, &result)
		case "bestmove":
			if len(fields) >= 2 && fields[1] != "0000" && fields[1] != "(none)" {
				result.BestMove = fields[1]
			}
		}
	}
	if result.TimeMS < 1 {
		result.TimeMS = time.Since(started).Milliseconds()
		if result.TimeMS < 1 {
			result.TimeMS = 1
		}
	}
	if result.NPS == 0 && result.Nodes > 0 && result.TimeMS > 0 {
		result.NPS = result.Nodes * 1000 / uint64(result.TimeMS)
	}
	return result, nil
}

func (c *uciEngine) readUntil(sentinel string) ([]string, error) {
	return c.readLines(func(line string) bool { return line == sentinel })
}

func (c *uciEngine) readUntilPrefix(prefix string) ([]string, error) {
	return c.readLines(func(line string) bool { return strings.HasPrefix(line, prefix) })
}

func (c *uciEngine) readLines(done func(string) bool) ([]string, error) {
	var lines []string
	for {
		line, err := c.out.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
			if done(line) {
				return lines, nil
			}
		}
		if err != nil {
			return lines, err
		}
	}
}

func (c *uciEngine) Close() error {
	c.closeOnce.Do(func() {
		_ = c.send("quit")
		_ = c.stdin.Close()
		c.err = c.cmd.Wait()
	})
	return c.err
}

func (c *uciEngine) label() string {
	if c.name != "" {
		return c.name
	}
	return filepath.Base(c.path)
}

func parseEngineInfo(fields []string, result *engineSearchResult) {
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
		case "hashfull":
			if value, ok := fieldInt(fields, i+1); ok && value >= 0 {
				result.HashFull = value
			}
		case "tbhits":
			if value, ok := fieldUint(fields, i+1); ok {
				result.TBHits = value
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

func engineReadError(label, operation string, lines []string, err error) error {
	if len(lines) == 0 {
		return fmt.Errorf("read %s %s output: %w", label, operation, err)
	}
	return fmt.Errorf("read %s %s output: %w (last line: %s)", label, operation, err, lines[len(lines)-1])
}
