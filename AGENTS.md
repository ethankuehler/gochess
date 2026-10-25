# AGENTS.md

## Project overview

`gochess` is a personal, work-in-progress chess engine written in Go. The author
is using it to learn Go; per the README it is "not close to done". The codebase
covers board representation, FEN handling, attack-table generation/lookup, legal
move generation, perft verification, a hand-written evaluation, iterative-
deepening alpha-beta search, and a UCI front end.

- Module: `github.com/ethankuehler/gochess` (see `go.mod`, `go 1.23.2`)
- Dependencies: Go standard library only. There is no `go.sum`.
- Packages:
  - `main` (repo root, `main.go`) — a scratch/demo binary that builds attack
    tables, parses a FEN, and prints boards. Not a real application yet.
  - `chess` (`chess/`) — the actual library. All engine logic lives here.
  - `uci` (`uci/`) — the UCI protocol layer: command loop, position/go parsing,
    time management, info formatting, options, and bench. It imports `chess`
    and `data`; `chess` does not import it.
  - `data` (`data/`) — embeds the eight runtime CSVs with `//go:embed` so the
    UCI binary is self-contained. The CSVs remain the canonical source data.
  - `cmd/uci` (`cmd/uci/`) — the UCI executable. Thin wrapper around `uci.Run`
    with `bench` and `-version` modes.
  - `cmd/genmagics` (`cmd/genmagics/`) — CLI that regenerates the magic-number
    CSVs in `data/`.
  - `cmd/perft` (`cmd/perft/`) — perft runner, divide printer, fixture suite,
    and optional Stockfish comparison.
  - `cmd/match` (`cmd/match/`) — plays gochess against Stockfish, validates
    every position with Stockfish perft, adjudicates all draw rules, and checks
    that gochess's search recognizes drawn and terminal positions.
  - `cmd/search` (`cmd/search/`) — fixed-depth search CLI; prints the best
    move, score, nodes, time, and nps, or just the static evaluation.
  - `cmd/web` (`cmd/web/`) — local browser board; human White, engine Black,
    plus auto play where the engine plays both sides. Engine moves are
    requested through an in-process `uci.Client` using `position`/`go depth`;
    also click-to-move, undo, reset, and PGN export.

## Repository layout

- `main.go` — demo entry point (`package main`).
- `cmd/genmagics/main.go` — regenerates `data/rook_magics.csv` and
  `data/bishop_magics.csv`; run from the repo root.
- `cmd/perft/main.go` — runs perft, lists legal moves, prints a divide, runs
  `data/perft.csv`, and can diff a divide against Stockfish; run from the repo
  root.
- `cmd/search/main.go` — parses a FEN, optionally applies `-moves` (which also
  supplies repetition history), builds the attack tables, and runs a
  fixed-depth search; `-eval` prints the static score instead. Run from the repo
  root so the `data/` paths resolve.
- `cmd/match/main.go` — match orchestration, game loop, draw/terminal
  adjudication, and gochess draw-awareness checks. `cmd/match/stockfish.go` is
  a persistent external UCI client used as both opponent and perft oracle. It
  uses the embedded attack tables, so it does not depend on the working
  directory.
- `cmd/arena/main.go` — rating harness. Plays gochess against Stockfish with
  fixed-time or fixed-node limits, a paired opening book (`data/openings.csv`),
  draw adjudication, JSON output, and a bootstrap Elo estimate over opening
  pairs. It runs games sequentially and reads `data/openings.csv` from disk, so
  run it from the repo root.
- `cmd/uci/main.go` — the UCI executable. `go build -o gochess ./cmd/uci`
  produces a self-contained binary; `gochess bench [depth]` runs the benchmark
  and exits, and `gochess -version` prints the version. Normal startup runs the
  UCI command loop on stdin/stdout.
- `cmd/web/main.go`, `cmd/web/analysis.go`, `cmd/web/cpu_unix.go`,
  `cmd/web/cpu_other.go`, and `cmd/web/index.html` — HTTP
  server, optional runtime analysis, and embedded board UI. The server owns one
  in-memory game and exposes `/api/state`, `/api/pgn`, `/api/move`,
  `/api/claim-draw`, `/api/engine`, `/api/engine/move`, `/api/reset`, and
  `/api/undo`. `/api/pgn` downloads the game as PGN with the Seven Tag Roster,
  SAN movetext, and the current result. Threefold repetition and the fifty-move
  rule are claimable: they are advertised through
  `threefoldClaim`/`fiftyMoveClaim` and do not end the game until
  `POST /api/claim-draw`. Auto play calls that endpoint automatically, because
  neither engine side can press the button; human play uses the **Claim draw**
  button. Fivefold repetition, the seventy-five-move rule, and insufficient
  material end the game automatically. Engine mode plays Black against a human
  White; auto play asks the engine to move for both sides. Each engine move sends
  `position fen ... moves ...` and `go depth N` to an in-process `uci.Client`
  and applies the returned `bestmove`. The panel has an engine toggle and a
  numeric depth input with no upper limit. `newGame` starts the client;
  `game.Close` shuts it down, and tests must close the game. With `-analysis`,
  `newGameWithAnalysis` also enables UCI metrics when built with `-tags metrics`,
  samples runtime data, and registers `/api/analysis` plus `/debug/pprof/`;
  ordinary mode registers none of those routes. The analysis service retains 300 runtime samples and 64
  searches, and `game.Close` stops its sampler. Process CPU time comes from
  `getrusage` on Unix (the runtime CPU-class metrics are zero on macOS) and from
  `/cpu/classes` elsewhere; samples mark CPU unavailable if neither source
  works.
