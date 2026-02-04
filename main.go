package main

import (
	"fmt"

	"github.com/ethankuehler/gochess/chess"
)

func main() {

	chess.BuildAllAttacks()
	b, err := chess.NewBoardFEN("rnbqkb1r/1p2pppp/p2p1n2/8/3NP3/2N5/PPP2PPP/R1BQKB1R w KQkq - 0 6")
	//b, err := chess.NewBoardFEN("rnbqkbnr/ppp2ppp/8/3Pp3/8/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 3")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(b.StringUni())
	fmt.Println(b.String())
	fmt.Println(b.FEN())

	// manual testing of various functions
	fmt.Println("testing bishope moves at c1")
	loc, err := chess.ShiftFromAlg("c1") // locaton of bishop
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(loc)
	blockers := b.Occupied(chess.BOTH)
	fmt.Printf("blockers: %064b\n", blockers)
	fmt.Println(blockers.String())
	mask := chess.GetBishopAttack(loc, blockers)
	fmt.Println(mask.String())
	newloc, _ := chess.LocFromAlg("c1")
	fmt.Println(newloc.String())
}
