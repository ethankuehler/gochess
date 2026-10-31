// Command genmagics regenerates data/rook_magics.csv and data/bishop_magics.csv.
package main

import (
	"fmt"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

func main() {
	chess.LoadMagicsFromDisk = false
	chess.SaveMagicsToDisk = true

	start := time.Now()
	chess.BuildRookAttacks()
	chess.BuildBishopAttacks()
	fmt.Printf("wrote data/rook_magics.csv and data/bishop_magics.csv in %s\n", time.Since(start).Round(time.Millisecond))
}
