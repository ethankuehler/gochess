# Search & Evaluation Plan

**Status:** implemented
**Last updated:** 2026-10-24
**Scope:** fixed-depth alpha-beta search and a hand-written evaluation for standard
chess; no UCI yet

## 1. Goal

Add the first engine layer on top of move generation:

- `Position.Evaluate()` — a static score in centipawns from White's point of view.
- `Position.Search(depth)` — a fixed-depth negamax alpha-beta search that returns
  the best move, the score, and a node count.
- `cmd/search` — a small CLI to inspect evaluation and search from a FEN.

The evaluation is deliberately small and explainable. It scores four things:

1. material (piece values),
2. centrality of the pieces (the user-facing "major pieces" term),
3. king safety through a corner / back-rank bonus,
4. pawn advancement toward promotion.

The search is deliberately plain: negamax + alpha-beta. No transposition table,
no quiescence, no iterative deepening, no time management. Those are listed as
follow-ups, not hidden scope.

## 2. What already exists

The move-generation layer gives us everything search needs:

- `Position.LegalMoves()` returns `[]Move`; the internal `legalMoves(buf)` variant
  writes into a caller-supplied buffer and avoids allocation in the hot path.
- `Position.makeMove(m)` is copy-make and returns a `Position` value.
- `Position.InCheck()`, `SideToMove()`, `PieceAt()`, and the unexported
  `kingSquare()` / `sideToMove()` are available inside the `chess` package.
- `Position.halfmoveClock` already tracks the fifty-move counter.
- A `Move` is a comparable value (two squares plus flags), so it can be stored in
  `SearchResult` directly.

There is no evaluation, no search, and no game-history type. The search will
therefore be stateless with respect to repetition and will only know the current
position.

### 2.1 Why negamax

A plain minimax has two almost-identical branches (White maximizes, Black
minimizes). Negamax uses one routine: evaluate from the side to move's point of
view and negate on the way back up. Alpha-beta is then the standard window
`(-beta, -alpha)` on the recursive call. This keeps the search code about half
the size of a two-sided minimax and matches how the evaluation is already signed.

## 3. Evaluation

### 3.1 Units and material

Scores are integer centipawns: `+100` means "White is up one pawn" from White's
point of view. Piece values are the usual simplified-evaluation numbers:

| Piece | Value (cp) |
| --- | ---: |
| Pawn | 100 |
| Knight | 320 |
| Bishop | 330 |
| Rook | 500 |
| Queen | 900 |
| King | 0 |

Watch the `Piece` constant order: `Pawn, Bishop, Knight, Rook, Queen, King`
(bishop is index 1, knight is index 2). Value tables must use the named
constants, not a hand-written order:

```go
var pieceValues = [6]int{
    Pawn: 100, Bishop: 330, Knight: 320, Rook: 500, Queen: 900, King: 0,
}
```

### 3.2 Centrality of the major pieces

Use one `centrality[64]` table, `0` on the rim through `5` in the four central
squares, and a per-piece weight in centipawns per table step:

| Piece | Weight | Max bonus |
| --- | ---: | ---: |
| Pawn | 0 | 0 |
| Knight | 5 | 25 |
| Bishop | 4 | 20 |
| Rook | 3 | 15 |
| Queen | 4 | 20 |
| King | 0 | 0 |

Knights and bishops get the largest weights because they are short-range pieces
that really do improve toward the center. Rooks and the queen get a smaller nudge;
"major pieces" is interpreted here as the queen and rooks, with the minor pieces
included at slightly different weights. If we want strict Q+R only, set the
knight and bishop weights to zero — nothing else changes.

```go
var centralityWeight = [6]int{
    Pawn: 0, Bishop: 4, Knight: 5, Rook: 3, Queen: 4, King: 0,
}
```

The table, written rank 1 first because `square = file + rank*8`:

```go
var centrality = [64]int{
    0, 0, 1, 2, 2, 1, 0, 0, // rank 1
    0, 1, 2, 3, 3, 2, 1, 0, // rank 2
    1, 2, 3, 4, 4, 3, 2, 1, // rank 3
    2, 3, 4, 5, 5, 4, 3, 2, // rank 4
    2, 3, 4, 5, 5, 4, 3, 2, // rank 5
    1, 2, 3, 4, 4, 3, 2, 1, // rank 6
    0, 1, 2, 3, 3, 2, 1, 0, // rank 7
    0, 0, 1, 2, 2, 1, 0, 0, // rank 8
}
```

