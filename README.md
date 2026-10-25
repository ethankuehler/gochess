# gochess

A personal, work-in-progress chess engine written in Go — an excuse to learn Go
and see how far a garbage-collected language can get with bitboards. It is not
close to done. (Meow. — Ethan)

Go 1.23+ and the standard library only; there is nothing to `go get`.

## Quick start

```sh
go build -o gochess ./cmd/uci   # self-contained UCI engine
go run ./cmd/web                # board demo at http://127.0.0.1:8080
```

The attack tables are embedded in the binary, so `./gochess` runs from any
directory. Drive it directly:

```sh
{ printf 'uci\nisready\nposition startpos\ngo depth 4\n'; sleep 0.2; printf 'quit\n'; } | ./gochess
```

Or point a UCI GUI (Cute Chess, Arena, Banksia, Scid vs PC) at the binary. With
python-chess:

```python
import chess.engine

with chess.engine.SimpleEngine.popen_uci("./gochess") as engine:
    board = chess.Board()
    result = engine.play(board, chess.engine.Limit(time=0.5))
    board.push(result.move)
```

Supported commands: `uci`, `isready`, `debug`, `ucinewgame`, `position`
(`startpos` or `fen`, plus moves), `go` (`depth`, `nodes`, `movetime`,
`infinite`, `searchmoves`, or clock fields `wtime`, `btime`, `winc`, `binc`,
`movestogo`), `stop`, `quit`, and `bench`. It emits `info` lines (`depth`,
`seldepth`, `score`, `nodes`, `nps`, `time`, `pv`) and `bestmove`; checkmate
and stalemate are `bestmove 0000`. Options are `Hash` (default 16 MB, range
1–1024), `Clear Hash`, `Threads` (fixed at 1), and `Move Overhead` (default
50 ms, range 0–5000). A `metrics` build also advertises `Analysis` (default
false; adds final search metrics as `info string analysis`). `Ponder`,
`MultiPV`, and Chess960 are accepted but ignored, since none are implemented
yet.

Search metrics are compiled out by default. Build an instrumented binary with:

```sh
go build ./cmd/uci                          # normal, no metrics overhead
go build -tags metrics -o gochess-metrics ./cmd/uci
go test -tags metrics ./...                 # metrics tests
```

For a release build with a version stamp:

```sh
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X github.com/ethankuehler/gochess/uci.Version=0.1.0" \
  -o gochess ./cmd/uci
```

## How it works

- **Board** — `chess.Position` stores 12 piece bitboards plus castling rights,
  the en passant square, and clocks. Squares are little-endian rank-file
  (a1 = bit 0, h8 = bit 63). FEN parsing is strict and round-trips.
- **Attack tables** — knight, king, and pawn tables plus magic bitboards for
  rooks and bishops live in `data/*.csv`, embedded with `//go:embed`. Tables are
  built at startup, so lookups are just array indexing.
- **Move generation** — checkers, absolute pins, and castling-transit safety are
  computed once per position, then ordinary moves are emitted directly through
  the check and pin masks. King and en-passant candidates are validated with
  copy-make (`makeMove` returns a new position) and an attack test. Castling,
  en passant, and promotions are handled with explicit move flags.
- **Evaluation** — material plus a hand-tuned piece-square table (centrality,
  pawn advancement, king safety). Scores are centipawns from White's view;
  search flips to the side-to-move convention.
- **Search** — iterative-deepening negamax alpha-beta with capture-first move
  ordering, PV tracking, mate-distance scoring, and cancellation by deadline,
  node count, or stop. A Zobrist-keyed transposition table caches results,
  supplies a hash move for ordering, and is reused across moves in a UCI game;
  `Hash` and `Clear Hash` control it. There is no pondering or opening book.
- **UCI layer** — `uci.Run` is the engine side, a separate goroutine owns each
  search, and `uci.Client` drives an in-process engine. `cmd/uci` is the
  standalone binary; `cmd/web` is the browser demo.

## Tools