- `uci/` — the UCI protocol layer (`package uci`):
  - `engine.go` — `Engine`, `Run`, command loop, handshake, `isready`,
    `setoption`, `ucinewgame`, `stop`, `quit`, EOF handling, and the
    command-loop/search-goroutine state split.
  - `position.go` — `position startpos|fen ... moves ...`, 4/5-field FEN
    padding, and move application through `Position.ApplyUCIMove`.
  - `go.go` — `go` parsing, `searchmoves`, root filtering, and conservative
    clock allocation (`allocateBudget`, `Move Overhead`).
  - `search.go` — search goroutine, info formatting, final info, `bestmove`,
    infinite-mode waiting, and panic recovery.
  - `client.go` — GUI-side `Client` used by `cmd/web` and tests:
    `StartInProcess`, `SetPosition`, `GoDepth`, `NewGame`, and `Close`. A
    dedicated reader goroutine drains engine output so a command write cannot
    deadlock against an `info string` response.
  - `options.go` — the advertised options (`Threads` fixed at 1 and
    `Move Overhead`; `Analysis` only in a `metrics` build) plus tolerant
    `setoption` parsing for ignored options.
  - `metrics_on.go`, `metrics_off.go` — compile-time `metricsSupported` flag.
  - `output.go` — mutex-protected, flushed stdout writer.
  - `bench.go` — fixed-position deterministic benchmark.
  - `version.go` — engine name, author, and `Version` (overridable with
    `-ldflags -X`).
- `chess/` — the engine library (`package chess`):
  - `bitboard.go` — `BitBoard` (`uint64`) and `Position` types, validated FEN
    parse/serialize (`ParseFEN`, `FEN()`), board printing (`String`,
    `UnicodeString`), `Occupancy`, `PieceBitboard`.
  - `const.go` — `Square`, `FileRank`, `Piece`, and `Colour` enums, position
    flag bits, named squares, file masks, and piece/board notation tables.
  - `move.go` — `Move` (two `uint8` squares plus a `uint16` flag word),
    `ParseUCIMove`, the
    `From`/`To`/`Promotion` accessors, move-flag accessors
    (`IsCapture`, `IsEnPassant`, `IsCastling`, `IsDoublePush`), and
    algebraic-notation helpers (`SquareName`, `ParseFileRank`, `ParseSquare`,
    `BitBoardFromAlgebraic`).
  - `ucimove.go` — `FindLegalMove` (matches notation-only moves by
    from/to/promotion and returns the flagged generated move) and
    `ApplyUCIMove` (parse, match, and apply in one step). Use these for any
    UCI/FEN move list; `ParseUCIMove` alone is not enough for `ApplyMove`.
  - `pgn.go` — Standard Algebraic Notation (`Position.SAN`) and PGN export
    (`PGNHeader`, `PGN`) with castling, promotions, file/rank/square
    disambiguation, check/checkmate suffixes, tag escaping, and 80-column
    movetext wrapping.
  - `magic.go` — precomputed attack tables (knight, king, pawn) loaded from
    CSV, `SlidingAttacks` for sliding pieces, magic-bitboard generation/lookup
    for rooks and bishops (`MagicEntry`, `findMagic`, `tryMagic`,
    `RelevantBlockers`, `RookAttack`, `BishopAttack`, `QueenAttack`), and
    `BuildAllAttacksFromFS` for the embedded UCI tables.
  - `csv.go` — CSV plumbing (`readCSV`, `readCSVFS`, `readRecord`) and
    attack-table loading (`loadAttackTable`, `loadAttackTableFS`).
  - `magic_io.go` — magic-number CSV load/save (`LoadMagicEntries`,
    `LoadMagicEntriesFS`, `SaveMagicEntries`).
  - `movegen.go` — attack detection (`isSquareAttacked`), pseudo-legal
    generation, castling checks, king-safety filtering, `LegalMoves`, and
    `InCheck`.
  - `movegen_metrics.go` — instrumented legal generation behind
    `//go:build metrics`.
  - `makemove.go` — copy-make `makeMove`, castling-rights updates, en passant,
    promotions, clocks, and the incremental Zobrist key.
  - `repetition.go` — `PositionKey`, Zobrist keys, `Position.Key`, and the
    fifty-move, seventy-five-move, threefold/fivefold-repetition, and
    insufficient-material helpers.
  - `perft.go` — `Perft`, `PerftDivide`, and `PerftResult`.
  - `eval.go` — material values and the precomputed piece-square table
    (centrality, king safety, pawn advancement), plus `Position.Evaluate`.
  - `search.go` — `MateScore`, `SearchResult`, `SearchInfo`, `SearchLimits`,
    fixed-depth `Search`, iterative-deepening `SearchWithLimits`, capture-first
    move ordering, PV tracking, seldepth, cancellation/deadline/node limits,
    and `ScoreToMate`.
  - `search_metrics_on.go` / `search_metrics_off.go` — compile-time metrics
    wrappers; the off build calls the position methods directly. `MetricsEnabled`
    reports which build is active.
  - `bench_test.go` — benchmarks for attack lookups, parsers, perft,
    evaluation, and search.