### 3.3 Kings at the corner

A second `kingSafety[64]` table rewards the king for being on a corner or castled
back-rank square and penalizes it for wandering into the center. Values are from
White's perspective:

```go
var kingSafety = [64]int{
    30, 10, 25, 5, 5, 10, 25, 30,   // rank 1
    10, 5, 0, -5, -5, 0, 5, 10,     // rank 2
    -5, -10, -15, -20, -20, -15, -10, -5, // rank 3
    -10, -15, -20, -25, -25, -20, -15, -10, // rank 4
    -10, -15, -20, -25, -25, -20, -15, -10, // rank 5
    -5, -10, -15, -20, -20, -15, -10, -5, // rank 6
    10, 5, 0, -5, -5, 0, 5, 10,     // rank 7
    30, 10, 25, 5, 5, 10, 25, 30,   // rank 8
}
```

So `a1`/`h1` get `+30`, the castled squares `c1`/`g1` get `+25`, and a king on
`e4` gets `-25`. The table is vertically symmetric; black uses the same table on
the mirrored square. The symmetry is what makes the "black king on a8" case
behave like "white king on a1", at the cost of also giving a white king on a8 a
corner bonus. That case is rare enough to ignore for now.

This term is intentionally crude. It encodes "castled/cornered is safe", not a
real king-safety model, and it will discourage king centralization in king-and-pawn
endgames. The fix (blend king safety with king activity by game phase) is a
follow-up, not part of this pass.

### 3.4 Pawns near promotion

A rank table gives a white pawn an increasing bonus as it approaches the eighth
rank; black pawns use the mirrored rank:

```go
var pawnAdvance = [8]int{0, 0, 5, 10, 20, 40, 80, 0} // index = zero-based rank
```

A white pawn on rank 2 gets `0`, on rank 6 gets `+40`, and on rank 7 gets `+80`
(on top of its `100` material value). A black pawn on rank 2 gets the mirrored
`+80`. This is an advancement bonus, not passed-pawn detection: it does not know
whether the pawn can actually promote or is about to be captured. The search is
responsible for resolving that.

### 3.5 One piece-square table

Instead of four separate loops at evaluation time, precompute a
`[12][64]int` table once at package initialization:

```go
var pieceSquareValues = buildPieceSquareValues()

func buildPieceSquareValues() (table [12][64]int) {
    for piece := Pawn; piece <= King; piece++ {
        for square := Square(0); square < squareCount; square++ {
            table[piece][square] = pieceSquareScore(piece, square)
            mirrored := square ^ 56 // flip the rank
            table[piece+blackPieceOffset][square] = -pieceSquareScore(piece, mirrored)
        }
    }
    return table
}

func pieceSquareScore(piece Piece, square Square) int {
    score := pieceValues[piece] + centralityWeight[piece]*centrality[square]
    switch piece {
    case King:
        score += kingSafety[square]
    case Pawn:
        score += pawnAdvance[FileRankFromSquare(square).rank]
    }
    return score
}
```

Black entries are stored already negated, so `Evaluate` is a single sum over all
twelve bitboards:

```go
// Evaluate returns the static score in centipawns from White's point of view.
func (b *Position) Evaluate() int {
    score := 0
    for index, board := range b.pieceBitboards {
        for board != 0 {
            square := Square(bits.TrailingZeros64(uint64(board)))
            board &= board - 1
            score += pieceSquareValues[index][square]
        }
    }
    return score
}
```

This has no allocations and no dependency on the attack tables, so it is cheap to
test and benchmark in isolation.

### 3.6 Worked examples

- **Starting position:** material is equal, the pawns are on rank 2/7 with no
  advance bonus, the kings on `e1`/`e8` cancel, and the piece squares mirror, so
  `Evaluate() == 0`. This is a good first assertion.
