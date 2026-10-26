package chess

// quiesce resolves forcing moves at a leaf: captures, en passant, and
// promotions. It counts its own node, so negamax must delegate at depth zero
// before calling tick. Every node in the search is counted exactly once.
//
// When the side to move is in check, stand-pat is forbidden and every legal
// evasion is searched; checkmate takes precedence over a draw claim. Otherwise
// the static score is the stand-pat lower bound and only forcing moves are
// searched. Quiescence does not use the transposition table and does not extend
// the principal variation.
func (s *searcher) quiesce(b Position, alpha, beta, ply int) int {
	if s.tick() {
		return 0
	}
	if ply > s.seldepth {
		s.seldepth = ply
	}
	if ply < len(s.pvLength) {
		s.pvLength[ply] = 0
	}
	if index := s.pathBase + ply; index < len(s.keys) {
		s.keys[index] = b.key
	}

	if ply >= maxPly {
		// Safety net. Threefold repetition and this cap are what terminate long
		// check-evasion sequences; mate detection is not attempted at the cap.
		if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() {
			return 0
		}
		return s.evaluate(&b)
	}

	var buf [256]Move
	if b.InCheck() {
		moves := s.legalMoves(&b, buf[:0])
		if len(moves) == 0 {
			return -mateScore + ply
		}
		if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() ||
			s.repetitionDraw(b.key, int(b.halfmoveClock), ply) {
			return 0
		}
		orderQSMoves(&b, moves)
		return s.searchQSMoves(&b, moves, -infinity, alpha, beta, ply)
	}

	if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() ||
		s.repetitionDraw(b.key, int(b.halfmoveClock), ply) {
		return 0
	}

	standPat := s.evaluate(&b)
	if standPat >= beta {
		if MetricsEnabled && s.metrics != nil {
			s.metrics.BetaCutoffs++
		}
		return standPat
	}
	if standPat > alpha {
		alpha = standPat
	}

	moves := s.legalMoves(&b, buf[:0])
	forcing := filterCapturesAndPromotions(moves)
	if len(forcing) == 0 {
		return standPat
	}
	orderQSMoves(&b, forcing)
	return s.searchQSMoves(&b, forcing, standPat, alpha, beta, ply)
}

// searchQSMoves searches moves with fail-soft alpha-beta. best is stand-pat at
// a non-check node and -infinity at an evasion node. It never updates the PV.
func (s *searcher) searchQSMoves(b *Position, moves []Move, best, alpha, beta, ply int) int {
	for _, m := range moves {
		child := s.makeMove(b, m)
		score := -s.quiesce(child, -beta, -alpha, ply+1)
		if s.stopped {
			return 0
		}
		if score > best {
			best = score
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			if MetricsEnabled && s.metrics != nil {
				s.metrics.BetaCutoffs++
			}
			break
		}
	}
	return best
}

// filterCapturesAndPromotions compacts captures, en passant, and promotions to
// the front of moves and returns the used prefix. It writes in place, so it
// does not allocate.
func filterCapturesAndPromotions(moves []Move) []Move {
	kept := moves[:0]
	for _, m := range moves {
		if _, promotion := m.Promotion(); m.IsCapture() || promotion {
			kept = append(kept, m)
		}
	}
	return kept
}

// orderQSMoves sorts moves by MVV-LVA in descending order. The insertion sort
// is stable, so equal scores keep generation order; quiet evasions score zero,
// which puts them after every capture and promotion in generation order.
func orderQSMoves(b *Position, moves []Move) {
	var scores [256]int
	for i, m := range moves {
		scores[i] = qsMoveScore(b, m)
	}
	for i := 1; i < len(moves); i++ {
		move, score := moves[i], scores[i]
		j := i - 1
		for j >= 0 && scores[j] < score {
			moves[j+1], scores[j+1] = moves[j], scores[j]
			j--
		}
		moves[j+1], scores[j+1] = move, score
	}
}

// qsMoveScore returns the MVV-LVA score of a move: the victim's value plus any
// promoted value, times 16, minus the attacker's value. Move does not encode
// the captured piece, so the victim comes from the board. An en-passant capture
// targets an empty square and is therefore scored as a pawn capture.
func qsMoveScore(b *Position, m Move) int {
	attacker := 0
	if _, piece, ok := b.PieceAt(m.From()); ok {
		attacker = pieceValues[piece]
	}
	victim := 0
	if m.IsCapture() {
		if _, piece, ok := b.PieceAt(m.To()); ok {
			victim = pieceValues[piece]
		} else {
			victim = pieceValues[Pawn]
		}
	}
	promoted := 0
	if piece, ok := m.Promotion(); ok {
		promoted = pieceValues[piece]
	}
	return (victim+promoted)*16 - attacker
}