| Command | What it does |
| --- | --- |
| `go run ./cmd/web [-addr 127.0.0.1:8080]` | Local board. You play White; toggle **Engine** to play Black at a depth you choose, or **Auto play** to watch the engine play both sides. Click-to-move, undo, reset, PGN export, and a **Claim draw** button for threefold/fifty-move draws; auto play claims those draws automatically, and reset cancels a running search. |
| `go run ./cmd/search -depth 5` | Fixed-depth search; prints best move, score, nodes, transposition-table probes/hits/cutoffs, time, and nps. Add `-fen "<FEN>"`, `-moves "e2e4 e7e5 ..."`, or `-eval` for a static score. |
| `go run ./cmd/perft -depth 5` | Move-generation node count with time and nps. Flags: `-fen`, `-moves`, `-divide`, `-suite data/perft.csv`, `-deep`, `-compare [-stockfish path]`. |
| `go run ./cmd/match -stockfish stockfish -games 4 -depth 4` | Play gochess against Stockfish. Every position is compared with Stockfish's perft, every move is checked for legality, and checkmate, stalemate, fifty-move, threefold, and insufficient-material results are adjudicated. Fails on any rule mismatch. Flags: `-fen`, `-moves`, `-random-plies`, `-seed`, `-validate-depth`, `-max-moves`, `-verbose`. |
| `go run ./cmd/genmagics` | Regenerate `data/rook_magics.csv` and `data/bishop_magics.csv`. |
| `python3 generate_attack_tables.py` | Regenerate the knight/king/pawn CSVs; requires `pandas`. |
| `go run .` | Scratch demo that prints a board. |

Run `cmd/search`, `cmd/perft`, and `cmd/genmagics` from the repo root: they open
`data/...` via relative paths. `cmd/uci`, `cmd/web`, and `cmd/match` use the
embedded files and do not care about the working directory.

## Runtime analysis

Runtime analysis is disabled by default. Start the local board with:

```sh
go run ./cmd/web -analysis
```

To include search phase metrics in the panel, use the metrics build:

```sh
go run -tags metrics ./cmd/web -analysis
```

The default build still provides CPU, heap, goroutine, and pprof sampling; only
the search-metric fields and the UCI `Analysis` option are compiled out.

Open **Runtime analysis** in the side panel during engine play or auto play. The
page plots process CPU utilization, live heap, goroutines, and per-iteration
search nodes, time, and NPS. CPU utilization is process user+system time and is
marked unavailable if the platform cannot supply it. It also shows inclusive
wall-clock timings and call counts for legal move generation, special-move
legality filtering (copy-make plus attack test), evaluation, and search-edge
copy-make. Every copy-make call is counted, including the copies made while
filtering; their time is reported with the legality filter because timing each
individual copy would add too much overhead.
The bars are disjoint but incomplete: the panel reports the measured share and
an unattributed remainder covering draw checks, ordering, PV maintenance,
callbacks, and the timing instrumentation itself. `elapsedNs` is search-loop time, not the UCI `time` field. The server
retains five minutes of runtime samples and the latest 64 searches.

In the metrics build, collecting search metrics adds measurable overhead
(roughly 20–60% at moderate fixed depth, depending on the position and search
size), so a time-limited search may complete fewer iterations while it is on.
The default build has no metrics-wrapper or timer overhead; use it for real NPS
and strength benchmarks.

The panel links to Go CPU, heap, and trace profiles. Start a sustained search or
auto play, download the 10-second CPU profile, then inspect function hotspots and
flame graphs with:

```sh
go tool pprof -http=127.0.0.1:8081 gochess-cpu.pb.gz
```

Profiling routes and sampling exist only with `-analysis`, which requires a
loopback listen address. Profiles cover the whole local process, including HTTP
work, and short searches may contain too few CPU samples. Confirm optimizations
with normal, non-profiled benchmarks. Other UCI clients can request only the
final search metrics with `setoption name Analysis value true` when running a
`metrics` build.

## Benchmarking

```sh
./gochess bench          # 8 fixed positions, default depth 5
./gochess bench 6        # depth is capped at 8
```

The deterministic node counts are useful for comparing builds; the final stdout
line is `<nodes> nodes <nps> nps`, with per-position counts on stderr. The same
runner is available as the UCI `bench [depth]` command.

Go microbenchmarks:

```sh
go test -bench . ./chess
go test -bench 'Rook|Bishop|Search' -benchmem ./chess
```

For move generation, perft is the benchmark that matters:

```sh
go run ./cmd/perft -depth 6
go run ./cmd/perft -depth 4 -divide
go run ./cmd/perft -suite data/perft.csv          # -deep to include slow rows
```

## Playing on Lichess