- **Knight `g1` → `d4`:** centrality `0 → 5`, weight `5`, so `+25`.
- **King `e1` → `g1`:** `+5 → +25`, so `+20`. Castling also moves the rook
  `h1 → f1`, centrality `0 → 1`, weight `3`, for another `+3`.
- **Pawn `e2` → `e7`:** advancement `0 → 80`, so `+80`.
- **Black:** every term is mirrored and negated; a black knight on `d5` scores
  the same as a white knight on `d4`.

## 4. Search

### 4.1 API

```go
// MateScore is the score of a checkmate at the root. Scores within 128 points
// of ±MateScore encode forced mates and describe the distance to mate.
const MateScore = 1_000_000

// SearchResult is the outcome of a fixed-depth alpha-beta search.
type SearchResult struct {
    BestMove Move   // valid only when HasMove is true
    Score    int    // centipawns from the side to move's point of view
    HasMove  bool   // false for checkmate and stalemate
    Depth    int
    Nodes    uint64
}

// Search returns the best move for the side to move at the given depth in plies.
func (b *Position) Search(depth int) SearchResult

// ScoreToMate converts a mate score into a distance in moves. ok is false for
// ordinary evaluations. Positive means the side to move delivers mate.
func ScoreToMate(score int) (moves int, ok bool)
```

`Score` is from the side to move's perspective, matching the negamax convention.
`Evaluate` remains White-relative. The CLI prints both when asked.

```go
func ScoreToMate(score int) (moves int, ok bool) {
    switch {
    case score > mateScore-maxPly:
        return (mateScore - score + 1) / 2, true
    case score < -mateScore+maxPly:
        return -((mateScore + score + 1) / 2), true
    default:
        return 0, false
    }
}
```

`Search` requires `depth >= 1` and panics otherwise (programmer error, like the
other package-level preconditions). It calls `ensureAttackTables()`.

The zero value of `Move` is `a1a1`, which is not a legal move, so `HasMove` is
needed to distinguish "no move" from "the move is a1a1" (which is impossible
anyway, but the flag keeps the zero result honest).

### 4.2 Negamax alpha-beta

```go
const (
    mateScore = MateScore
    maxPly    = 128
    infinity  = mateScore + 1
)

type searcher struct {
    nodes uint64
}

func (s *searcher) negamax(b Position, depth, alpha, beta, ply int) int {
    s.nodes++

    // Fifty-move rule: the position is a claimable draw.
    if b.halfmoveClock >= 100 {
        return 0
    }

    // At the horizon, still detect checkmate so a depth-1 search finds mate in 1.
    var buf, ordered [256]Move
    if depth == 0 {
        if b.InCheck() && len(b.legalMoves(buf[:0])) == 0 {
            return -mateScore + ply
        }
        return b.evaluateForSideToMove()
    }

    moves := b.legalMoves(buf[:0])
    if len(moves) == 0 {
        if b.InCheck() {
            return -mateScore + ply // checkmate, prefer a faster mate
        }
        return 0 // stalemate
    }

    best := -infinity
    for _, m := range orderMoves(moves, ordered[:0]) {
        child := b.makeMove(m)
        score := -s.negamax(child, depth-1, -beta, -alpha, ply+1)
        if score > best {
            best = score
        }
        if score > alpha {
            alpha = score
        }
        if alpha >= beta {
            break // beta cutoff
        }
    }
    return best
}

func (b *Position) evaluateForSideToMove() int {
    score := b.Evaluate()
    if b.sideToMove() == Black {
        score = -score
    }
    return score
}
```

Notes:

- `ply` is only used to make mate scores prefer shorter mates. A mate at ply 1
  returns `-mateScore + 1` from the child, which the root negates to
  `mateScore - 1`; `ScoreToMate` turns that into "mate in 1".
- `mateScore` is far above any possible material score (a legal FEN is capped at
  16 pieces and 8 pawns per side), so mate scores cannot be confused with
  evaluations.
- The depth-0 checkmate test costs one legal-move generation only when the side
  to move is in check. Stalemate exactly at the horizon is not detected and is
  scored statically; that is a small, documented inaccuracy until quiescence
  exists.
- `Position` is passed **by value** into `negamax`. Passing `&child` made the
  copy-make child escape and cost one heap allocation per expanded node; the
  value parameter keeps the search allocation-free.

