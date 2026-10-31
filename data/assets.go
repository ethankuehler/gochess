// Package data embeds the runtime attack tables.
package data

import "embed"

//go:embed knight_attacks.csv king_attacks.csv white_pawn_move.csv black_pawn_move.csv white_pawn_attacks.csv black_pawn_attacks.csv rook_magics.csv bishop_magics.csv
var FS embed.FS