`tools/lichess` wires the engine into the [Lichess Bot
API](https://lichess.org/api#tag/Bot) through the third-party
[lichess-bot](https://github.com/lichess-bot-devs/lichess-bot) bridge. It is
optional developer tooling: no Go package imports network code, and CI never
runs it.

```sh
./tools/lichess/setup.sh                  # clone bridge, install deps, build gochess
cp tools/lichess/.env.example tools/lichess/.env
# edit .env: LICHESS_BOT_TOKEN=lip_...    # bot:play scope; .env is gitignored
./tools/lichess/run.sh --upgrade          # once, irreversible
./tools/lichess/run.sh                    # rated bots + casual human challenges
```

The default config accepts rated standard blitz/rapid games from other bots and
casual challenges from humans; one of the two slots is reserved for human
challengers. Humans can challenge
[gochess6767](https://lichess.org/@/gochess6767) (Lichess only offers casual
games against bots), with at least a +2 increment. It writes PGNs to
`tools/lichess/games/` and logs to `tools/lichess/lichess-bot.log`. Lichess caps
bot-vs-bot games at 100/day, so a stable Glicko-2 rating takes several days of
running. That number is not comparable to `cmd/arena`'s Stockfish-scale Elo; use
it as a periodic external check, not a per-commit benchmark. See
[tools/lichess/README.md](tools/lichess/README.md) for the full walkthrough.

## Testing

```sh
go test ./...                 # full suite
go test -race ./...           # what CI runs
go vet ./...                  # static checks
PERFT_DEEP=1 go test ./chess -run '^TestPerft$' -v
STOCKFISH_BIN=stockfish go test ./chess -run TestPerftAgainstStockfish
STOCKFISH_BIN=stockfish go test ./cmd/match -run TestMatchAgainstStockfish
```

Tests are data-driven where practical: FEN fixtures live in `data/FEN.csv`,
sliding attacks in `data/sliding_attacks_examples.json`, and perft positions in
`data/perft.csv` (rows with `deep=1` only run with `PERFT_DEEP=1`). The fast
suite covers FEN parsing, magic tables, move generation, evaluation, search
limits, the UCI protocol, and the web endpoints; `cmd/uci` tests build and run
the binary from an empty temporary directory. The Stockfish tests run in CI and
locally when `STOCKFISH_BIN` is set; `cmd/perft -compare` remains an opt-in
debugging tool.

`cmd/match` is the end-to-end rules check. It plays real games against
Stockfish, compares our perft divide with Stockfish's at every position (depth
1 checks the complete legal move list), verifies every move before applying it,
and adjudicates checkmate, stalemate, fifty-move, threefold repetition, and
insufficient material. At a drawn or terminal position it also requires
gochess's own search to return the correct draw or mate score. Any mismatch is
a hard failure with the offending FEN and move list.

```sh
go run ./cmd/match -stockfish stockfish -games 4 -depth 4
go run ./cmd/match -stockfish stockfish -fen "4k3/8/8/8/8/8/8/R3K3 w - - 0 1" \
  -moves "e1d1 e8d8 d1e1 d8e8 e1d1 e8d8 d1e1 d8e8"
```

CI checks `gofmt`, `go build`, `go vet`, `go test -race ./...`, Stockfish perft
and match comparisons, a `bench` smoke test, and a Windows/macOS cross-compile.
Run `gofmt -l .` before pushing.

## Layout

```
chess/       engine library: bitboards, movegen, eval, search, perft, PGN
uci/         protocol layer: command loop, time management, client, bench
cmd/uci/     standalone UCI binary
cmd/web/     local board UI and in-process engine client
cmd/search/  fixed-depth search CLI
cmd/match/   gochess-vs-Stockfish match and rules verifier
cmd/perft/   perft runner, suite, and Stockfish comparison
cmd/genmagics/  magic-number CSV generator
data/        embedded attack tables, magics, and test fixtures
tools/lichess/  optional Lichess Bot API bridge (Python, not in the Go build)
```

## Known limitations

No pondering, MultiPV, Chess960, tablebases, or opening book. The transposition
table is single-threaded and keyed by the Zobrist position key, which excludes
the halfmove clock; table use is disabled near the fifty-move rule to avoid
reusing a score found in a different clock regime. Threefold repetition and the
fifty-move rule are detected during search when the caller supplies game history
(the UCI move list and the web demo both do); insufficient material is detected
as well. The engine can find short mates and win free material, but it is not
strong yet.