- `data/` — generated/test data:
  - `assets.go` — `package data` with `//go:embed` of the eight runtime CSVs.
    The UCI binary calls `BuildAllAttacksFromFS(data.FS)` and therefore does not
    depend on the working directory.
  - `knight_attacks.csv`, `king_attacks.csv`, `white/black_pawn_move.csv`,
    `white/black_pawn_attacks.csv` — attack tables (columns: index, start
    square bit, move bitboard), generated by `generate_attack_tables.py`.
  - `rook_magics.csv`, `bishop_magics.csv` — magic numbers for the sliding
    pieces (columns: square, mask, magic), written by `SaveMagicEntries`
    (not by `generate_attack_tables.py`).
  - `FEN.csv` — FEN corpus used as test fixtures.
  - `sliding_attacks_examples.json` — JSON fixture for sliding attacks.
  - `perft.csv` — Stockfish-verified perft fixtures (columns: fen, depth,
    nodes, deep). Rows with `deep=1` run only with `PERFT_DEEP=1`.
  - `cmd/search`, `cmd/perft`, `cmd/genmagics`, and the package tests still
    read these files from disk via relative `data/...` paths and must run from
    the repo root. `cmd/uci`, `cmd/web`, and `cmd/match` initialize from the
    embedded FS, so they do not depend on the working directory.
- `generate_attack_tables.py` — Python 3 script that regenerates the CSV
  attack tables in `data/`. Requires `pandas`. Run from the repo root
  (`python3 generate_attack_tables.py`); it writes to `data/*.csv`.
- `tools/lichess/` — optional Lichess Bot API integration. `setup.sh` clones the
  third-party `lichess-bot` bridge into `tools/lichess/lichess-bot/`, creates a
  Python virtualenv, installs its dependencies, builds `./gochess`, and validates
  `config.yml`. `run.sh` loads the gitignored `tools/lichess/.env` (or an
  exported token), optionally upgrades the account to a bot, and then runs the
  bridge. The bridge plays rated standard games against other bots and accepts
  casual human challenges, reserving one of two slots for humans; no token is
  stored in the repo, and neither script is part of the Go build, tests, or CI.
  The generated `lichess-bot/`, `venv/`, `games/`, `.env`, and logs are
  gitignored.
- `.github/workflows/ci.yml` — GitHub Actions CI (format, build, vet, race
  tests).
- `.vscode/launch.json` — VS Code debug config for `main.go`.

## Build and test commands

```sh
go build ./...            # build everything
go build -o gochess ./cmd/uci  # self-contained UCI engine, no search metrics
go build -tags metrics -o gochess-metrics ./cmd/uci  # instrumented build
go run .                  # run the demo binary (must run from repo root)
go run ./cmd/uci          # run the UCI engine on stdin/stdout
go run ./cmd/web          # local board UI; Engine uses the in-process UCI client
go run ./cmd/web -analysis # opt-in runtime plots and pprof (loopback only)
go run ./cmd/genmagics    # regenerate data/*_magics.csv (from repo root)
go run ./cmd/perft -depth 5
go run ./cmd/perft -moves -fen "<FEN>"
go run ./cmd/perft -depth 4 -divide
go run ./cmd/perft -depth 4 -compare -fen "<FEN>"
go run ./cmd/perft -suite data/perft.csv
go run ./cmd/match -stockfish stockfish -games 4 -depth 4
go run ./cmd/arena -pairs 20 -movetime 250ms -sf-elo 1320
go run ./cmd/arena -check-book
go run ./cmd/search -depth 4
go run ./cmd/search -fen "<FEN>" -depth 5
go run ./cmd/search -fen "<FEN>" -moves "e2e4 e7e5" -depth 5
go run ./cmd/search -fen "<FEN>" -eval
./gochess bench           # deterministic bench; prints "<nodes> nodes <nps> nps"
./gochess bench 6         # optional depth
./gochess -version
go test ./...             # default build; metrics tests skip
go test -tags metrics ./... # run the metrics-enabled tests
go test -race ./...       # run tests under the race detector
go test -tags metrics -race ./... # race tests with metrics enabled
PERFT_DEEP=1 go test ./chess -run '^TestPerft$' -v
STOCKFISH_BIN=stockfish go test ./chess -run TestPerftAgainstStockfish
STOCKFISH_BIN=stockfish go test ./cmd/match -run TestMatchAgainstStockfish
go test -bench . ./chess  # run benchmarks
go vet ./...              # static checks
```

Working directory matters for `cmd/search`, `cmd/perft`, and `cmd/genmagics`:
the `chess` package's disk `Build*` functions and the tests open `data/...` via
relative paths. Run those binaries and tests from the repo root. The test suite
handles this with a `TestMain` in `chess/bitboard_test.go` that `os.Chdir("..")`s
to the repo root — preserve that pattern if you add tests that touch `data/`.

