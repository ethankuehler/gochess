package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// sfClient is a minimal external UCI client used for Stockfish. The arena only
// needs fixed-time and fixed-node limits plus a handful of strength options.
type sfClient struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	out       *bufio.Reader
	closeOnce sync.Once
	err       error
}

type sfOption struct {
	name  string
	value string
}

func startSF(path string, options []sfOption, errOut io.Writer) (*sfClient, error) {
	if errOut == nil {
		errOut = io.Discard
	}
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("Stockfish stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("Stockfish stdout: %w", err)
	}
	cmd.Stderr = errOut
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start Stockfish %q: %w", path, err)
	}

	client := &sfClient{
		cmd:   cmd,
		stdin: stdin,
		out:   bufio.NewReaderSize(stdout, 64*1024),
	}
	if err := client.handshake(options); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *sfClient) handshake(options []sfOption) error {
	if err := c.send("uci"); err != nil {
		return err
	}
	if _, err := c.readUntil("uciok"); err != nil {
		return fmt.Errorf("Stockfish uci handshake: %w", err)
	}
	// Fixed defaults keep games comparable and reproducible.
	base := []sfOption{
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

func (c *sfClient) setOption(name, value string) error {
	return c.send("setoption name " + name + " value " + value)
}

func (c *sfClient) ready() error {
	if err := c.send("isready"); err != nil {
		return err
	}
	if _, err := c.readUntil("readyok"); err != nil {
		return fmt.Errorf("Stockfish isready: %w", err)
	}
	return nil
}

func (c *sfClient) newGame() error {
	if err := c.send("ucinewgame"); err != nil {
		return err
	}
	return c.ready()
}

func (c *sfClient) send(command string) error {
	if _, err := fmt.Fprintln(c.stdin, command); err != nil {
		return fmt.Errorf("send Stockfish command %q: %w", command, err)
	}
	return nil
}

func (c *sfClient) setPosition(fen string, moves []string) error {
	command := "position fen " + fen
	if len(moves) > 0 {
		command += " moves " + strings.Join(moves, " ")
	}
	if err := c.send(command); err != nil {
		return err
	}
	return c.ready()
}

func (c *sfClient) goMovetime(budget time.Duration) (string, int, error) {
	ms := budget.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	return c.goCommand(fmt.Sprintf("go movetime %d", ms))
}

func (c *sfClient) goNodes(nodes uint64) (string, int, error) {
	if nodes < 1 {
		nodes = 1
	}
	return c.goCommand(fmt.Sprintf("go nodes %d", nodes))
}

func (c *sfClient) goCommand(command string) (string, int, error) {
	if err := c.send(command); err != nil {
		return "", 0, err
	}
	lines, err := c.readUntilPrefix("bestmove")
	if err != nil {
		return "", 0, stockfishReadError("search", lines, err)
	}

	var move string
	score := 0
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "bestmove" {
			if len(fields) >= 2 {
				move = fields[1]
			}
			continue
		}
		if fields[0] != "info" {
			continue
		}
		for i := 1; i+2 < len(fields); i++ {
			if fields[i] != "score" {
				continue
			}
			if fields[i+1] == "cp" {
				if value, err := strconv.Atoi(fields[i+2]); err == nil {
					score = value
				}
			}
		}
	}
	if move == "" || move == "0000" || move == "(none)" {
		return "", score, fmt.Errorf("Stockfish returned no move")
	}
	return move, score, nil
}

func (c *sfClient) readUntil(sentinel string) ([]string, error) {
	return c.readLines(func(line string) bool { return line == sentinel })
}

func (c *sfClient) readUntilPrefix(prefix string) ([]string, error) {
	return c.readLines(func(line string) bool { return strings.HasPrefix(line, prefix) })
}

func (c *sfClient) readLines(done func(string) bool) ([]string, error) {
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

func (c *sfClient) Close() error {
	c.closeOnce.Do(func() {
		_ = c.send("quit")
		_ = c.stdin.Close()
		c.err = c.cmd.Wait()
	})
	return c.err
}

func stockfishReadError(operation string, lines []string, err error) error {
	if len(lines) == 0 {
		return fmt.Errorf("read Stockfish %s output: %w", operation, err)
	}
	return fmt.Errorf("read Stockfish %s output: %w (last line: %s)", operation, err, lines[len(lines)-1])
}
