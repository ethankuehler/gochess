# AGENTS.md

## Project overview

`gochess` is a personal, work-in-progress chess engine for learning Go. It
implements bitboards, FEN/PGN handling, legal move generation, perft, evaluation,
iterative-deepening alpha-beta search, UCI, and several testing/match tools.

- Module: `github.com/ethankuehler/gochess`; Go 1.23.2.
- Dependencies: Go standard library only; there is no `go.sum`.
- `chess/`: engine library. It must not import `uci`.
- `uci/`: protocol engine and in-process client; imports `chess` and `data`.
- `data/`: canonical CSV/JSON data plus embedded runtime attack tables.
- `main.go`: scratch/demo binary, not the real application.

Commands under `cmd/`:

- `uci`: self-contained UCI executable, benchmark, and version output.
- `search`: fixed-depth search or static evaluation for one position.
- `perft`: perft, divide, fixtures, and optional Stockfish comparison.
- `genmagics`: regenerate magic-number CSVs.
- `match`: gochess-versus-Stockfish games with perft and draw validation.
- `arena`: paired Stockfish rating games; `versions` compares local builds at a
  fixed depth, time, or node limit and reports W/D/L, Elo, and search metrics.
- `web`: local browser board, human/auto play, undo/reset, and PGN export.

## Build and test commands

Run Go commands and disk-based tools from the repository root.

```sh
go build ./...
go build -o gochess ./cmd/uci
go vet ./...

go test ./...
go test -race ./...
go test -bench . ./chess

go run ./cmd/uci
go run ./cmd/web
go run ./cmd/search -fen "<FEN>" -moves "e2e4 e7e5" -depth 5
go run ./cmd/search -fen "<FEN>" -eval
go run ./cmd/perft -depth 5
go run ./cmd/perft -depth 4 -divide
go run ./cmd/perft -suite data/perft.csv
go run ./cmd/perft -depth 4 -compare -fen "<FEN>"
go run ./cmd/genmagics
go run ./cmd/match -stockfish stockfish -games 4 -depth 4
go run ./cmd/arena -pairs 20 -movetime 250ms -sf-elo 1320
go run ./cmd/arena versions -openings 20 -depth 5 -anchor-elo 1640
go run ./cmd/arena versions -engine ./gochess-new -baseline ./gochess-old -nodes 100000

./gochess bench           # optional depth: ./gochess bench 6
./gochess -version
PERFT_DEEP=1 go test ./chess -run '^TestPerft$' -v
STOCKFISH_BIN=stockfish go test ./chess -run TestPerftAgainstStockfish
STOCKFISH_BIN=stockfish go test ./cmd/match -run TestMatchAgainstStockfish
```

CI checks formatting, build, vet, both race-test variants, Stockfish perft and
match tests, UCI bench/version, and Windows/macOS cross-compilation.

Never commit or push Markdown files other than `README.md` and `AGENTS.md`.
The compiled binaries, `.vscode/`, local Go caches, and `plans/` are gitignored.

## Data and initialization

- `cmd/search`, `cmd/perft`, `cmd/genmagics`, and disk-based tests use relative
  `data/...` paths. `chess/bitboard_test.go` has a `TestMain` that changes to the
  repository root; preserve that pattern for tests that read disk data.
- `cmd/uci`, `cmd/web`, and `cmd/match` use `BuildAllAttacksFromFS(data.FS)` and
  are working-directory independent. `cmd/uci` tests run from an empty directory.
- When adding a runtime CSV, update both `data/assets.go`'s `//go:embed` list and
  the FS loader. FS initialization returns errors and must not fall back to a
  random magic search.
- `generate_attack_tables.py` regenerates the non-magic attack CSVs and requires
  `pandas` in a virtual environment. `cmd/genmagics` writes the magic CSVs.
  Generated CSVs are committed source data.
- Fixtures include `data/FEN.csv`, `data/perft.csv`,
  `data/sliding_attacks_examples.json`, and `data/openings.csv`. Deep perft rows
  require `PERFT_DEEP=1`; Stockfish differential tests require `STOCKFISH_BIN`.
- Attack tables and magic-loading flags are init-time global state. Tests that
  change `LoadMagicsFromDisk` or `SaveMagicsToDisk` must restore them; do not
  mutate tables concurrently.

## Board and move conventions

- `BitBoard` is `uint64`, with bit 0 = a1 and bit = `file + rank*8`. `Square` is
  the canonical 0–63 index; `FileRank` uses zero-based coordinates and named
  fields in literals. Use `SquareFromFileRank`/`FileRankFromSquare` to convert.
- `Position.pieceBitboards` contains white Pawn, Knight, Bishop, Rook, Queen,
  King, then black pieces at offset 6. This order is load-bearing for FEN,
  notation, promotion, and piece tables; do not reorder it casually.
