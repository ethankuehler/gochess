This is my own personal project for a chess engine written in Go. I wanted to use this as an opportunity to learn Go and see what it has to offer over something better for this use case, like C++ or Rust. I understand that Go isn’t the right tool for the job, but if people are writing chess engines in Python and Java, why not Go? It can’t be that bad, can it? btw this is not close to done.

Anyway, meow.
Ethan.

## Use as a UCI engine

Build the self-contained engine:

```sh
go build -o gochess ./cmd/uci

# release-style build with a version stamp
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X github.com/ethankuehler/gochess/uci.Version=0.1.0" \
  -o gochess ./cmd/uci
```

The attack tables are embedded in the binary, so it can be copied to another
directory and launched from any working directory. Add it to a UCI GUI such as
Cute Chess, Arena, Banksia, or Scid vs PC, or drive it directly. The web demo
below also drives this engine through an in-process UCI session.

```sh
{ printf 'uci\nisready\nposition startpos\ngo depth 4\n'; sleep 0.2; printf 'quit\n'; } | ./gochess
```

With python-chess:

```python
import chess.engine

with chess.engine.SimpleEngine.popen_uci("./gochess") as engine:
    board = chess.Board()
    result = engine.play(board, chess.engine.Limit(time=0.5))
    board.push(result.move)
```

The engine supports the standard handshake (`uci`, `isready`, `id`, options,
`uciok`, `readyok`), `debug`, `ucinewgame`, `position startpos|fen ... moves ...`,
`go depth|movetime|wtime/btime/winc/binc/movestogo|nodes|infinite|searchmoves`,
`stop`, and `quit`. It emits `info depth/seldepth/score/nodes/nps/time/pv` and
`bestmove`. Checkmate and stalemate are reported as `bestmove 0000`.

Options:

- `Threads` — fixed at 1; the engine is single-threaded.
- `Move Overhead` — milliseconds reserved from the clock for GUI/network
  latency (default 50, range 0-5000).

`Hash`, `Clear Hash`, `Ponder`, `MultiPV`, Chess960, tablebases, and opening
books are accepted or ignored where appropriate but are not advertised or
implemented yet. The engine does not track repetition or insufficient material;
the GUI adjudicates those draws.

Run the deterministic benchmark (used by engine-testing frameworks):

```sh
./gochess bench
./gochess bench 6
./gochess -version
```

## Run the board demo

```sh
go run ./cmd/web
```

Then open <http://127.0.0.1:8080>. The page uses the engine for legal move
generation and plays random replies; you can also click your own moves.

Turn on **Engine** in the side panel to play as White against the engine. The
server sends `position` and `go depth N` over UCI and applies the returned
`bestmove`. The depth selector trades speed for strength (1-5 ply). With the
engine off, the page goes back to random replies.

## Run the search

```sh
go run ./cmd/search -depth 4
go run ./cmd/search -fen "<FEN>" -depth 5
go run ./cmd/search -fen "<FEN>" -eval
```

The UCI engine uses iterative deepening with clock and node limits. `cmd/search`
is the small fixed-depth debugging tool. The evaluation is deliberately small:
material, piece centrality, king corner safety, and pawn advancement. It is not
strong yet, but it can find short mates and free material.