### 4.3 Root

```go
func (b *Position) Search(depth int) SearchResult {
    ensureAttackTables()
    if depth < 1 {
        panic("chess: search depth must be at least 1")
    }

    result := SearchResult{Depth: depth}
    var buf, ordered [256]Move
    moves := b.legalMoves(buf[:0])
    if len(moves) == 0 {
        if b.InCheck() {
            result.Score = -mateScore
        }
        return result
    }

    s := searcher{}
    alpha, beta := -infinity, infinity
    best := -infinity
    for _, m := range orderMoves(moves, ordered[:0]) {
        child := b.makeMove(m)
        score := -s.negamax(child, depth-1, -beta, -alpha, 1)
        if score > best {
            best = score
            result.BestMove = m
            result.HasMove = true
        }
        if score > alpha {
            alpha = score
        }
    }
    result.Score = best
    result.Nodes = s.nodes
    return result
}
```

The root uses a full `beta` window, so it returns a true minimax score and the
best root move. Individual non-best root moves may have fail-low (upper-bound)
scores, which is fine; only the best move is reported in this pass.

### 4.4 Move ordering

Alpha-beta only pays off when good moves are searched first. The first pass is a
cheap stable partition: captures, en passant, and promotions before quiet moves.
No transposition table, no killer moves, no history heuristic.

```go
func orderMoves(moves, scratch []Move) []Move {
    out := scratch[:0]
    for _, m := range moves {
        if _, promotion := m.Promotion(); m.IsCapture() || promotion {
            out = append(out, m)
        }
    }
    for _, m := range moves {
        if _, promotion := m.Promotion(); !m.IsCapture() && !promotion {
            out = append(out, m)
        }
    }
    return out
}
```

`scratch` is a caller-owned `[256]Move`; returning a slice backed by a local array
would make it escape and allocate on every node. The same reasoning applies to
`buf` passed to `legalMoves`.

If the node counts warrant it, the capture block can be sorted by MVV-LVA
(most valuable victim, least valuable attacker) in a later phase. Ordering must
never change the search result, only the number of nodes visited; the
alpha-beta-vs-minimax test below enforces that.

### 4.5 Draws

- **Fifty-move rule:** checked at node entry with `b.halfmoveClock >= 100`, which
  is exactly 50 full moves. The clock is maintained by `makeMove`.
- **Threefold repetition:** not implemented. `Position` has no history, and the
  CLI is stateless. A search that is winning may shuffle instead of making
  progress; that is acceptable for this pass.
- **Insufficient material:** not implemented. The evaluation will happily call
  K+B vs K "winning". Deferred.

### 4.6 Performance expectations

- `Evaluate` should be a few hundred nanoseconds and allocation-free.
- With capture-first ordering, effective branching is roughly 5-8 instead of
  ~30, so startpos depth 4 should be on the order of 10^4 nodes and finish in
  milliseconds; depth 6 should be seconds at worst.
- Copy-make is already the move-application strategy; search does not change
  that. If search throughput becomes a problem, make/unmake and a transposition
  table are the next big steps, in that order.

These are expectations to measure with benchmarks, not gates.

## 5. CLI

New command `cmd/search`, run from the repo root:

```sh
go run ./cmd/search -depth 4
go run ./cmd/search -fen "r1bqkbnr/pppp1ppp/2n5/4p3/2B1P3/5Q2/PPPP1PPP/RNB1K1NR w KQkq - 4 4" -depth 5
go run ./cmd/search -fen "<FEN>" -eval
```

Flags:

```text
-fen <FEN>    position (default: starting position)
-depth N      search depth in plies, at least 1 (default 4)
-eval         print the static evaluation and exit
```

Output shape:

```text
fen        rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1
depth      4
bestmove   e2e4
score      +0.12 (side to move)
nodes      41203
time       31ms
nps        1329129
```

`-eval` prints:

```text
evaluation +0.00 (white)
evaluation +0.00 (side to move)
```

The CLI validates `-depth`, builds the attack tables once, parses the FEN, and
prints a mate distance via `ScoreToMate` when the score is a mate score. A
positive distance means the engine delivers mate, a negative distance means it is
being mated; both are printed as "mate in N" with the sign spelled out. It should
print "no legal moves: checkmate" or "no legal moves: stalemate" when
`HasMove` is false, instead of an empty best move.

