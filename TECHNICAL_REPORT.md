# gochess: Algorithms and Techniques

## 1. Overview

`gochess` is a single-threaded chess engine written in Go using only the standard library. Its main search pipeline is:

1. Represent the position with bitboards and compact state flags.
2. Generate pseudo-legal moves using precomputed attack tables.
3. Apply each candidate with copy-make and reject moves that leave the king attacked.
4. Search the resulting game tree with iterative-deepening negamax and alpha-beta pruning.
5. Score leaf positions using material and hand-written piece-square tables.
6. Detect repetitions and other draws using incremental Zobrist keys and position history.
7. Expose the engine through UCI, with asynchronous search, cancellation, and clock management.

The design prioritizes correctness, simplicity, and protocol reliability. It contains the standard foundation of a chess engine, but not many of the features needed for strong modern play, such as a transposition table, quiescence search, or advanced move ordering.

| Area | Technique used | Main implementation |
| --- | --- | --- |
| Board representation | 12 piece bitboards plus packed state | `chess/bitboard.go`, `chess/const.go` |
| Non-sliding attacks | Precomputed square-indexed tables | `chess/magic.go` |
| Sliding attacks | Magic-bitboard hashing | `chess/magic.go` |
| Legal move generation | Pseudo-legal generation followed by king-safety filtering | `chess/movegen.go` |
| Making moves | Immutable-style copy-make with incremental hash updates | `chess/makemove.go` |
| Evaluation | Material and precomputed piece-square values | `chess/eval.go` |
| Repetition detection | Deterministic Zobrist hashing and bounded history scans | `chess/repetition.go`, `chess/search.go` |
| Search | Iterative-deepening negamax with alpha-beta pruning | `chess/search.go` |
| Move ordering | Previous best root move, then captures and promotions first | `chess/search.go` |
| UCI and timing | Separate search goroutine, soft/hard deadlines, context cancellation | `uci/search.go`, `uci/go.go` |
| Rules verification | Perft fixtures and differential testing against Stockfish | `chess/perft.go`, `cmd/perft`, `cmd/match` |

## 2. Board and move representation

### 2.1 Bitboards

A `BitBoard` is a `uint64`, with one bit for each square. The mapping is little-endian rank-file:

```text
square = file + 8 * rank
a1 = 0, b1 = 1, ..., h1 = 7, a2 = 8, ..., h8 = 63
```

This representation makes operations such as occupancy unions, attack intersections, and square removal inexpensive bitwise operations. Set bits are iterated with `bits.TrailingZeros64`, and cleared with `board &= board - 1`.

`Position` contains:

- Twelve bitboards: six piece types for White and six for Black.
- A packed `uint8` containing side to move and four castling rights.
- A one-bit bitboard for the en passant target square.
- Halfmove and fullmove clocks.
- A 64-bit incremental Zobrist key.

- White and black occupancy caches, updated incrementally by `makeMove`.
- A cached en passant contribution to the Zobrist key.

`Occupancy` is therefore O(1). The constructors rebuild both caches from the piece bitboards, and `makeMove` updates them alongside the board. Legality filtering uses a lighter copy that updates only the board and occupancy, because the key, clocks, castling rights, and en passant target are irrelevant to the king-safety test.

### 2.2 Moves

A move is a 4-byte value: the origin and destination squares as `uint8` indices (0–63) plus a `uint16` flag word. `Square` itself remains an `int` so the public API and square arithmetic are unchanged; only the stored fields narrow, through the internal `newMove` constructor. The compact layout shrinks the principal-variation table from about 399 KB to 66 KB and the per-node move scratch buffers from about 12 KB to 2 KB. It also brings the per-search `searcher` allocation under the Go compiler's stack-variable limit, so the search state is no longer heap-allocated. The flags distinguish:

- Captures.
- Double pawn pushes.
- En passant.
- Castling.
- Promotions, with the promotion piece encoded in the low three flag bits.

UCI notation does not contain enough information to reconstruct all these flags. Consequently, input moves are parsed and matched against generated legal moves by `FindLegalMove`; the flagged generated move is then applied. This avoids treating, for example, a castling or en passant move as an ordinary from/to move.

## 3. Attack generation

### 3.1 Pawns, knights, and kings

Knight and king attacks, along with pawn pushes and attacks, are loaded from square-indexed tables. Runtime lookup is therefore an array access followed by bitboard masking.

The data files are embedded in the UCI executable. At startup, `BuildAllAttacksFromFS` loads all tables into temporary values and only updates the package state after every input has been validated. This makes initialization atomic with respect to invalid data and allows the executable to run independently of its working directory.

Pawn attack detection during king-safety checks uses shifts and file masks rather than the pawn attack tables. The masks prevent a shift from wrapping across the board's left or right edge.