`cmd/uci`, `cmd/web`, and `cmd/match` are different: they initialize from
`BuildAllAttacksFromFS(data.FS)`, so they do not read `data/` at runtime.
`cmd/uci` is tested from an empty temporary directory. If you add a runtime CSV,
add it to the `//go:embed` list in `data/assets.go` and to the FS loader.

CI runs on pushes to `main` and on pull requests: `gofmt` check, `go build`,
`go vet`, `go test -race ./...`, `go test -tags metrics -race ./...`, the
Stockfish perft differential test, and a short Stockfish match test. The compiled `gochess` binary, `gochess-uci`,
`.vscode/`, the local Go caches (`.gocache/`, `.gomodcache/`, `.gopath/`), and
local planning notes in `plans/` are gitignored.

## Core conventions and architecture

Bitboard representation:

- `BitBoard` is a `uint64`; square mapping is little-endian rank-file:
  bit 0 = a1, bit = `file + rank*8`, bit 63 = h8.
- `Square` is the canonical square index. `FileRank` holds zero-based
  `file`/`rank` coordinates; convert with `SquareFromFileRank` /
  `FileRankFromSquare`. Use named fields in `FileRank` literals.
- `Position.pieceBitboards` is a `[12]BitBoard` array: indices 0–5 are white
  `Pawn`, `Knight`, `Bishop`, `Rook`, `Queen`, `King`; black pieces are the
  same order offset by `blackPieceOffset` (6).
- `Position.flags` (uint8) packs side-to-move (`whiteToMove`) and the four
  castling rights (`whiteKingsideCastling`, `whiteQueensideCastling`,
  `blackKingsideCastling`, `blackQueensideCastling`). En passant is a
  single-square `BitBoard` (`enPassantSquare`), not an index.
- `Position.occupancy` caches white and black occupancy, and `Position.epHash`
  caches the en passant contribution to the Zobrist key. Constructors call
  `refreshOccupancy` and `computeEnPassantKey`; `makeMove` updates both
  incrementally.
- Piece attack tables are `[]BitBoard` of length 64 indexed by `Square`,
  populated from the CSVs by `BuildKnightAttacks` / `BuildKingAttacks` /
  `BuildPawnMoves` / `BuildPawnAttacks` (or all at once via
  `BuildAllAttacks`). Sliding-piece (rook/bishop) magic tables are
  restored from `data/*_magics.csv` by default (`LoadMagicsFromDisk`),
  rebuilt with `tryMagic` to validate the stored magics. If a CSV is
  missing or invalid, the builder logs a message and falls back to a random
  magic search; setting `SaveMagicsToDisk` rewrites the CSVs after a search
  (normally done via `go run ./cmd/genmagics`). Queens reuse the rook and
  bishop lookups via `QueenAttack`.
- `Move` is a 4-byte struct: two `uint8` square indices (0–63) plus a `uint16`
  flag word. `From`/`To` return `Square`; the internal
  `newMove(from, to Square, flags uint16)` is the only constructor and the only
  place that narrows. The low three bits hold the promotion `Piece`; bit 3 is
  `promotionFlag`, bit 4 `captureFlag`, bit 5 `doublePushFlag`, bit 6
  `enPassantFlag`, and bit 7 `castlingFlag`; bits 8–15 are reserved. Generated
  moves carry the flags; `ParseUCIMove` is notation-only. Use
  `Position.FindLegalMove` or `Position.ApplyUCIMove` to turn notation into the
  flagged generated move before calling `ApplyMove`/`makeMove`.
- `makeMove` is copy-make and returns a new `Position`. It clears castling
  rights for any move from or to e1/a1/h1/e8/a8/h8, clears the en passant
  square unless the move is a double push, updates the halfmove/fullmove
  clocks, moves the rook when castling, and updates the incremental Zobrist
  key plus the occupancy and en passant caches. `Position.key` is the
  repetition key: piece placement, side to move, castling rights, and a
  capturable en passant target, but not the clocks. `makeMoveForLegality`
  performs only the board and occupancy updates needed for king-safety
  filtering; its key, clocks, rights, and en passant target are stale, so it
  must not be used as a game position.
- `LegalMoves` generates pseudo-legal moves, applies each one with
  `makeMoveForLegality`, and rejects moves that leave the mover's king attacked.
  Castling additionally checks that the king is not in check and that its
  transit squares are safe with the king removed from its origin.
- `Perft` and `PerftDivide` require `BuildAllAttacks`; `TestMain` builds the
  tables once, and `cmd/perft` builds them at startup. `Perft(0)` is 1.
- `Position.ApplyMove` validates a generated `Move` and returns a new
  `Position`; `Position.PieceAt` and `Position.SideToMove` expose read-only
  state for UIs such as `cmd/web`.
- Pawn attack detection in `isSquareAttacked` uses bit shifts with file masks,
  not the reverse attack tables. The tables are zero on ranks where a pawn of
  that colour cannot exist (for example black pawns on rank 8), but those
  entries are needed to find attackers of back-rank squares.

Evaluation and search:

- `Position.Evaluate` returns centipawns from **White's** point of view and
  needs no attack tables. It sums a precomputed `[12][64]int`
  (`pieceSquareValues`) containing material, centrality, king safety, and pawn
  advancement. Black entries are the negated value on the rank-mirrored square
  (`square ^ 56`).
