// Command genmagics regenerates the rook and bishop magic-number tables in
// data/. Run it from the repository root:
//
//	go run ./cmd/genmagics
//
// It rewrites data/rook_magics.csv and data/bishop_magics.csv. The search is
// random, so the resulting numbers differ on every run but are equally valid.
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
