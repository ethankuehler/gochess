package uci

import (
	"fmt"
	"os"
	"testing"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/data"
)

func TestMain(m *testing.M) {
	if err := chess.BuildAllAttacksFromFS(data.FS); err != nil {
		fmt.Fprintf(os.Stderr, "initialize attack tables: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