- `SearchResult.Score` is from the **side to move's** point of view, the negamax
  convention. `evaluateForSideToMove` does the sign flip for Black; forgetting
  it is the classic evaluation bug.
- `Position.Search(depth)` is the fixed-depth entry point. Depth is in plies and
  must be at least 1. It delegates to `SearchWithLimits` with `MaxDepth=depth` and
  no deadline, so existing callers keep the same scores and determinism while
  gaining iterative deepening internally.
- `Position.SearchWithLimits(ctx, limits, info)` performs iterative-deepening
  negamax alpha-beta. `SearchLimits` has `MaxDepth` (0 means no explicit cap, up
  to `maxPly`), `MaxNodes`, `Deadline` (hard wall clock), `SoftDeadline` (stop
  between iterations), `RootMoves` (nil means all legal moves; a non-nil empty
  list means no root moves), `GameHistory` (position keys before the root,
  oldest first), and `CollectMetrics` (default false). It checks cancellation
  and the hard deadline every 256 nodes and at every root move, so `stop` stays
  responsive.
- Optional `SearchResult.Metrics` contains per-search counters and disjoint,
  inclusive wall-clock timings for pseudo-legal generation, legality filtering
  (copy-make plus attack test), evaluation, and search-edge copy-make.
  `MakeMoveCalls` counts every copy-make, including the copies legal generation
  makes to filter moves; `SearchMakeMoveCalls` is the search-tree subset.
  Metrics are compiled in only with `-tags metrics`; `MetricsEnabled` reports
  which build is active. The default build has no timer, counter, or wrapper
  dispatch overhead: `CollectMetrics` is ignored and `SearchResult.Metrics` is
  nil. In a metrics build, `CollectMetrics` false takes a nil-check wrapper path
  and still costs a few percent. Draw checks, check detection, move ordering, PV
  maintenance, timer overhead, and the info callback are not timed, so the phase
  sum is a lower bound; `ElapsedNS` minus the phases is the unmeasured remainder.
  Metrics add roughly 20–60% overhead depending on the position and search size,
  so use the default build for strength and NPS comparisons.
- `SearchWithLimits` returns the last completed iteration's `BestMove`, `Score`,
  `PV`, and `Depth`. If it stops before any iteration completes it returns the
  first legal move with `Depth == 0`; a terminal position returns
  `HasMove == false`. `Draw` is true when the root itself is a fifty-move,
  threefold, or insufficient-material draw; a legal move is still returned.
  `Nodes` is cumulative across iterations, `SelDepth` is the deepest ply
  reached, and `PonderMove` is the second PV move when present. The `info`
  callback runs once per completed iteration.
- UCI `searchmoves` is resolved to generated moves and passed as `RootMoves`.
  UCI clocks become `Deadline`/`SoftDeadline` through `uci.allocateBudget`;
  `Move Overhead` is reserved before the deadline is computed.
- Checkmate is `-MateScore + ply`; the ply offset makes the search prefer
  shorter mates. Stalemate is 0. `ScoreToMate` converts a mate score into a
  distance in moves. Mate scores are always far above ordinary evaluations.
- The fifty-move rule is checked at node entry and at the root (`halfmoveClock
  >= 100`). Threefold repetition is checked against the last `halfmoveClock`
  positions of the game history plus the current search path; a third
  occurrence scores 0. `Position.IsInsufficientMaterial` covers K vs K, a lone
  minor, and bishops confined to one colour complex. Callers pass prior keys
  through `SearchLimits.GameHistory`; the draw helpers live in `repetition.go`.
  A drawn root returns a legal move with `Draw == true` and score 0, because
  UCI still requires a `bestmove`. Checkmate takes precedence over a draw.
- At depth 0 the search still tests for checkmate when the side to move is in
  check, so a depth-1 search finds mate in one. Stalemate exactly at the horizon
  is not detected.
- `cmd/search` builds the attack tables itself. Run it from the repo root so
  the `data/` paths resolve.
- `cmd/web` owns engine mode: `engineEnabled`, `engineDepth` (default 4, no
  upper cap), `engineBusy`, the `uci.Client`, and the last `engineInfo`. The
  engine normally plays Black; `POST /api/engine` updates settings,
  `POST /api/engine/move` asks the client for one move for the side to move
  (auto play uses it for both colours), and `/api/move` rejects a human move
  when it is the engine's turn. The search runs without the game mutex, and
  reset replaces the client so an in-flight search is cancelled and its result
  discarded. Undo removes the engine's reply as well. `GET /api/state` includes
  the engine fields so the page can drive the reply loop. Each engine move
  sends the initial FEN plus the full move list when it matches the stored
  positions, so the engine can see repetition history, and applies the returned
  move with `ApplyUCIMove`. `GET /api/state` reports `threefoldClaim` and
  `fiftyMoveClaim` when a draw is claimable; those draws leave `status` as
  `playing` until `POST /api/claim-draw` records the claim. The page's auto
  play loop calls the claim endpoint as soon as a claim is available; human
  play uses the **Claim draw** button. Fivefold repetition, the
  seventy-five-move rule, and insufficient material are terminal automatically.
  `GET /api/pgn` replays the stored moves through `chess.PGN` and returns an
  attachment whose result is derived from the current status. Close the game in
  tests and on HTTP shutdown to stop the engine goroutines and optional analysis
  sampler. Analysis mode records only completed, non-stale client results;
  reset clears search history but retains process samples. `GET /api/state`
  exposes only the `analysis` discovery boolean, while `GET /api/analysis`
  returns bounded runtime/search snapshots. The collapsed browser panel polls
  only while open and visible and renders independently of the board. CPU/trace
  captures are bounded to 1–60 seconds and analysis requires a loopback address.

