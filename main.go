package main

import (
	"fmt"

	"github.com/ethankuehler/gochess/chess"
)

func main() {

	chess.BuildKnightAttacks()
	chess.BuildPawnAttacks()
	chess.BuildPawnMoves()
	fmt.Println(chess.WHITE_PAWN_ATTACKS)
	fmt.Println(chess.KNIGHT_ATTACKS)
	//b, err := chess.NewBoardFEN("rnbqkb1r/1p2pppp/p2p1n2/8/3NP3/2N5/PPP2PPP/R1BQKB1R w KQkq - 0 6")
	b, err := chess.NewBoardFEN("rnbqkbnr/ppp2ppp/8/3Pp3/8/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 3")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(b)
	fmt.Println(b.FEN())

	m, err := chess.NewMoveUCI("e2e4")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(m)
}
