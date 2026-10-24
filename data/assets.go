// Package data embeds the runtime attack tables so that the engine binary is
// self-contained. The CSV files in this directory remain the canonical source
// data; cmd/genmagics still writes them, and the other commands still read them
// from disk.
package data

import "embed"

// FS holds the attack tables and magic numbers used by BuildAllAttacksFromFS.
//
//go:embed knight_attacks.csv king_attacks.csv white_pawn_move.csv black_pawn_move.csv white_pawn_attacks.csv black_pawn_attacks.csv rook_magics.csv bishop_magics.csv
var FS embed.FS