UCI protocol:

- `uci.Run` is the engine side; `uci.Client` is the GUI side for the web demo and
  tests. `StartInProcess` runs the engine over `io.Pipe`, completes the
  handshake, and starts a reader goroutine. `Client.SetPosition` takes a FEN
  plus optional moves, waits for `readyok`, and surfaces
  `info string position error`; `Client.GoDepth` reads
  info lines until `bestmove`; `Client.NewGame` sends `ucinewgame`; `Close`
  sends `quit`, closes the pipes, and waits for the engine goroutine. Commands
  are not safe to issue concurrently, but `Close` may run during `GoDepth` to
  cancel a search; the web demo replaces the client on reset to do exactly that.
- `uci.Run` owns stdin/stdout and never writes protocol text anywhere else.
  `lineWriter` serializes and flushes stdout because search info and `readyok`
  can come from different goroutines; diagnostics go to a locked stderr writer.
- The command loop and the search goroutine split responsibilities. The loop
  owns position/options and handles `isready`, `stop`, `quit`, and EOF while a
  search is running; the search goroutine owns the search and is the only writer
  of `bestmove`.
- `isready` is answered immediately, including during a search. `stop` cancels
  the search context; `quit` cancels and waits for the search to finish. EOF is
  treated like quit, so the process exits cleanly when its stdin closes.
- `position` is accepted only while idle. It supports `startpos` and `fen`, pads
  4/5-field FENs, and applies moves with `ApplyUCIMove`. It records the key of
  every position before the final one as game history, which `go` passes to
  `SearchLimits.GameHistory`; a bare `position fen` has no history. A parse or
  move failure invalidates the position; a later `go` replies
  `info string no valid position` and `bestmove 0000` rather than searching a
  stale board.
- `go` supports `depth`, `nodes`, `movetime`, `wtime`/`btime`/`winc`/`binc`/
  `movestogo`, `infinite`, `ponder` (treated as infinite; `ponderhit` acts as stop because
  pondering is not implemented), and `searchmoves`.
  A bare `go` is treated as infinite. `infinite` waits for `stop` after the
  search result is ready; terminal positions send `bestmove 0000` immediately.
- Checkmate and stalemate use `bestmove 0000`; checkmate is `info score mate 0`
  and stalemate is `info score cp 0`. Scores are side-to-move relative, and mate
  distances are in moves via `ScoreToMate`.
- `Threads` (1..1) and `Move Overhead` are always advertised. A metrics build
  also advertises `Analysis` (check, default false); it adds one compact
  `info string analysis <json>` before the final `bestmove`, and the client
  retains per-iteration depth/seldepth/nodes/time/NPS and the final metrics. `Hash`, `Clear Hash`, `Ponder`, `MultiPV`, and
  `UCI_Chess960` are accepted/ignored where harmless but are not advertised
  because they are not implemented. There is no transposition table, pondering,
  MultiPV, Chess960, tablebase, or opening book support.
- `bench` uses a fixed position list, fixed depth, one thread, and no clock; its
  final stdout line is `<nodes> nodes <nps> nps`. `./gochess bench` exits after
  printing; the UCI `bench` command returns to the command loop.

## Naming conventions

- Go style is mixedCaps. Unexported fields and locals are camelCase
  (`halfmoveClock`, `enPassantSquare`).
- `Square` means a 0–63 square index; `FileRank` means file/rank coordinates;
  `Position` means the full game state; `Directions` means a set of movement
  vectors.
- Avoid `Get` prefixes: use `Occupancy`, `PieceBitboard`, `RookAttack`, etc.
  Parsers are named `Parse*`; formatting helpers end in `Name` (`SquareName`).
- `Colour` uses British spelling throughout; keep it for consistency.

## Go style and error handling

- Uses Go 1.22+ features: `iter.Seq` iterators (`PieceTypes`) and
  `math/rand/v2`. Keep compatibility with the `go 1.23.2` directive.
- Error handling is plain `errors.New` / `fmt.Errorf`; the disk
  `loadAttackTable` helper uses `log.Fatal`, while `loadAttackTableFS`,
  `LoadMagicEntriesFS`, and `BuildAllAttacksFromFS` return errors.
  `PieceBitboard` panics on invalid input. Match the surrounding style when
  editing nearby code.
- `LoadMagicEntries`, `LoadMagicEntriesFS`, and `SaveMagicEntries` return
  errors. Both CSV loaders validate row count, column count, square range, and
  duplicate squares. `BuildAllAttacksFromFS` loads every table into locals
  before assigning the package globals, so a bad embedded file leaves the
  previous tables intact.
