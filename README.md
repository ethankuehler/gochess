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
and stalemate are `bestmove 0000`. Options are `Threads` (fixed at 1) and
`Move Overhead` (default 50 ms, range 0–5000). `Hash`, `Ponder`, `MultiPV`, and
Chess960 are accepted but ignored, since none are implemented yet.

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
- **Move generation** — pseudo-legal moves are generated with the tables, then
  copy-make (`makeMove` returns a new position) filters out moves that leave the
  king attacked. Castling, en passant, and promotions are handled with explicit
  move flags.
- **Evaluation** — material plus a hand-tuned piece-square table (centrality,
  pawn advancement, king safety). Scores are centipawns from White's view;
  search flips to the side-to-move convention.
- **Search** — iterative-deepening negamax alpha-beta with capture-first move
  ordering, PV tracking, mate-distance scoring, and cancellation by deadline,
  node count, or stop. There is no transposition table, pondering, or opening
  book.
- **UCI layer** — `uci.Run` is the engine side, a separate goroutine owns each
  search, and `uci.Client` drives an in-process engine. `cmd/uci` is the
  standalone binary; `cmd/web` is the browser demo.

## Tools

| Command | What it does |
| --- | --- |
| `go run ./cmd/web [-addr 127.0.0.1:8080]` | Local board. You play White; toggle **Engine** to play Black at a depth you choose, or **Auto play** to watch the engine play both sides. Click-to-move, undo, and reset; reset cancels a running search. |
| `go run ./cmd/search -depth 5` | Fixed-depth search; prints best move, score, nodes, time, and nps. Add `-fen "<FEN>"`, `-moves "e2e4 e7e5 ..."`, or `-eval` for a static score. |
| `go run ./cmd/perft -depth 5` | Move-generation node count with time and nps. Flags: `-fen`, `-moves`, `-divide`, `-suite data/perft.csv`, `-deep`, `-compare [-stockfish path]`. |
| `go run ./cmd/match -stockfish stockfish -games 4 -depth 4` | Play gochess against Stockfish. Every position is compared with Stockfish's perft, every move is checked for legality, and checkmate, stalemate, fifty-move, threefold, and insufficient-material results are adjudicated. Fails on any rule mismatch. Flags: `-fen`, `-moves`, `-random-plies`, `-seed`, `-validate-depth`, `-max-moves`, `-verbose`. |
| `go run ./cmd/genmagics` | Regenerate `data/rook_magics.csv` and `data/bishop_magics.csv`. |
| `python3 generate_attack_tables.py` | Regenerate the knight/king/pawn CSVs; requires `pandas`. |
| `go run .` | Scratch demo that prints a board. |

Run `cmd/search`, `cmd/perft`, and `cmd/genmagics` from the repo root: they open
`data/...` via relative paths. `cmd/uci`, `cmd/web`, and `cmd/match` use the
embedded files and do not care about the working directory.

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
chess/       engine library: bitboards, movegen, eval, search, perft
uci/         protocol layer: command loop, time management, client, bench
cmd/uci/     standalone UCI binary
cmd/web/     local board UI and in-process engine client
cmd/search/  fixed-depth search CLI
cmd/match/   gochess-vs-Stockfish match and rules verifier
cmd/perft/   perft runner, suite, and Stockfish comparison
cmd/genmagics/  magic-number CSV generator
data/        embedded attack tables, magics, and test fixtures
```

## Known limitations

No transposition table, pondering, MultiPV, Chess960, tablebases, or opening
book. Threefold repetition and the fifty-move rule are detected during search
when the caller supplies game history (the UCI move list and the web demo both
do); insufficient material is detected as well. The engine can find short mates
and win free material, but it is not strong yet.
