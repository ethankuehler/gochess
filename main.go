package main

import (
	"fmt"

	"github.com/ethankuehler/gochess/chess"
)

func main() {

	chess.BuildKingAttacks()
	chess.BuildKnightAttacks()
	chess.BuildPawnAttacks()
	chess.BuildPawnMoves()
	//fmt.Println(chess.whitePawnAttacks)
	//fmt.Println(chess.knightAttacks)
	b, err := chess.ParseFEN("rnbqkb1r/1p2pppp/p2p1n2/8/3NP3/2N5/PPP2PPP/R1BQKB1R w KQkq - 0 6")
	//b, err := chess.ParseFEN("rnbqkbnr/ppp2ppp/8/3Pp3/8/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 3")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(b.UnicodeString())
	var bb chess.BitBoard = 0
	for i := range chess.PieceTypes(chess.White) {
		bb |= b.PieceBitboard(chess.White, i)
	}
	fmt.Println(bb.String())
	m, err := chess.ParseUCIMove("e2e4")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(m)
}