## 6. Files and phases

| Phase | Work | Exit criteria |
| --- | --- | --- |
| 1 | `chess/eval.go`: material, centrality, king, pawn tables, `Evaluate` | starting eval is 0; eval term and symmetry tests pass |
| 2 | `chess/search.go`: negamax alpha-beta, terminal/mate handling, `Search`, `ScoreToMate` | mate-in-1, checkmate, stalemate, and best-move-legal tests pass |
| 3 | Move ordering (captures/promotions first), alpha-beta-vs-minimax test | search scores match unpruned minimax on all fixtures |
| 4 | `cmd/search` with `-fen`, `-depth`, `-eval` | CLI runs from the repo root and prints bestmove/nodes/nps |
| 5 | Benchmarks, docs, optional Stockfish spot check | `gofmt`, `go build`, `go vet`, `go test -race ./...` green |

New files:

- `chess/eval.go` — values, tables, `Evaluate`.
- `chess/search.go` — `SearchResult`, `Search`, negamax, ordering, mate helpers.
- `chess/eval_test.go`.
- `chess/search_test.go`.
- `cmd/search/main.go` (and a small `cmd/search/main_test.go` if the `run`
  function is testable with an `io.Writer`).

Updated files:

- `chess/bench_test.go` — `BenchmarkEvaluate`, `BenchmarkSearch`.
- `AGENTS.md` — search/eval descriptions, CLI command, testing notes.
- `README.md` — a short "run the search CLI" snippet.

No changes to `data/` or the existing move-generation API.

## 7. Tests

### 7.1 Evaluation

- **Starting position:** `Evaluate() == 0`.
- **Colour symmetry:** for a set of FENs (startpos, Kiwipete, a middlegame
  position, a KQ vs K endgame), mirror the position vertically and swap colours;
  assert `Evaluate(mirror) == -Evaluate(original)`. The test helper reverses the
  ranks, swaps piece case, flips the side to move, reverses the castling field,
  and mirrors the en passant square.
- **Material:** removing the black queen from the starting position should raise
  the eval by exactly the black queen's contribution
  (`pieceSquareScore(Queen, d1)`, including its `d8` square term). Compute the
  expected value from the table in the test so tuning the table does not silently
  break the test.
- **Centrality:** knight on `d4` vs knight on `a2`, same kings — delta `+25`
  (`5 * 5`).
- **King corner:** king on `a1` vs king on `e4`, same black king — delta `+55`
  (`30 - (-25)`).
- **Pawn advance:** white pawn on `e7` vs `e2`, same kings — delta `+80`.
- **Table mirror:** every black entry equals the negated white entry on the
  rank-flipped square.

Fixture FENs for the term tests (verified with the current move generator):

| Case | FEN |
| --- | --- |
| Knight `d4` | `8/8/8/8/3N4/8/8/K6k w - - 0 1` |
| Knight `a2` | `8/8/8/8/8/8/N7/K6k w - - 0 1` |
| King `a1` | `7k/8/8/8/8/8/8/K7 w - - 0 1` |
| King `e4` | `7k/8/8/8/4K3/8/8/8 w - - 0 1` |
| Pawn `e2` | `7k/8/8/8/8/8/4P3/K7 w - - 0 1` |
| Pawn `e7` | `7k/4P3/8/8/8/8/8/K7 w - - 0 1` |

### 7.2 Search

| Test | Position | Expectation |
| --- | --- | --- |
| Mate in 1 | `6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 0 1` | depth 1 plays `a1a8`, `ScoreToMate` says 1 |
| Checkmated | `7k/6Q1/6K1/8/8/8/8/8 b - - 0 1` | `HasMove == false`, score `-MateScore` |
| Stalemate | `7k/5Q2/6K1/8/8/8/8/8 b - - 0 1` | `HasMove == false`, score 0 |
| Free queen | `4k3/8/8/3q4/8/8/3R4/4K3 w - - 0 1` | depth 2 plays `d2d5` |
| Poisoned pawn | `8/8/8/8/4k3/4p3/8/4Q1K1 w - - 0 1` | depth 1 takes `e1e3`; depth 2 does not (the king recaptures) |

