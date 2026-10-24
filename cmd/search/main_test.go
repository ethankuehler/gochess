package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// The chess package loads data/ via relative paths.
	os.Chdir("../..")
	os.Exit(m.Run())
}

func runSearch(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := run(args, &out, &errOut)
	return out.String(), err
}

func TestRunEval(t *testing.T) {
	out, err := runSearch(t, "-eval")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, want := range []string{
		"evaluation +0.00 (white)",
		"evaluation +0.00 (side to move)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

func TestRunEvalSideToMove(t *testing.T) {
	fen := "rnb1kbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR b KQkq - 0 1"
	out, err := runSearch(t, "-fen", fen, "-eval")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, want := range []string{
		"evaluation +9.08 (white)",
		"evaluation -9.08 (side to move)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

func TestRunMateInOne(t *testing.T) {
	out, err := runSearch(t,
		"-fen", "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 0 1",
		"-depth", "1",
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, want := range []string{
		"bestmove   a1a8",
		"score      mate in 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

func TestRunCheckmate(t *testing.T) {
	out, err := runSearch(t,
		"-fen", "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1",
		"-depth", "3",
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, want := range []string{
		"bestmove   (none)",
		"score      checkmate",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

func TestRunRejectsInvalidDepth(t *testing.T) {
	_, err := runSearch(t, "-depth", "0")
	if err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("error = %v, want a depth error", err)
	}
}

func TestRunRejectsUnexpectedArguments(t *testing.T) {
	_, err := runSearch(t, "extra")
	if err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("error = %v, want an unexpected-arguments error", err)
	}
}

func TestRunRejectsInvalidFEN(t *testing.T) {
	_, err := runSearch(t, "-fen", "not-a-fen")
	if err == nil || !strings.Contains(err.Error(), "parse FEN") {
		t.Fatalf("error = %v, want a parse error", err)
	}
}

func TestRunUsesMoveHistoryForDraws(t *testing.T) {
	out, err := runSearch(t,
		"-fen", "4k3/8/8/8/8/8/8/R3K3 w - - 0 1",
		"-moves", "e1d1 e8d8 d1e1 d8e8 e1d1 e8d8 d1e1 d8e8",
		"-depth", "1",
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, want := range []string{
		"depth      0",
		"draw       fifty-move or threefold repetition",
		"score      +0.00",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

func TestRunRejectsInvalidMove(t *testing.T) {
	_, err := runSearch(t, "-moves", "e2e5")
	if err == nil || !strings.Contains(err.Error(), "apply move") {
		t.Fatalf("error = %v, want an apply-move error", err)
	}
}
