package uci

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var processDivideLine = regexp.MustCompile(`^([a-h][1-8][a-h][1-8][qrbn]?): (\d+)$`)

type ProcessOption struct {
	Name  string
	Value string
}

type ProcessClient struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	out       *bufio.Reader
	path      string
	name      string
	closeOnce sync.Once
	closeErr  error
}

func StartProcess(path string, options []ProcessOption, errOut io.Writer, args ...string) (*ProcessClient, error) {
	if errOut == nil {
		errOut = io.Discard
	}
	cmd := exec.Command(path, args...)
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

	client := &ProcessClient{
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

func (c *ProcessClient) handshake(options []ProcessOption) error {
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
	for _, option := range options {
		command := "setoption name " + option.Name
		if option.Value != "" {
			command += " value " + option.Value
		}
		if err := c.send(command); err != nil {
			return err
		}
	}
	return c.ready()
}

func (c *ProcessClient) SetOption(name, value string) error {
	command := "setoption name " + name
	if value != "" {
		command += " value " + value
	}
	if err := c.send(command); err != nil {
		return err
	}
	return c.ready()
}

func (c *ProcessClient) NewGame() error {
	if err := c.send("ucinewgame"); err != nil {
		return err
	}
	return c.ready()
}

func (c *ProcessClient) SetPosition(fen string, moves ...string) error {
	command := "position fen " + fen
	if len(moves) > 0 {
		command += " moves " + strings.Join(moves, " ")
	}
	if err := c.send(command); err != nil {
		return err
	}
	if err := c.send("isready"); err != nil {
		return err
	}
	lines, err := c.readUntil("readyok")
	if err != nil {
		return processReadError(c.Label(), "position", lines, err)
	}
	for _, line := range lines {
		if isProcessError(line) {
			return fmt.Errorf("%s rejected position: %s", c.Label(), line)
		}
	}
	return nil
}

func (c *ProcessClient) GoDepth(depth int) (EngineResult, error) {
	if depth < 1 {
		depth = 1
	}
	return c.goCommand(fmt.Sprintf("go depth %d", depth))
}

func (c *ProcessClient) GoMovetime(budget time.Duration) (EngineResult, error) {
	ms := budget.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	return c.goCommand(fmt.Sprintf("go movetime %d", ms))
}

func (c *ProcessClient) GoNodes(nodes uint64) (EngineResult, error) {
	if nodes < 1 {
		nodes = 1
	}
	return c.goCommand(fmt.Sprintf("go nodes %d", nodes))
}

func (c *ProcessClient) goCommand(command string) (EngineResult, error) {
	if err := c.send(command); err != nil {
		return EngineResult{}, err
	}
	started := time.Now()
	lines, err := c.readUntilPrefix("bestmove")
	if err != nil {
		return EngineResult{}, processReadError(c.Label(), "search", lines, err)
	}

	var result EngineResult
	for _, line := range lines {
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

func (c *ProcessClient) Perft(depth int) (map[string]uint64, uint64, error) {
	if depth < 1 {
		return nil, 0, fmt.Errorf("perft depth must be at least 1")
	}
	if err := c.send(fmt.Sprintf("go perft %d", depth)); err != nil {
		return nil, 0, err
	}
	lines, err := c.readUntilPrefix("Nodes searched:")
	if err != nil {
		return nil, 0, processReadError(c.Label(), "perft", lines, err)
	}

	divide := make(map[string]uint64)
	var total uint64
	foundTotal := false
	for _, line := range lines {
		if isProcessError(line) {
			return nil, 0, fmt.Errorf("%s perft: %s", c.Label(), line)
		}
		if rest, ok := strings.CutPrefix(line, "Nodes searched:"); ok {
			nodes, err := strconv.ParseUint(strings.TrimSpace(rest), 10, 64)
			if err != nil {
				return nil, 0, fmt.Errorf("parse perft total %q: %w", line, err)
			}
			total = nodes
			foundTotal = true
			continue
		}
		match := processDivideLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		nodes, err := strconv.ParseUint(match[2], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("parse perft move %q: %w", line, err)
		}
		divide[match[1]] = nodes
	}
	if !foundTotal {
		return nil, 0, fmt.Errorf("%s produced no perft total", c.Label())
	}
	return divide, total, nil
}

func (c *ProcessClient) ready() error {
	if err := c.send("isready"); err != nil {
		return err
	}
	if _, err := c.readUntil("readyok"); err != nil {
		return fmt.Errorf("%s isready: %w", c.Label(), err)
	}
	return nil
}

func (c *ProcessClient) send(command string) error {
	if _, err := fmt.Fprintln(c.stdin, command); err != nil {
		return fmt.Errorf("send %s command %q: %w", c.Label(), command, err)
	}
	return nil
}

func (c *ProcessClient) readUntil(sentinel string) ([]string, error) {
	return c.readLines(func(line string) bool { return line == sentinel })
}

func (c *ProcessClient) readUntilPrefix(prefix string) ([]string, error) {
	return c.readLines(func(line string) bool { return strings.HasPrefix(line, prefix) })
}

func (c *ProcessClient) readLines(done func(string) bool) ([]string, error) {
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

func (c *ProcessClient) Close() error {
	c.closeOnce.Do(func() {
		_ = c.send("quit")
		_ = c.stdin.Close()
		c.closeErr = c.cmd.Wait()
	})
	return c.closeErr
}

func (c *ProcessClient) Label() string {
	if c.name != "" {
		return c.name
	}
	return filepath.Base(c.path)
}

func isProcessError(line string) bool {
	return strings.Contains(line, "CRITICAL ERROR") || strings.Contains(line, "Illegal move")
}

func processReadError(label, operation string, lines []string, err error) error {
	if len(lines) == 0 {
		return fmt.Errorf("read %s %s output: %w", label, operation, err)
	}
	return fmt.Errorf("read %s %s output: %w (last line: %s)", label, operation, err, lines[len(lines)-1])
}
