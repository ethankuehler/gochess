package chess

// quiesce resolves captures and promotions, or every evasion when in check.
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

func filterCapturesAndPromotions(moves []Move) []Move {
	kept := moves[:0]
	for _, m := range moves {
		if _, promotion := m.Promotion(); m.IsCapture() || promotion {
			kept = append(kept, m)
		}
	}
	return kept
}

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

// mvvLva treats an empty capture target as an en-passant pawn.
func mvvLva(b *Position, m Move) int {
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
	return victim*16 - attacker
}

func qsMoveScore(b *Position, m Move) int {
	score := mvvLva(b, m)
	if piece, ok := m.Promotion(); ok {
		score += pieceValues[piece] * 16
	}
	return score
}