### 3.2 Magic bitboards for sliding pieces

Rooks and bishops use magic bitboards. A slow ray-walking reference function, `SlidingAttacks`, includes squares along each direction until and including the first occupied square.

For fast lookup, the engine first selects only occupancy bits that can affect attacks from the given square:

```text
blockers = occupancy & relevantMask
```

It then multiplies those blockers by a precomputed 64-bit magic number and keeps the high bits:

```text
index = (uint64(blockers) * magic) >> (64 - indexBits)
attacks = attackTable[square][index]
```

The relevant mask excludes the origin and the terminal board-edge square of each ray. Excluding those irrelevant bits reduces the number of occupancy configurations and therefore the table size.

A valid magic may map different blocker configurations to the same slot only if they produce identical attacks. `tryMagic` enumerates every relevant blocker subset using the standard Carry-Rippler-style subset step:

```text
blockers = (blockers - mask) & mask
```

It rejects a candidate when two configurations with different attacks collide. Stored magic numbers are validated and their attack tables rebuilt at initialization. Queens reuse the rook and bishop lookups and OR the results, so they require no separate table.

This gives constant-time sliding attack lookup at runtime, at the cost of memory for per-square tables and a one-time table-construction cost.

## 4. Legal move generation

Move generation has two stages: generate pseudo-legal moves, then filter them for king safety.

### 4.1 Pseudo-legal generation

For the side to move, the engine calculates own, enemy, and total occupancy. It then generates:

- **Pawn moves:** single and double pushes from the push table, diagonal captures from the attack table, en passant, and all four promotion choices.
- **Knights and kings:** precomputed attacks excluding squares occupied by friendly pieces.
- **Bishops, rooks, and queens:** magic lookups using total occupancy, again excluding friendly destinations.
- **Castling:** generated separately because it moves two pieces and has additional safety conditions.

Targets are tested against enemy occupancy to set the capture flag. Promotions generate queen, rook, bishop, and knight moves in that order.

A double push is discarded if either its destination or intermediate square is occupied. Castling requires the appropriate right, the king and rook on their original squares, empty intervening squares, and no attack on the king's origin, transit, or destination square. Transit-square tests use occupancy with the king removed from its origin, which matters for attacks previously hidden behind the king.

### 4.2 King-safety filtering

Each pseudo-legal move is applied to a copied position. The resulting position is queried to determine whether the opponent attacks the mover's king; if so, the move is discarded.

This single mechanism handles ordinary pins, moving into check, illegal king moves, and en passant discoveries without separate pin or check-evasion logic. It is straightforward and robust, but more expensive than generating only legal moves directly.

### 4.3 Copy-make move application

`makeMove` copies the entire `Position`, updates the copy, and leaves the parent unchanged. It handles:

- Removing and placing pieces.
- Captures, including the off-destination pawn removed during en passant.
- Replacing a promoted pawn with its selected piece.
- Moving the rook during castling.
- Clearing castling rights when a king or rook home square is involved, including rook capture on its home square.
- Setting an en passant target only after a double push.
- Resetting the halfmove clock after a pawn move or capture, and incrementing it otherwise.
- Toggling side to move and incrementing the fullmove number after Black moves.
- Updating the Zobrist key incrementally.

Copy-make simplifies recursion, non-mutation, cancellation, and caller-owned position state. Its trade-off is more copying than an in-place make/unmake design, although the search passes positions by value to encourage stack allocation.

## 5. Static evaluation

`Position.Evaluate` returns an integer score in centipawns from White's perspective: `+100` means an advantage approximately equal to one pawn. The search converts this to the side-to-move perspective by negating the score when Black is to move.

### 5.1 Evaluation terms

The evaluation combines:

| Piece/feature | Values or approach |
| --- | --- |
| Pawn | 100 centipawns, plus an advancement bonus |
| Knight | 320 centipawns, plus centrality |
| Bishop | 330 centipawns, plus centrality |
| Rook | 500 centipawns, plus centrality |
| Queen | 900 centipawns, plus centrality |
| King | No material value; a king-safety square table |
| Pawn advancement | Increasing rank bonuses, reaching 80 on the seventh rank |
| Centrality | Small per-square bonuses, weighted by piece type |
| King safety | Rewards corner/castled locations and penalizes central exposure |

Material and positional terms are folded into one precomputed `[12][64]int` table. Black's entries use the rank-mirrored square (`square ^ 56`) and negate White's value. Evaluation is therefore a single sum over the pieces present on the board; it requires no attack-table initialization and does not mutate the position.

### 5.2 Evaluation characteristics and limitations

This is a fast, colour-symmetric baseline rather than a detailed positional model. It currently has no explicit terms for:

