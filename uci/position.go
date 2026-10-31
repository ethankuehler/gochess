package uci

import (
	"fmt"
	"strings"

	"github.com/ethankuehler/gochess/chess"
)

// handlePosition implements:
//
//	position startpos [moves ...]
//	position fen <fen> [moves ...]
func (e *Engine) handlePosition(args []string) {
	if e.isSearching() {
		fmt.Fprintln(e.errOut, "position ignored while searching")
		return
	}
	if err := e.ensureInitialized(); err != nil {
		e.positionFailed(fmt.Errorf("initialization failed: %w", err))
		return
	}

	position, history, err := parsePosition(args)
	if err != nil {
		e.positionFailed(err)
		return
	}

	e.mu.Lock()
	e.position = position
	e.history = history
	e.posValid = true
	e.hasPosition = true
	e.mu.Unlock()
}

func (e *Engine) positionFailed(err error) {
	e.out.line("info string position error: %v", err)
	e.mu.Lock()
	e.position = nil
	e.history = nil
	e.posValid = false
	e.hasPosition = true
	e.mu.Unlock()
}

func parsePosition(args []string) (*chess.Position, []chess.PositionKey, error) {
	if len(args) == 0 {
		return nil, nil, fmt.Errorf("position: expected startpos or fen")
	}

	var (
		position *chess.Position
		index    int
	)

	switch strings.ToLower(args[0]) {
	case "startpos":
		position = chess.NewStartingPosition()
		index = 1
	case "fen":
		index = 1
		fenFields := make([]string, 0, 6)
		for index < len(args) && len(fenFields) < 6 && !strings.EqualFold(args[index], "moves") {
			fenFields = append(fenFields, args[index])
			index++
		}
		if len(fenFields) < 4 {
			return nil, nil, fmt.Errorf("position: FEN has %d fields, expected at least 4", len(fenFields))
		}
		for len(fenFields) < 6 {
			if len(fenFields) == 4 {
				fenFields = append(fenFields, "0")
			} else {
				fenFields = append(fenFields, "1")
			}
		}
		parsed, err := chess.ParseFEN(strings.Join(fenFields, " "))
		if err != nil {
			return nil, nil, err
		}
		position = parsed
	default:
		return nil, nil, fmt.Errorf("position: expected startpos or fen, got %q", args[0])
	}

	history := make([]chess.PositionKey, 0, len(args))
	if index < len(args) {
		if !strings.EqualFold(args[index], "moves") {
			return nil, nil, fmt.Errorf("position: unexpected token %q", args[index])
		}
		index++
		for ; index < len(args); index++ {
			history = append(history, position.Key())
			next, _, err := position.ApplyUCIMove(args[index])
			if err != nil {
				return nil, nil, fmt.Errorf("position: move %q: %w", args[index], err)
			}
			position = next
		}
	}

	return position, history, nil
}