The poisoned-pawn case documents the horizon effect: at depth 1 the queen grab
looks free, at depth 2 the recapture is visible. It is the clearest argument for
quiescence later.

More search invariants:

- every reported best move appears in `LegalMoves()` and applies cleanly;
- search does not mutate the position;
- search is deterministic (same position and depth give the same move, score, and
  node count);
- `Search(0)` panics with the documented message;
- `ScoreToMate` round-trips mate scores and returns `ok == false` for ordinary
  evaluations.

### 7.3 Alpha-beta correctness

Implement a reference `minimax(b, depth, ply)` in `search_test.go` with the same
evaluation and terminal rules but no pruning. For several fixtures (startpos,
Kiwipete, the poisoned-pawn position, a queen endgame) and depths 1-3, assert:

```go
position.Search(depth).Score == minimax(position, depth, 0)
```

This is the test that makes alpha-beta refactoring safe: if a cutoff or window is
wrong, scores diverge. Keep the depths small so `go test -race ./...` stays fast.

Optionally log the node counts for alpha-beta vs minimax on Kiwipete depth 3 to
show pruning is doing something; do not assert a brittle exact ratio.

### 7.4 Benchmarks

Add to `chess/bench_test.go`:

- `BenchmarkEvaluate` on the starting position, reporting allocs/op.
- `BenchmarkSearchDepth4` on the starting position, reporting nodes and nodes/s.

Record the numbers in the plan's outcome section after implementation; do not gate
CI on performance.

### 7.5 Optional Stockfish spot check

The repo already has Stockfish plumbing in `cmd/perft`. Once search works, an
opt-in test can ask Stockfish for `go depth N` on a handful of tactical FENs and
compare the best move, or compare our mate distance against Stockfish's mate
announcement. This is not required for CI and is not part of the definition of
done.

## 8. Gotchas

- **Evaluation is White-relative; search is side-to-move-relative.** The negation
  in `evaluateForSideToMove` is the classic bug. The symmetry test catches it.
- **Piece order is not the usual one.** `Pawn, Bishop, Knight, Rook, Queen, King`
  and a black offset of 6. Name the indices; never hand-order the value table.
- **Black evaluation mirrors the square** with `square ^ 56`, not by flipping the
  file, and stores the negated value.
- **Mate scores are not evaluations.** Keep `MateScore` at 1,000,000 and all
  static scores far below `MateScore - maxPly`.
- **No legal moves is not necessarily checkmate.** Test `InCheck()` at the same
  node; stalemate is 0, checkmate is `-mateScore + ply`.
- **Depth-0 checkmate.** The leaf check exists so a depth-1 search finds mate in
  1. Without it, `Search(1)` returns a static eval for a mate.
- **`Move` has no null value.** `HasMove` exists because the zero `Move` is
  `a1a1`.
- **`ParseUCIMove` is notation-only.** Never feed it to `makeMove` or compare it
  directly to a generated move (it lacks capture/castle/en-passant flags); search
  always uses moves from `legalMoves`.
- **Caller-owned buffers.** `legalMoves` and `orderMoves` write into slices
  supplied by the caller. Returning a slice backed by a local array makes it
  escape and defeats the allocation-free goal.
- **Fifty-move counter is in plies** (`halfmoveClock >= 100`, not `> 100`).
- **The king-corner term is phase-blind.** It will hurt king activity in
  endgames; document it and revisit before treating the engine's endgames as
  meaningful.
- **Search depth is plies, not full moves.** State this in the CLI help text.
- **Attack tables.** `Search` calls `ensureAttackTables()`; `Evaluate` does not
  need them.
- **Determinism.** Order moves deterministically; never iterate a map in the hot
  path.

## 9. Non-goals for this pass

Quiescence search, transposition tables, iterative deepening, aspiration windows,
PVS/NegaScout, null-move pruning, late-move reductions, killer/history heuristics,
opening books, endgame tablebases, repetition and insufficient-material draws,
UCI, time controls, pondering, and parallel search. Wiring the engine into
`cmd/web` was added in a follow-up after this pass; see section 12.

