package main

import (
	"fmt"

	"github.com/ethankuehler/gochess/chess"
)

func main() {
	b, err := chess.NewBoardFEN("rnbqkb1r/1p2pppp/p2p1n2/8/3NP3/2N5/PPP2PPP/R1BQKB1R w KQkq - 0 6")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(b)

	m, err := chess.NewMoveUCI("e2e4")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(m)
}
