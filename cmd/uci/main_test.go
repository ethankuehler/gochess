package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/data"
	"github.com/ethankuehler/gochess/uci"
)

func buildUCI(t *testing.T) string {
	t.Helper()
	name := "gochess-uci"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Env = os.Environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}
	return binary
}

func TestBinaryRunsOutsideRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}

	binary := buildUCI(t)
	if err := chess.BuildAllAttacksFromFS(data.FS); err != nil {
		t.Fatalf("initialize attack tables: %v", err)
	}
	workDir := t.TempDir()
	cmd := exec.Command(binary)
	cmd.Dir = workDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	waited := false
	t.Cleanup(func() {
		if !waited && cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})

	lines := make(chan string, 256)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	send := func(line string) {
		t.Helper()
		if _, err := fmt.Fprintln(stdin, line); err != nil {
			t.Fatalf("send %q: %v", line, err)
		}
	}
	waitFor := func(prefix string) string {
		t.Helper()
		deadline := time.After(10 * time.Second)
		for {
			select {
			case line, ok := <-lines:
				if !ok {
					t.Fatalf("engine output ended while waiting for %q; stderr: %s", prefix, stderr.String())
				}
				if strings.HasPrefix(line, prefix) {
					return line
				}
			case <-deadline:
				t.Fatalf("timed out waiting for %q; stderr: %s", prefix, stderr.String())
			}
		}
	}

	send("uci")
	waitFor("id name " + uci.EngineName)
	waitFor("uciok")
	send("isready")
	waitFor("readyok")
	send("position startpos")
	send("go depth 2")
	bestmove := strings.Fields(waitFor("bestmove "))[1]
	if bestmove == "0000" {
		t.Fatal("engine returned no move from the starting position")
	}
	parsed, err := chess.ParseUCIMove(bestmove)
	if err != nil {
		t.Fatalf("bestmove %q: %v", bestmove, err)
	}
	if _, ok := chess.NewStartingPosition().FindLegalMove(*parsed); !ok {
		t.Fatalf("bestmove %q is not legal", bestmove)
	}

	send("quit")
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		waited = true
		if err != nil {
			t.Fatalf("engine exited with %v; stderr: %s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Fatal("engine did not exit after quit")
	}
}

func TestBenchCLIOutsideRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}

	binary := buildUCI(t)
	cmd := exec.Command(binary, "bench", "2")
	cmd.Dir = t.TempDir()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bench: %v\n%s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		t.Fatal("bench produced no output")
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) != 4 || fields[1] != "nodes" || fields[3] != "nps" {
		t.Fatalf("bench final line = %q", lines[len(lines)-1])
	}
	nodes, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil || nodes == 0 {
		t.Fatalf("bench nodes = %q (%v)", fields[0], err)
	}
}

func TestVersionFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}

	binary := buildUCI(t)
	cmd := exec.Command(binary, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), uci.VersionString()) {
		t.Fatalf("version output = %q, want %q", output, uci.VersionString())
	}
}
