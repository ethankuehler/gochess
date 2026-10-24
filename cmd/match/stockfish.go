package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var stockfishDivideLine = regexp.MustCompile(`^([a-h][1-8][a-h][1-8][qrbn]?): (\d+)$`)

// stockfishClient is a small UCI client for an external Stockfish process. It
// is used as both an opponent and an independent move-generation oracle.
type stockfishClient struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	out       *bufio.Reader
	closeOnce sync.Once
	err       error
}

func startStockfish(path string, errOut io.Writer) (*stockfishClient, error) {
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

	client := &stockfishClient{
		cmd:   cmd,
		stdin: stdin,
		out:   bufio.NewReaderSize(stdout, 64*1024),
	}
	if err := client.handshake(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *stockfishClient) handshake() error {
	if err := c.send("uci"); err != nil {
		return err
	}
	if _, err := c.readUntil("uciok"); err != nil {
		return fmt.Errorf("Stockfish uci handshake: %w", err)
	}
	if err := c.send("setoption name Threads value 1"); err != nil {
		return err
	}
	if err := c.send("setoption name Hash value 16"); err != nil {
		return err
	}
	return c.ready()
}

func (c *stockfishClient) ready() error {
	if err := c.send("isready"); err != nil {
		return err
	}
	if _, err := c.readUntil("readyok"); err != nil {
		return fmt.Errorf("Stockfish isready: %w", err)
	}
	return nil
}

func (c *stockfishClient) newGame() error {
	if err := c.send("ucinewgame"); err != nil {
		return err
	}
	return c.ready()
}

func (c *stockfishClient) send(command string) error {
	if _, err := fmt.Fprintln(c.stdin, command); err != nil {
		return fmt.Errorf("send Stockfish command %q: %w", command, err)
	}
	return nil
}

// setPosition sends a FEN plus the moves that led to it and waits for readyok.
// Stockfish reports illegal move sequences as an error line and may exit.
func (c *stockfishClient) setPosition(fen string, moves []string) error {
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
		return stockfishReadError("position", lines, err)
	}
	for _, line := range lines {
		if isStockfishError(line) {
			return fmt.Errorf("Stockfish rejected position: %s", line)
		}
	}
	return nil
}

// perft asks Stockfish for a perft divide. The returned map is keyed by UCI
// root move and includes every legal move at depth 1.
func (c *stockfishClient) perft(depth int) (map[string]uint64, uint64, error) {
	if depth < 1 {
		return nil, 0, fmt.Errorf("perft depth must be at least 1")
	}
	if err := c.send(fmt.Sprintf("go perft %d", depth)); err != nil {
		return nil, 0, err
	}
	lines, err := c.readUntilPrefix("Nodes searched:")
	if err != nil {
		return nil, 0, stockfishReadError("perft", lines, err)
	}

	divide := make(map[string]uint64)
	var total uint64
	foundTotal := false
	for _, line := range lines {
		if isStockfishError(line) {
			return nil, 0, fmt.Errorf("Stockfish perft: %s", line)
		}
		if rest, ok := strings.CutPrefix(line, "Nodes searched:"); ok {
			nodes, err := strconv.ParseUint(strings.TrimSpace(rest), 10, 64)
			if err != nil {
				return nil, 0, fmt.Errorf("parse Stockfish perft total %q: %w", line, err)
			}
			total = nodes
			foundTotal = true
			continue
		}
		match := stockfishDivideLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		nodes, err := strconv.ParseUint(match[2], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("parse Stockfish perft move %q: %w", line, err)
		}
		divide[match[1]] = nodes
	}
	if !foundTotal {
		return nil, 0, fmt.Errorf("Stockfish produced no perft total")
	}
	return divide, total, nil
}

// goDepth runs a fixed-depth search and returns the best move and score.
func (c *stockfishClient) goDepth(depth int) (move string, score int, hasMate bool, err error) {
	if depth < 1 {
		return "", 0, false, fmt.Errorf("search depth must be at least 1")
	}
	if err := c.send(fmt.Sprintf("go depth %d", depth)); err != nil {
		return "", 0, false, err
	}
	lines, err := c.readUntilPrefix("bestmove")
	if err != nil {
		return "", 0, false, stockfishReadError("search", lines, err)
	}

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
			value, err := strconv.Atoi(fields[i+2])
			if err != nil {
				continue
			}
			switch fields[i+1] {
			case "cp":
				score = value
				hasMate = false
			case "mate":
				score = value
				hasMate = true
			}
		}
	}
	if move == "" || move == "0000" || move == "(none)" {
		return "", score, hasMate, fmt.Errorf("Stockfish returned no move")
	}
	return move, score, hasMate, nil
}

func (c *stockfishClient) readUntil(sentinel string) ([]string, error) {
	return c.readLines(func(line string) bool { return line == sentinel })
}

func (c *stockfishClient) readUntilPrefix(prefix string) ([]string, error) {
	return c.readLines(func(line string) bool { return strings.HasPrefix(line, prefix) })
}

func (c *stockfishClient) readLines(done func(string) bool) ([]string, error) {
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

func (c *stockfishClient) Close() error {
	c.closeOnce.Do(func() {
		_ = c.send("quit")
		_ = c.stdin.Close()
		c.err = c.cmd.Wait()
	})
	return c.err
}

func isStockfishError(line string) bool {
	return strings.Contains(line, "CRITICAL ERROR") || strings.Contains(line, "Illegal move")
}

func stockfishReadError(operation string, lines []string, err error) error {
	if len(lines) == 0 {
		return fmt.Errorf("read Stockfish %s output: %w", operation, err)
	}
	return fmt.Errorf("read Stockfish %s output: %w (last line: %s)", operation, err, lines[len(lines)-1])
}