- Position flags pack side to move and castling rights. En passant is a
  single-square bitboard. Occupancy and en-passant hash caches are maintained by
  constructors and `makeMove`.
- Attack tables are indexed by `Square`. Disk builders load stored magics and can
  regenerate missing/invalid ones; queens combine rook and bishop lookups.
  `Perft`, evaluation, and search require initialized attack tables.
- `Move` stores two `uint8` squares and a `uint16` flag word. Construct moves with
  `newMove`; never assign a `Square` directly to its byte fields without a
  checked conversion. Generated moves carry flags, while `ParseUCIMove` is
  notation-only. Use `FindLegalMove` or `ApplyUCIMove` before applying UCI/FEN
  move lists.
- `makeMove` is copy-make: it returns a new position and updates rights, clocks,
  en passant, Zobrist key, and caches. The repetition key excludes clocks and
  includes en passant only when capturable.
- `makeMoveForLegality` leaves key, clocks, rights, and en passant stale; never
  use its result as a game position. `nullMove` is search-only, never legal.
- `LegalMoves` uses checker/pin masks and copy-make validation only for king and
  en-passant candidates. Castling validates transit squares with the king removed
  from its origin. `Perft(0)` is 1.
- Pawn attack detection and SEE derive pawn origins with shifts and file masks.
  Do not substitute reverse attack tables: that drops back-rank recaptures and
  promotion defenders.
- `ParseFEN` is intentionally strict about board shape, kings, piece/pawn counts,
  back-rank pawns, clocks, castling, and en-passant placement.

## Evaluation and search

- `Position.Evaluate` returns centipawns from **White's** perspective.
  `SearchResult.Score` and UCI scores are from the **side to move's** perspective;
  `evaluateForSideToMove` performs the negamax sign flip.
- `Position.Search(depth)` uses plies, requires depth >= 1, and delegates to
  `SearchWithLimits`. The latter performs iterative-deepening PVS with aspiration
  windows, quiescence, move ordering, pruning, draw detection, and optional TT.
- `SearchLimits` supports depth, nodes, hard/soft deadlines, `SearchClock`, root
  moves, game history, and TT. Nil `RootMoves` means all moves; non-nil empty
  means none. A non-nil clock overrides deadline fields. Limits include
  quiescence work.
- Unsound pruning/reductions belong only at null-window scout nodes, not PV
  nodes. Quiescence directly generates tactical moves, searches all evasions in
  check, and detects mate but deliberately not horizon stalemate. It does not
  probe/store TT or extend the PV.
- Search returns the last completed iteration. Before one completes, it returns
  the first legal move at depth 0; terminal positions have `HasMove == false`.
  `Nodes` includes quiescence and all iterations/retries; `SelDepth` includes
  quiescence plies. `PonderMove` is the second PV move when available.
- Checkmate is `-MateScore + ply`; stalemate is 0. `ScoreToMate` reports distance
  in moves. Checkmate takes precedence over draws.
- Fifty-move and repetition draws are checked at the root and through the search,
  including quiescence. Pass prior position keys through `GameHistory`.
  `repeatsEarlier` deliberately scores an early repetition as a draw to avoid
  shuffling and perpetual-check TT errors. A drawn root still returns a legal
  move with `Draw == true` for UCI.
- The transposition table is single-threaded and caller-owned. Nil TT disables
  it. Reuse it only sequentially; UCI resizes/clears it only while idle. Probes
  follow draw checks, PV nodes do not take TT cutoffs, mate scores are normalized
  by ply, and table use is disabled at `halfmoveClock >= 80`.
- Cancellation/deadlines are polled every 256 nodes and at root moves; preserve
  responsive `stop` behavior if changing this. The root is excluded from node
  counts.

## UCI protocol and concurrency

- `uci.Run` is the engine side; `uci.Client` is the GUI side used by web/tests.
  Client commands are sequential, except `Close` may cancel an active search.
  Its reader goroutine must keep draining output to avoid pipe deadlocks.
- Only stdout protocol text is allowed. Serialize/flush stdout writes; send
  diagnostics to stderr. Never call `log.Fatal` in the UCI layer. Initialization
  failures become `info string` plus `readyok` or `bestmove 0000`.
- The command loop owns position/options and handles `isready`, `stop`, `quit`,
  and EOF during search. The search goroutine owns search and is the only writer
  of `bestmove`. `isready` is answered immediately.
- `position` is idle-only, supports startpos/FEN and 4/5-field FEN padding, uses
  `ApplyUCIMove`, and records prior keys as game history. Invalid input clears
  the position; a later `go` must not search a stale board.
