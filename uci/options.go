package uci

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

const (
	defaultMoveOverheadMS = 50
	maxMoveOverheadMS     = 5000
)

// options holds the settings that are actually implemented. Ponder, MultiPV,
// and similar options are deliberately absent: setoption accepts them so a GUI
// does not break, but advertising them would promise behaviour the engine does
// not have.
type options struct {
	moveOverhead time.Duration
	threads      int
	hashMB       int
	analysis     bool
}

func defaultOptions() options {
	return options{
		moveOverhead: defaultMoveOverheadMS * time.Millisecond,
		threads:      1,
		hashMB:       chess.DefaultHashMB,
	}
}

// lines returns the option declarations sent in response to uci.
func (o options) lines() []string {
	lines := []string{
		fmt.Sprintf("option name Hash type spin default %d min %d max %d", chess.DefaultHashMB, chess.MinHashMB, chess.MaxHashMB),
		"option name Clear Hash type button",
		"option name Threads type spin default 1 min 1 max 1",
		fmt.Sprintf("option name Move Overhead type spin default %d min 0 max %d", defaultMoveOverheadMS, maxMoveOverheadMS),
	}
	if metricsSupported {
		lines = append(lines, "option name Analysis type check default false")
	}
	return lines
}

// set applies a setoption value. It returns a message worth reporting to the
// GUI, or an empty string. Unknown options and unsupported options are
// accepted and ignored.
func (o *options) set(name, value string, hasValue bool) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "move overhead":
		if !hasValue {
			return ""
		}
		ms, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fmt.Sprintf("invalid Move Overhead value %q", value)
		}
		if ms < 0 {
			ms = 0
		}
		if ms > maxMoveOverheadMS {
			ms = maxMoveOverheadMS
		}
		o.moveOverhead = time.Duration(ms) * time.Millisecond
		return ""
	case "threads":
		if !hasValue {
			return ""
		}
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fmt.Sprintf("invalid Threads value %q", value)
		}
		if n > 1 {
			return "Threads is fixed at 1; this engine is single-threaded"
		}
		o.threads = 1
		return ""
	case "analysis":
		if !hasValue {
			return ""
		}
		switch {
		case strings.EqualFold(strings.TrimSpace(value), "true"):
			o.analysis = true
		case strings.EqualFold(strings.TrimSpace(value), "false"):
			o.analysis = false
		default:
			return fmt.Sprintf("invalid Analysis value %q", value)
		}
		return ""
	case "hash":
		if !hasValue {
			return ""
		}
		mb, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fmt.Sprintf("invalid Hash value %q", value)
		}
		if mb < chess.MinHashMB {
			mb = chess.MinHashMB
		}
		if mb > chess.MaxHashMB {
			mb = chess.MaxHashMB
		}
		o.hashMB = mb
		return ""
	case "clear hash":
		// The engine clears its table in handleSetOption; the option parser
		// only needs to accept the button.
		return ""
	case "ponder":
		// Ponder is not advertised. Ignore the value rather than beginning to
		// ponder unexpectedly.
		return ""
	case "uci_chess960":
		if hasValue && strings.EqualFold(strings.TrimSpace(value), "true") {
			return "Chess960 is not supported"
		}
		return ""
	default:
		return ""
	}
}

// parseSetOption splits "setoption name <id> [value <x>]" arguments. The name
// and value are joined back with single spaces because UCI allows both to
// contain spaces.
func parseSetOption(args []string) (name, value string, hasValue, ok bool) {
	if len(args) == 0 || !strings.EqualFold(args[0], "name") {
		return "", "", false, false
	}
	valueIndex := -1
	for i := 1; i < len(args); i++ {
		if strings.EqualFold(args[i], "value") {
			valueIndex = i
			break
		}
	}
	if valueIndex == -1 {
		name = strings.Join(args[1:], " ")
		return name, "", false, name != ""
	}
	name = strings.Join(args[1:valueIndex], " ")
	value = strings.Join(args[valueIndex+1:], " ")
	return name, value, true, name != ""
}
