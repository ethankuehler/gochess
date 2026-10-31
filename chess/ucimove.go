package chess

import "fmt"

// FindLegalMove restores the flags omitted by UCI notation.
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