- Mobility, piece coordination, outposts, or trapped pieces.
- Doubled, isolated, connected, or passed-pawn structure beyond advancement.
- Bishop pairs, rook files, or king pawn-shield structure.
- Game-phase interpolation or separate endgame king activity.
- Tempo.

The king-safety table is deliberately phase-blind, so central king activity is not rewarded in the endgame. These omissions limit playing strength even when the search itself is tactically correct.

## 6. Position identity, repetition, and draws

### 6.1 Zobrist hashing

Each position receives a 64-bit Zobrist key formed by XORing random-looking contributions for:

- Every piece type and colour on every square.
- Each of the 16 castling-right combinations.
- The file of a usable en passant target.
- Black to move.

The random values are generated deterministically with SplitMix64, so keys are reproducible across runs without storing a random table. FEN parsing computes a key from scratch; `makeMove` XORs out changed old state and XORs in the new state.

Move clocks are excluded because they do not affect whether the board position is the same for repetition. An en passant target is included only when a pawn of the side to move is positioned to make the capture. This check is pseudo-legal: it does not test whether the capture would expose that pawn's king.

As with all fixed-width Zobrist systems, collisions are theoretically possible. Keys are compared directly and are not used as persistent identifiers.

### 6.2 Repetition history

The UCI `position ... moves ...` handler records the key of every position before applying the next move. These keys become `SearchLimits.GameHistory`. During search, prior game keys are followed by keys from the current search path, allowing repetition detection across the boundary between played moves and hypothetical moves.

Only recent history is relevant:

- History is limited to the halfmove-clock window because a pawn move or capture prevents earlier positions from recurring in the current sequence.
- At most 100 prior game positions are retained for the search.
- A node scans backward through this bounded key array and returns a draw when the current key has already occurred twice, making the current occurrence the third.

Repetitions need not be consecutive. Search currently folds a third repetition to a draw score; it does not use the more aggressive single-repetition heuristic sometimes used by engines.

### 6.3 Other draw rules

The halfmove clock directly supports:

- Fifty-move draws at 100 halfmoves, which are claimable.
- Seventy-five-move automatic draws at 150 halfmoves in game adjudication.

Insufficient-material detection recognizes positions with no pawns, rooks, or queens when there is at most one minor piece, or when all bishops are confined to one colour complex. It is intentionally a simple dead-position test, not a complete classifier of every theoretically dead position.

At the root, fifty-move, threefold, and insufficient-material draws return score zero and `Draw=true`, while still supplying a legal `bestmove` as UCI requires. In the tree they score zero. Legal move generation and checkmate detection happen first, so checkmate takes precedence over a draw. Game-level interfaces additionally handle fivefold repetition and automatic seventy-five-move draws.

## 7. Search algorithm

### 7.1 Negamax

The core is depth-limited negamax. Minimax's two-player maximization/minimization is expressed with one recursive rule:

```text
score(position, depth) = -score(child, depth - 1)
```

Every score is therefore relative to the player to move at that node. At depth zero, the engine normally returns the static evaluation. If the side to move is in check, it still generates legal moves to detect checkmate at the horizon; this allows a depth-one search to recognize mate in one. Stalemate exactly at the horizon is not specially detected and instead receives the static evaluation.

At non-terminal internal nodes:

- No legal moves plus check gives checkmate.
- No legal moves without check gives stalemate, scored zero.
- A recognized draw gives zero.
- Otherwise, all legal children are searched recursively.

Mate is represented by a very large score, `-1_000_000 + ply` for the side being mated. The ply term makes the engine prefer faster forced mates and delay forced losses. `ScoreToMate` converts these scores into UCI mate distances.

### 7.2 Alpha-beta pruning

Each recursive call receives an alpha-beta window. Alpha records the best score already guaranteed to the side to move; beta is the opponent's corresponding bound. When `alpha >= beta`, the remaining siblings cannot affect the parent decision and are skipped.

The implementation uses full windows at the root and fail-soft bound cutoffs in recursion. It does not currently add principal-variation search, aspiration windows, late-move reductions, null-move pruning, or search extensions.

### 7.3 Iterative deepening

The root searches depth 1, then depth 2, and so on until a limit is reached. After every completed iteration it saves:

- Best move and score.
- Principal variation.
- Completed depth and maximum reached ply (`seldepth`).
- Cumulative node count and elapsed time.

The previous iteration's best root move is promoted to the front for the next iteration. This usually improves alpha-beta cutoffs. Iterative deepening also provides a good completed result when time expires during a deeper iteration.

If cancellation occurs before depth 1 completes, the engine returns the first legal root move with depth zero and score zero rather than returning an incomplete search result. Terminal positions return no move.