- The UCI layer must never call `log.Fatal` or write non-protocol text to
  stdout. Initialization failures become `info string` plus `readyok` or
  `bestmove 0000`, not a process exit. Diagnostics go to stderr.

Things to be careful with:

- `pieceSymbols` follows the standard `Piece` constant order (white Pawn,
  Knight, Bishop, Rook, Queen, King, then the black pieces). That order is
  load-bearing across FEN parsing, board printing, and promotion encoding;
  don't reorder it without updating every table.
- `Move.from`/`Move.to` are `uint8`; never assign a `Square` directly. Build
  moves with `newMove`, or convert explicitly after checking the 0–63 range.
- `tryMagic` uses a zero table slot as "not yet written"; that sentinel is
  safe only because rook/bishop attack sets are never empty. Keep that
  invariant in mind before reusing the pattern elsewhere.
- `ParseFEN` is strict: it requires 8 ranks of 8 squares, exactly one king
  per side, at most 16 pieces and 8 pawns per side, no pawns on the back
  ranks, a fullmove number of at least 1, and an en passant square on rank 3
  or 6. `NewStartingPosition` uses the standard start position (`... 0 1`).
- `LoadMagicsFromDisk` and `SaveMagicsToDisk` are package-level mutable
  flags. Tests set and restore them; treat the attack tables as init-time
  state and don't mutate them concurrently.
- The UCI engine builds the tables from `data.FS`, not from the working
  directory. `BuildAllAttacksFromFS` deliberately does not fall back to a
  random magic search: a bad embedded file should be a startup error, not an
  unpredictable delay.
- `SearchWithLimits` counts nodes at `negamax` entry, excluding the root, and
  polls cancellation/deadlines every 256 nodes. If you change the polling
  interval, keep `stop` responsive at low time controls. `SearchResult.Nodes`
  is cumulative across iterative-deepening iterations.
- The UCI search goroutine marks the session idle with `finishSearch` before it
  writes the final `bestmove`. Do not move that call after the write: a GUI may
  send `position`/`go` immediately after seeing `bestmove`, and the command loop
  must already accept it. `finishSearch` must be called exactly once; the
  `finished` guard in `searchRoutine` enforces that.
- `bestmove` is written exactly once by the search goroutine. `stop` only
  cancels; it never writes a move. `quit`/EOF set `quitting`, cancel, and wait
  for the search goroutine, which suppresses the final `bestmove`.
- `go infinite` can reach a depth/node limit before `stop`. The search
  goroutine waits for `ctx.Done()` before reporting in that mode; terminal
  positions are the exception because there is nothing to search.
- `generate_attack_tables.py` is a one-off generator script, not part of the
  Go build; the CSVs it writes are committed and treated as source data.

## Testing instructions

- Tests live in `chess/*_test.go` using the standard `testing` package;
  run with `go test ./...`.
- Tests are data-driven: FEN round-trip tests read `data/FEN.csv`, and
  sliding-attack tests read `data/sliding_attacks_examples.json`. Add new
  cases by extending those files rather than hardcoding positions, where
  practical.
- Current suite covers FEN parse/serialize round-trips, rejection of
  malformed FENs (placement, kings, pawns, clocks, castling, en passant),
  the starting position, `Occupancy`, `PieceBitboard`, `SquaresToBitBoard`,
  `Square`/`FileRank` parsing, UCI move round-trips (including promotions and
  invalid moves), pawn table loading, `SlidingAttacks` (fixtures plus an
  exhaustive reference comparison), `RelevantBlockers` popcounts, magic
  rook/bishop/queen lookups checked against `SlidingAttacks` for all squares
  with random blocker boards, a magic save/load round-trip, disk restore by
  default (`LoadMagicsFromDisk`), the missing-CSV fallback path,
  `BuildAllAttacks`, and rejection of malformed magic CSVs.
- Move-generation coverage includes move flags, make-move FEN transitions,
  attack detection (including back-rank pawn attacks), castling, en passant,
  promotions, pins, double check, legal-move invariants, and parity between the
  instrumented and normal legal-generation paths. Perft is
  data-driven from `data/perft.csv`; the fast rows run by default and the deep
  rows run with `PERFT_DEEP=1`.
- Evaluation coverage includes the starting position (0), exact deltas for
  centrality, king corner, pawn advancement, material removal, a table-mirror
  check, and a colour-symmetry test that flips a FEN vertically and asserts the
  score negates.
- Search coverage includes mate in one, checkmate, stalemate, a free queen, a
  poisoned-pawn horizon case, best-move legality, determinism, non-mutation, and
  a score comparison against a full-width reference minimax at depths 1-3.
- `chess/ucimove_test.go` covers double pushes, captures, en passant, castling,
  all four promotions, promotion captures, illegal/malformed moves, and
  `FindLegalMove` with flagless notation. `chess/repetition_test.go` covers key
  equality and incremental maintenance, fifty-move, seventy-five-move,
  threefold/fivefold, and insufficient-material draws at the root,
  repetition/fifty-move detection along the search path, and checkmate
  precedence.
  `chess/data_fs_test.go` covers `BuildAllAttacksFromFS(data.FS)`, missing
  embedded files, and `LoadMagicEntriesFS`. `chess/search_limits_test.go`
  covers iterative deepening, info callbacks, PV legality, soft deadlines, node
  limits, cancellation/fallback, `RootMoves`, terminal positions, optional
  metrics, and non-mutation. `chess/pgn_test.go` covers SAN for pawn moves,
  captures, en passant, castling, promotions, check/checkmate, and every
  disambiguation form, plus PGN tags, defaults, escaping, wrapping,
  black-to-move starts, and errors.