- `go` supports depth, nodes, movetime, clocks/increments, movestogo, infinite,
  ponder, and searchmoves. Bare `go` is infinite. When movetime and clocks are
  both present, movetime is a ceiling and clock allocation uses the smaller
  budget, after reserving `Move Overhead`.
- `stop` only cancels and still yields exactly one `bestmove`. `quit`/EOF cancel,
  wait, and suppress the final move. Infinite searches wait for `stop` after a
  result is ready, except terminal positions, which immediately return
  `bestmove 0000`. Checkmate/stalemate also use `bestmove 0000`.
- `go ponder` starts with a `SearchClock` but no deadline. `ponderhit` computes
  and installs the normal budget starting at the hit. Both `ponderhit` and `stop`
  release the final move exactly once; never suppress `bestmove` on ponder stop.
  Include `ponder <reply>` when a ponder move is available.
- Call `finishSearch` exactly once and before writing `bestmove`, so a GUI can
  immediately send another position/go. Do not move it after the write.
- Always-advertised options: Hash (16 MB default, 1–1024), Clear Hash, Threads
  fixed at 1, Move Overhead, and Ponder.
  Hash changes are idle-only. MultiPV/Chess960 may be tolerated but are not
  implemented or advertised; there are no tablebases or opening book in UCI.
- Bench uses fixed positions/depth, one thread, no clock, and clears one TT
  between positions for deterministic nodes. Final stdout is
  `<nodes> nodes <nps> nps`; per-position diagnostics go to stderr.

## Web server

- `cmd/web` owns one in-memory game and an in-process `uci.Client`. Engine moves
  send the initial FEN plus move history and `go depth N`, then apply the returned
  move with `ApplyUCIMove`. Human mode engines play Black; auto play uses both.
- Do not hold the game mutex during search. Reset replaces the client to cancel
  in-flight work and discard stale results. Undo also removes the engine reply.
  Close the game/client in tests and on shutdown.
- Threefold repetition and fifty-move draws are claimable through
  `/api/claim-draw`; they do not automatically end the game. Auto play claims
  automatically. Fivefold, seventy-five-move, and insufficient-material draws
  end automatically. PGN results derive from current game status.

## Style and error handling

- Match surrounding Go style: mixedCaps/camelCase, no `Get` prefixes, `Parse*`
  parsers, `Name` formatting helpers, and British `Colour`.
- `Square`, `FileRank`, and `Position` are distinct concepts; do not use their
  names interchangeably. Go 1.22+ features such as `iter.Seq` and
  `math/rand/v2` are allowed within the Go 1.23.2 directive.
- Use plain `errors.New`/`fmt.Errorf` and existing panic/error conventions.
  FS loaders return errors and validate CSV shape, squares, and duplicates.
  `BuildAllAttacksFromFS` loads into locals before replacing global tables.
- `tryMagic` uses zero as an unwritten slot only because rook/bishop attack sets
  are never empty. Preserve that invariant before reusing the pattern.

## Testing conventions

- Use the standard `testing` package. Extend data-driven fixtures rather than
  hardcoding positions where practical. Add tests only alongside the project's
  existing test suites and run the relevant default and race variants.
- Engine tests cover parsing, attacks/magics, legal generation, make-move, perft,
  evaluation symmetry, SEE, quiescence, search/reference equivalence, draws, TT,
  limits, SAN/PGN, and non-mutation. Preserve deterministic search and bench
  behavior when changing ordering, TT, pruning, or time management.
- UCI tests use in-process pipes and cover protocol state, concurrency, output
  purity, options, limits, terminal positions, pondering, and client cleanup.
- Web tests use `httptest`, close every game/client, and cover engine lifecycle,
  draw claims/automatic draws, reset/undo, and PGN.
- Stockfish tests skip unless `STOCKFISH_BIN` is set. `cmd/arena` uses fake
  engines for deterministic harness tests. Do not weaken reference/perft checks
  to accommodate an engine change without understanding the discrepancy.

## Security and external tools

- The Go engine and CLIs are local tools with no third-party Go modules or
  network service. UCI trusts its controlling GUI; long searches and large hash
  allocations are accepted local inputs. `cmd/web` is an unauthenticated local
  demo bound to `127.0.0.1:8080` by default, not a public server.
- Stockfish comparison/match tools execute a local binary. Python table
  generation installs dependencies only in a virtual environment.
- `tools/lichess/` is optional, third-party shell/Python tooling outside the Go
  build/tests/CI. Tokens come from `$LICHESS_BOT_TOKEN` or gitignored
  `tools/lichess/.env`; never commit them or write them into `config.yml`.
- **Do not deploy the Lichess bot to GitHub Actions.** The disabled workflow must
  not be enabled, re-added, or proposed unless the owner explicitly asks in that
  conversation. Run it on the owner's machine or a VPS; the deploy workflow is
  only an inert template.