### 7.4 Move ordering and principal variation

At non-root nodes, captures, en passant, and all promotions are placed before quiet moves. Their relative order otherwise remains the generated order; there is no MVV-LVA, static exchange evaluation, killer-move table, history heuristic, or countermove table. Better ordering is important because it changes the number of nodes alpha-beta must visit, although not the result of an unpruned fixed-depth search.

The principal variation is stored in a triangular, ply-indexed table. When a move improves a node, that move is prepended to the child's saved line. The second root PV move is exposed as a possible ponder move, but actual pondering is not implemented.

### 7.5 Search limits and cancellation

Search supports:

- Maximum depth, capped at 128 plies.
- Maximum nodes.
- Hard and soft wall-clock deadlines.
- Context cancellation.
- A legal root-move restriction for UCI `searchmoves`.
- Prior game history for repetition detection.

The node limit is checked at every node. Context and hard-deadline checks occur every 256 nodes and at root-move boundaries. A soft deadline is checked between completed iterations; a hard deadline can interrupt an iteration. Results always come from the last completed iteration, except for the legal fallback described above. Node counts are cumulative across iterative-deepening iterations and exclude the root itself.

The most consequential search limitation is the lack of quiescence search: most leaf nodes are evaluated immediately, even if captures are pending. This can create horizon effects in tactical positions. There is also no transposition table, so identical positions reached by different move orders are searched again.

## 8. UCI integration and time management

The protocol layer separates the command loop from search. A search runs in its own goroutine with a cancellable context, leaving the command loop able to answer `isready` and process `stop`, `quit`, or end-of-file. Protocol output is serialized and flushed so search information and command responses do not interleave incorrectly.

The engine reports completed-iteration information including depth, selective depth, centipawn or mate score, nodes, nodes per second, elapsed time, and PV. Checkmate and stalemate produce `bestmove 0000`. Infinite searches hold a completed result until `stop`, except when the root is already terminal.

For `go movetime`, the requested duration minus `Move Overhead` becomes both the soft and hard budget, with a minimum of one millisecond. For clock-based play, the initial soft allocation is approximately:

```text
soft = remainingTime / movesToGo + 3 * increment / 4
```

`movesToGo` defaults to 30 and is capped at 50. The soft allocation is limited to a fraction of remaining time, and the hard allowance is normally twice the soft allowance, bounded by remaining time minus overhead. This deliberately conservative policy favors not losing on time over using every available millisecond.

The engine is single-threaded. It accepts but does not advertise unsupported features such as pondering, MultiPV, hash sizing, and Chess960.

## 9. Correctness and performance verification

The repository uses several complementary verification techniques:

- **Perft:** recursively counts legal leaf nodes at an exact depth. It is sensitive to subtle errors in castling, en passant, promotion, pins, and check handling.
- **Perft divide:** reports the count under each root move, making it possible to localize a rules error to one branch.
- **Stockfish differential testing:** compares legal move counts and perft results with an independent engine, including positions from real games.
- **Reference attack comparisons:** validates magic rook and bishop lookups against the slower ray-walking implementation.
- **Search tests:** cover mate, stalemate, material gain, deterministic results, non-mutation, cancellation, root filtering, and comparison with a simple full-width reference search.
- **Repetition and draw tests:** exercise key maintenance, history-dependent repetition, move-clock draws, insufficient material, and checkmate precedence.
- **Protocol and end-to-end tests:** run UCI sessions and matches, including embedded-data operation outside the repository directory.

Benchmarks cover attack lookup, parsing, perft, evaluation, and search. The deterministic UCI benchmark is useful for detecting changes in searched node counts, while perft is the primary move-generation benchmark.

## 10. Overall assessment and natural next steps

The codebase has a sound baseline architecture: compact board state, fast attack lookup, rules-oriented legal move generation, predictable static evaluation, reliable position history, and a cancellable alpha-beta search. Copy-make and full legal-move filtering are especially useful for correctness and maintainability.

The largest strength-related gaps, in a practical order, are:

1. **Quiescence search**, to resolve tactical captures before applying static evaluation.
2. **A transposition table**, to reuse results and improve ordering across iterative-deepening iterations and transpositions.
3. **Stronger move ordering**, such as MVV-LVA, killer moves, and history heuristics.
4. **A richer, game-phase-aware evaluation**, especially for pawn structure and endgame king activity.
5. **More efficient legal move generation or make/unmake**, if profiling shows that copying and full king-safety filtering dominate after the search improvements above.

Each change should be validated against the existing perft, rules, determinism, and UCI tests. Search changes that preserve exact fixed-depth semantics can also be checked for unchanged scores while expecting different node counts; strength changes should be measured with repeated paired games rather than inferred from node speed alone.