Quiescence is the first follow-up after this plan: a captures-only search at the
horizon removes most of the horizon blunders (including the poisoned-pawn case)
and is much smaller than a transposition table.

## 10. Definition of done

- `Evaluate` implemented with the four requested terms; starting position scores 0
  and the symmetry test passes.
- `Search` returns legal best moves at depths 1-4, detects checkmate and
  stalemate, and reports mate distances.
- Alpha-beta scores match unpruned minimax on the fixture set at depths 1-3.
- `go run ./cmd/search -depth 4` works from the repo root and prints bestmove,
  score, nodes, time, and nps.
- `gofmt -l .`, `go build ./...`, `go vet ./...`, and `go test -race ./...` are
  clean.
- `AGENTS.md` and `README.md` describe the new files, command, and conventions.
- Benchmarks record evaluation and search throughput.

## 11. Decisions taken

The implementation used the proposed defaults; each remains a small, isolated
change:

1. **Centrality scope:** knights/bishops/rooks/queens, with the weights in
   `centralityWeight`.
2. **King term:** static corner/back-rank bonus. A phase blend is the obvious
   first evaluation improvement.
3. **Pawn term:** rank advancement only; no passed-pawn detection.
4. **CLI and web:** added `cmd/search` and an engine toggle in `cmd/web`
   (human White, engine Black).

## 12. Outcome

Implemented on 2026-10-24.

What landed:

- `chess/eval.go` — material, centrality, king-safety, and pawn-advance tables,
  the precomputed `pieceSquareValues` table, `Evaluate`, and
  `evaluateForSideToMove`.
- `chess/search.go` — `MateScore`, `SearchResult`, fixed-depth negamax
  alpha-beta `Search`, capture/promotion-first ordering, and `ScoreToMate`.
- `cmd/search` — `-fen`, `-depth`, and `-eval`, with bestmove, score, nodes,
  time, and nps output.
- `chess/eval_test.go` and `chess/search_test.go` — the term/symmetry and
  mate/tactic/alpha-beta tests described above, plus a full-width reference
  minimax for scores at depths 1-3.
- `cmd/search/main_test.go` — CLI coverage via `run(args, out, errOut)`.
- Benchmarks in `chess/bench_test.go`; `README.md` and `AGENTS.md` updated.
- `cmd/web` engine mode (follow-up) — `/api/engine` settings, `/api/engine/move`
  search reply, engine fields in `/api/state`, engine-aware undo, and guards
  that keep the human from moving for the engine. The panel has an engine
  toggle and a 1-5 ply depth selector; the engine plays Black and the human
  plays White.

One implementation note changed the design during phase 2: `negamax` takes
`Position` by value rather than `*Position`. Passing `&child` made the copy-make
child escape, costing one heap allocation per expanded node; with a value
parameter the search is allocation-free.

Verification:

- `gofmt -l .`, `go build ./...`, and `go vet ./...` are clean.
- `go test ./...` and `go test -race ./...` pass.
- Alpha-beta scores match the reference minimax at depths 1-3 on the start
  position, Kiwipete, the poisoned-pawn position, and a queen endgame.
- Evaluation is exactly `0` in the starting position and negates under the
  colour-mirror test.
- `go run ./cmd/search -depth 4` on the start position:
  `bestmove b1c3`, score `+0.00`, 12,957 nodes.
- `go run ./cmd/search -fen "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 0 1" -depth 1`
  reports `bestmove a1a8` and `mate in 1`.
- The web engine mode was smoke-tested over HTTP: with the engine enabled at
  depth 3, `e2e4` got the reply `g8f6`; undo returned to the starting position,
  and a human Black move was rejected with 409 while it was the engine's turn.
- Benchmarks on the development machine (Apple M2 Pro):
  - `BenchmarkEvaluate`: 25.3 ns/op, 0 B/op, 0 allocs/op.
  - `BenchmarkSearchDepth4`: 4.5 ms/op, 12,957 nodes/op, ~2.9M nodes/s,
    0 B/op, 0 allocs/op.

Known limits, all deferred by design: no quiescence, so the engine can blunder
at the horizon; no transposition table, so repeated positions are re-searched;
the king term is phase-blind; repetition and insufficient-material draws are not
detected; and there is no time management or UCI layer.
