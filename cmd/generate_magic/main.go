package main

import (
	"fmt"
	"os"

	"github.com/ethankuehler/gochess/chess"
)

func main() {
	// Build attack tables for simple pieces first (needed for generation)
	chess.BuildKnightAttacks()
	chess.BuildKingAttacks()
	chess.BuildPawnMoves()
	chess.BuildPawnAttacks()

	fmt.Println("Generating Rook Magic Numbers...")
	rookMagics := chess.GenerateRookMagics()
	fmt.Println("\nSaving rook magics to data/rook_magic.csv...")
	if err := chess.SaveRookMagicsToCSV(rookMagics, "data/rook_magic.csv"); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving rook magics: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Rook magics saved successfully!")

	fmt.Println("\nGenerating Bishop Magic Numbers...")
	bishopMagics := chess.GenerateBishopMagics()
	fmt.Println("\nSaving bishop magics to data/bishop_magic.csv...")
	if err := chess.SaveBishopMagicsToCSV(bishopMagics, "data/bishop_magic.csv"); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving bishop magics: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Bishop magics saved successfully!")

	fmt.Println("\nAll magic numbers generated and saved!")
}
