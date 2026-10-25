package chess

import "fmt"

// FindLegalMove matches a notation-only move against the legal moves,
// ignoring flags that UCI notation cannot express (capture, en passant,
// double push, and castling). The returned move is the generated move and
// carries the correct flags, so it can be passed to ApplyMove or makeMove.
//
// Promotion is part of the notation and must match. A move that is not legal
// returns false.
func (b *Position) FindLegalMove(candidate Move) (Move, bool) {
	ensureAttackTables()
	var buf [256]Move
	candidatePromotion, candidateHasPromotion := candidate.Promotion()
	for _, legal := range b.legalMoves(buf[:0]) {
		if legal.from != candidate.from || legal.to != candidate.to {
			continue
		}
		legalPromotion, legalHasPromotion := legal.Promotion()
		if legalHasPromotion != candidateHasPromotion {
			continue
		}
		if legalHasPromotion && legalPromotion != candidatePromotion {
			continue
		}
		return legal, true
	}
	return Move{}, false
}

// ApplyUCIMove parses and applies a UCI move such as "e2e4" or "e7e8q". It
// returns the resulting position and the generated legal move, which carries
// the move flags that the UCI notation omits. An illegal or malformed move
// returns an error and leaves the original position unchanged.
func (b *Position) ApplyUCIMove(uci string) (*Position, Move, error) {
	parsed, err := ParseUCIMove(uci)
	if err != nil {
		return nil, Move{}, err
	}
	legal, ok := b.FindLegalMove(*parsed)
	if !ok {
		return nil, Move{}, fmt.Errorf("illegal move %q", uci)
	}
	next := b.makeMove(legal)
	return &next, legal, nil
}
