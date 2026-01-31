package main

import (
	"flag"
	"fmt"
	"log"
	"math/bits"
	"strings"

	"github.com/ethankuehler/gochess/chess"
)

func main() {
	var fen string
	var square string
	var piece string
	var colourFlag string

	flag.StringVar(&fen, "fen", "rnbqkb1r/1p2pppp/p2p1n2/8/3NP3/2N5/PPP2PPP/R1BQKB1R w KQkq - 0 6", "FEN string to load")
	flag.StringVar(&square, "square", "c1", "square of the example piece (e.g. c1)")
	flag.StringVar(&piece, "piece", "bishop", "piece type: bishop, rook, queen, knight, king, pawn")
	flag.StringVar(&colourFlag, "color", "white", "piece colour: white or black")
	flag.Parse()

	chess.BuildAllAttacks()

	board, err := chess.NewBoardFEN(fen)
	if err != nil {
		log.Fatalf("invalid FEN: %v", err)
	}

	fmt.Println("Loaded position:")
	fmt.Println(board.StringUni())
	fmt.Println()

	shift, err := chess.ShiftFromAlg(square)
	if err != nil {
		log.Fatalf("invalid square: %v", err)
	}

	colour := chess.WHITE
	if strings.ToLower(colourFlag) == "black" {
		colour = chess.BLACK
	}

	occupied := board.Occupied(chess.BOTH)

	var moves chess.BitBoard
	switch strings.ToLower(piece) {
	case "bishop":
		moves = chess.GetBishopAttack(shift, occupied)
	case "rook":
		moves = chess.GetRookAttack(shift, occupied)
	case "queen":
		moves = chess.GetQueenAttack(shift, occupied)
	case "knight":
		moves = chess.KNIGHT_ATTACKS[shift]
	case "king":
		moves = chess.KING_ATTACKS[shift]
	case "pawn":
		var attacks, forward chess.BitBoard
		if colour == chess.WHITE {
			attacks = chess.WHITE_PAWN_ATTACKS[shift]
			forward = chess.WHITE_PAWN_MOVES[shift]
		} else {
			attacks = chess.BLACK_PAWN_ATTACKS[shift]
			forward = chess.BLACK_PAWN_MOVES[shift]
		}
		enemy := chess.WHITE
		if colour == chess.WHITE {
			enemy = chess.BLACK
		}
		moves = (attacks & board.Occupied(enemy)) | (forward &^ board.Occupied(chess.BOTH))
	default:
		log.Fatalf("unsupported piece type: %s", piece)
	}

	// exclude friendly occupied squares
	moves &^= board.Occupied(colour)

	fmt.Printf("Possible moves for %s on %s:\n", piece, square)
	moveList := bitboardToSquares(moves)
	if len(moveList) == 0 {
		fmt.Println("  (no moves)")
		return
	}
	for _, m := range moveList {
		fmt.Printf("  %s\n", m)
	}
}

func bitboardToSquares(bb chess.BitBoard) []string {
	squares := []string{}
	for bb != 0 {
		lsb := bb & -bb
		shift := bits.TrailingZeros64(uint64(lsb))
		squares = append(squares, chess.AlgFromLoc(chess.BitBoard(1)<<shift))
		bb &= bb - 1
	}
	return squares
}