- `uci` tests use in-process `io.Pipe` sessions: handshake, `isready` during a
  search, `position` with FEN/moves, clocks, `stop`, `quit`, EOF, `searchmoves`,
  node limits, terminal `bestmove 0000`, invalid FEN/move handling, options,
  analysis JSON, and stdout purity. Unit tests cover `setoption`, `go`, FEN
  padding, time allocation, and bench determinism.
- `cmd/match` tests cover draw-reason adjudication, perft diff reporting, and
  (when `STOCKFISH_BIN` is set) a short two-game match that exercises the
  external UCI client, per-position perft validation, legal-move checks, and
  draw awareness. CI runs the Stockfish match test after installing Stockfish.
- `cmd/arena` tests cover Elo conversion, the pair bootstrap, score conversion,
  and book loading, validation, and transposition dropping.
- `uci/client_test.go` covers `StartInProcess`, `SetPosition` (including an
  invalid FEN and recovery), `GoDepth`, `GoMovetime`, `GoNodes`, `SetOption`,
  PV legality, terminal positions, `NewGame`, idempotent `Close`, analysis
  iteration retention/deduplication, and `parseInfo` for
  cp/mate/nodes/nps/seldepth/time/pv.
- `cmd/uci` tests build the binary and run it from an empty temporary directory,
  which verifies embedded data and working-directory independence. They also
  exercise `bench` and `-version` as separate processes.
- `cmd/search` tests call `run` with buffers and cover `-eval`, mate/checkmate
  output, `-moves` draw detection, and invalid depth, moves, arguments, and
  FENs. Its `TestMain` changes to the repo root because `BuildAllAttacks` reads
  `data/`.
- `TestPerftAgainstStockfish` compares a deterministic random game with
  Stockfish at every position. It skips unless `STOCKFISH_BIN` is set; CI
  installs Stockfish and runs it in a dedicated step.
- `cmd/web` tests exercise the state, move, claim-draw, reset, undo, and PGN
  endpoints with `httptest`, plus engine settings/depth validation, UCI engine
  replies for either colour, the human-move guard, engine-aware undo, reset
  preserving the engine settings and cancelling a running search, the numeric
  depth input, claimable threefold/fifty-move draws, claiming and continuing
  after a threefold, fivefold/seventy-five-move automatic draws, and
  insufficient-material statuses. The PGN tests cover download headers, SAN
  movetext, unfinished and claimed draw results, and the checkmate result. Each
  test server starts a `uci.Client` and closes it in cleanup. Analysis tests
  cover disabled routes, real search records and reset, bounded runtime/search
  retention, snapshot copying, profile conflicts/duration limits, loopback
  validation, and idempotent sampler shutdown. The web `TestMain` builds the
  attack tables from `data.FS`, so the web tests do not depend on the working
  directory.
- Benchmarks cover `RookAttack`, `BishopAttack`, `ParseUCIMove`, `ParseFEN`,
  `Perft`, `Evaluate`, and `Search` depth 4 with and without search metrics.

## Security considerations

- The Go code has no network code, no external services, no secrets, and no
  third-party Go modules — its attack surface is essentially nil.
- `tools/lichess/` is optional shell/Python tooling, not part of the Go build,
  tests, or CI. It uses the network, installs third-party Python packages, and
  reads `$LICHESS_BOT_TOKEN` or the gitignored `tools/lichess/.env`; neither the
  token nor `.env` may be committed or written into `config.yml`. The
  `lichess-bot` checkout is third-party code; update it deliberately.
- `cmd/perft -compare` and `cmd/match` execute a local Stockfish binary; they
  do not access the network.
- `cmd/web` binds to `127.0.0.1:8080` by default, keeps a single in-memory
  game, and has no authentication. It is a local demo, not a public server.
  Engine depth has no upper limit; a long search is cancelled by resetting the
  game. Engine moves go through an in-process `uci.Client`; no engine binary is
  spawned and no network access is performed. `-analysis` exposes runtime data
  and pprof only on loopback addresses and is off by default; do not register
  those handlers in ordinary mode or on the global default mux.
- `cmd/search` is a local, single-position CLI. It reads only its flags and the
  local `data/` attack tables and performs no network access.
- `cmd/uci` is a local stdin/stdout engine. It performs no network access, has
  no authentication, and trusts the controlling GUI. Commands can request long
  searches (`go infinite`) or large node/depth limits; the GUI is responsible
  for sending `stop`/`quit`. The binary embeds its data, so it does not read
  `data/` at runtime.
- Runtime input is limited to FEN/UCI strings and the local `data/` files;
  the parsers return errors on malformed input and are unit-tested against
  invalid cases, but they are not fuzzed.
  `generate_attack_tables.py` requires `pandas` if you regenerate the CSVs —
  install it in a virtual environment, not system-wide.
