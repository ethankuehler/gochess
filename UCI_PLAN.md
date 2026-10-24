# UCI Protocol Plan

Status: implemented for the protocol, search, clock, bench, and validation
scope. `cmd/web` drives the engine through an in-process `uci.Client`
(`position` / `go depth`), so the web game uses the same protocol as an
external GUI. Release packaging and CCRL/OpenBench submission are manual steps.
The optional Phase 4 items (transposition table / `Hash`, real pondering,
`MultiPV`, and OpenBench packaging) remain future work.

## 1. Goal and short version

Make `gochess` usable as an engine process by ordinary chess software
(Cute Chess, Arena, Banksia, Scid vs PC, python-chess, Lichess bots) and by
online testing groups such as CCRL, without changing the chess logic that
already works.

The short version:

1. Add a thin UCI layer (`cmd/uci` + `uci` package). Keep the `chess` package
   free of protocol formatting.
2. Embed the runtime attack tables in the binary. A GUI launches an engine
   from an arbitrary working directory, so the current `data/...` relative
   paths are a release blocker.
3. Add a UCI move matcher. `ParseUCIMove` deliberately returns notation only;
   `ApplyMove` currently requires the generated move's flags, so `e2e4`,
   captures, en passant, and castling do not apply as parsed today.
4. Refactor search just enough for UCI: iterative deepening, cancellation,
   deadline/node limits, principal variation (PV), and a best move that is
   available even when a search is stopped.
5. Add command-loop concurrency: read stdin while searching, answer `isready`
   immediately, honour `stop`/`quit`/EOF, and send exactly one `bestmove` per
   `go`.
6. Validate with scripted protocol tests, `cutechess-cli`, and python-chess,
   then build self-contained release binaries.

Most engines do **not** implement all of UCI. The feature matrix in section 4
is deliberately tiered: implement the parts GUIs and testers actually use,
accept-and-ignore the rest, and never advertise an option that is not
implemented.

## 2. What already exists

The `chess` package already provides the hard parts:

- bitboards, FEN parse/serialize, magic attack tables, legal move generation,
  copy-make, perft, evaluation, and fixed-depth alpha-beta.
- `Position.LegalMoves`, `Position.ApplyMove`, `Position.InCheck`,
  `Position.FEN`, `Position.SideToMove`.
- `Move.String`, `Move.From`, `Move.To`, `Move.Promotion`,
  `Move.IsCastling`, `Move.IsCapture`, `Move.IsEnPassant`,
  `Move.IsDoublePush`.
- `ParseUCIMove`, `Position.Search`, `SearchResult`, `MateScore`,
  `ScoreToMate`.

What is missing or unsuitable for UCI:

| Area | Current state | Consequence for UCI |
| --- | --- | --- |
| Entry point | `CMD search` prints a human-readable report; root `main.go` is a demo | No process speaks UCI |
| Data loading | `chess` loads `data/*.csv` relative to the current directory | Engine fails when a GUI launches it outside the repo |
| Move application | `ParseUCIMove` has no flags; `ApplyMove` compares full `Move` values | `position ... moves e2e4` fails for double pushes, captures, en passant, castling |
| Search control | `Search(depth)` is synchronous, fixed-depth, no stop/deadline/nodes | `go infinite`, `stop`, clocks, and `movetime` cannot work |
| Search reporting | No PV, no seldepth, no per-iteration callback | `info ... pv` is unavailable; stopping has no "best so far" |
| FEN strictness | `ParseFEN` requires exactly 6 fields | Some tools send 4-field FENs; a UCI adapter should pad them |
| Draws | No repetition/insufficient-material history | The GUI adjudicates; this is a strength limitation, not a protocol blocker |
| Options | No option framework, no hash, no threads, no ponder | Minimal option set needed; do not advertise unsupported options |

## 3. What UCI actually requires

The authoritative description is the April 2006 UCI specification
([text copy](https://sources.debian.org/data/main/t/tagua/1.0~alpha2-15/doc/uci-engine-interface.txt),
[Chess Programming Wiki overview](https://chessprogramming.org/UCI)).

### 3.1 Transport and lifecycle

- Communication is line-oriented over stdin/stdout. Every command and every
  response ends with `\n`.
- Arbitrary whitespace between tokens is allowed. Windows GUIs may send
  `\r\n`; strip a trailing `\r`.
- The engine must boot quickly and must not search before `go`.
- The engine must keep reading stdin while thinking.
- An unknown command or token should be ignored, not crash the engine.
- The engine must not write non-UCI text to stdout. Diagnostics belong on
  stderr or in `info string`.
- `isready` must always be answered with `readyok`, including while a search
  is running, without stopping that search.
- For every `go` there must be exactly one `bestmove`, including after
  `stop`. The specification also says this for pondering when `stop` is sent.
- If stdin closes, the engine should exit cleanly instead of becoming a
  zombie process. This matters to OpenBench and to tournament managers.

### 3.2 Commands from GUI to engine

| Command | What it means | MVP? |
| --- | --- | --- |
| `uci` | Switch to UCI mode. Reply with `id`, options, `uciok`. | Yes |
| `debug [on\|off]` | Extra diagnostics; off by default. | Yes |
| `isready` | Synchronize; always reply `readyok`. | Yes |
| `setoption name <id> [value <x>]` | Set an option. Names/values are case-insensitive and may contain spaces. | Yes (parse/accept) |
| `register ...` | Registration handshake. Only needed for copy-protected engines. | Ignore |
| `ucinewgame` | Next search is a new game; clear game-specific state. GUIs may omit it, so never rely on it. | Yes |
| `position [fen <fen> \| startpos] moves ...` | Set the position and apply moves. | Yes |
| `go ...` | Start searching with limits. | Yes |
| `stop` | Stop as soon as possible and send `bestmove`. | Yes |
| `ponderhit` | The expected move was played; continue as a normal search. | Optional |
| `quit` | Exit as soon as possible. | Yes |

### 3.3 Commands from engine to GUI

| Command | Purpose | MVP? |
| --- | --- | --- |
| `id name <x>` | Engine name. | Yes |
| `id author <x>` | Engine author. | Yes |
| `uciok` | Handshake complete. | Yes |
| `readyok` | Reply to `isready`. | Yes |
| `bestmove <move> [ponder <move>]` | Final move. | Yes (`ponder` optional) |
| `info ...` | Search progress and final statistics. | Yes (subset) |
| `option name ... type ...` | Advertise settable parameters. | Yes (small set) |
| `copyprotection ...` | Copy protection. | Not needed |
| `registration ...` | Registration. | Not needed |

### 3.4 `go` parameters

The specification defines: `searchmoves`, `ponder`, `wtime`, `btime`, `winc`,
`binc`, `movestogo`, `depth`, `nodes`, `mate`, `movetime`, `infinite`.

Times are milliseconds. `wtime`/`btime` are the clocks for White/Black; the
engine uses the side-to-move clock. `movestogo` is the number of moves until
the next time control and may be absent (sudden death or Fischer increment).
`depth` is in plies. `nodes` is a node limit. `movetime` is an exact search
time. `infinite` means search until `stop`.

### 3.5 `info` fields

The useful subset is: `depth`, `seldepth`, `time`, `nodes`, `pv`, `score cp`,
`score mate`, `nps`, `hashfull`, `currmove`, `currmovenumber`,
`multipv`, and `string`.

- `score cp` is centipawns from the engine's point of view, which is the side
  to move at the root.
- `score mate` is in **moves**, not plies. Negative values mean the engine is
  being mated.
- If `seldepth` is sent, `depth` must also be present.
- A final `info` immediately before `bestmove` is recommended so the GUI has
  the last search statistics.

### 3.6 Time-control semantics

- `go movetime N` means spend about N ms, not N plus an unbounded overhead.
- `go wtime W btime B winc I binc J movestogo M` describes a clock. With no
  `movestogo`, it is sudden death or Fischer; with `movestogo`, it is a
  repeating time control.
- The engine must reserve a move overhead. Network lag, GUI scheduling, and
  process wake-up can all consume time after `bestmove` is printed.
- If the remaining time is already tiny, play a legal fallback immediately;
  losing on time is worse than playing a weak move.
- `go infinite` must not return until `stop` (except when the position has no
  legal moves).
- `go ponder` searches the position after the opponent's predicted move and
  must not return until `ponderhit` or `stop`.

### 3.7 Compatibility notes found during research

- Stockfish sends `bestmove (none)` when there is no legal move; Komodo and
  others send `bestmove 0000`. Cute Chess explicitly accepts both. python-chess
  accepts `(none)`, `NULL`, and the `0000` null move. The UCI specification
  defines `0000` as the null move. **Recommendation: send `bestmove 0000`**,
  and keep a small constant so it is easy to switch if a GUI misbehaves.
- `position fen` contains spaces inside the FEN. Parse the FEN as a unit,
  not by taking a single token.
- Some GUIs send 4-field FENs (placement, side, castling, en passant) without
  the clocks. Pad to 6 fields for compatibility.
- `setoption` names and values may contain spaces and are case-insensitive.
  `setoption name Hash value 256` and `setoption name Clear Hash` must both
  parse correctly.
- `go searchmoves` consumes the rest of the line. It is not a normal
  key/value pair.
- A `go` with no limits has no time/depth bound; treat it as `infinite` and
  wait for `stop`. This follows the "a missing parameter does not influence
  the search" rule.
- Never advertise `Ponder`, `MultiPV`, `Hash`, `UCI_Chess960`, `SyzygyPath`,
  or `OwnBook` unless the engine really implements the behaviour. Advertising
  an option is a promise; unknown `setoption` commands should still be
  accepted and ignored gracefully.
- `ucinewgame` is optional from the GUI's point of view. The engine must work
  if the GUI never sends it.

## 4. Feature matrix and scope decision

In practice, most hobby engines implement the handshake, `position`, `go` with
clocks/`movetime`/`depth`, `stop`/`quit`, PV info, and a couple of options.
Stronger engines add a transposition table (`Hash`), `Threads`, pondering,
`MultiPV`, Chess960, and tablebases. Almost none implement registration or
copy protection. This plan follows that pattern: a complete, reliable core
first, then only the extras that a chosen target actually requires.

### 4.1 Must have (protocol compliance; first release)

- `cmd/uci` executable and a reusable `uci` package.
- `uci` handshake with `id name`, `id author`, options, `uciok`.
- `isready`/`readyok` before initialization and during search.
- `debug on|off` with `info string` diagnostics.
- `ucinewgame` clearing search state.
- `position startpos|fen ... [moves ...]`, including castling, en passant,
  double pushes, all four promotions, and 4-field FEN padding.
- `go depth`, `go movetime`, `go wtime/btime/winc/binc/movestogo`,
  `go infinite`.
- `stop`, `quit`, clean EOF exit.
- Exactly one `bestmove` per `go`.
- `info depth`, `score cp|mate`, `nodes`, `nps`, `time`, `pv`.
- Iterative deepening, cancellation, fallback best move, terminal-position
  handling.
- `bestmove 0000` for checkmate and stalemate, with `info score mate 0` or
  `info score cp 0`.
- Embedded runtime data and a binary that works from any directory.
- Unit and integration tests, including `go test -race`.
- Release builds for Linux, Windows, and macOS (amd64 and arm64).

### 4.2 Should have (online testing readiness)

- `go nodes` and `go searchmoves`.
- `seldepth` in `info`.
- `Move Overhead` spin option (default about 50 ms; 0-5000).
- `Threads` spin option with `default 1 min 1 max 1` for OpenBench-style
  frameworks. This truthfully says the engine is single-threaded.
- A deterministic `bench` command and a CLI `bench` argument that prints
  `N nodes M nps`.
- `-version`/`--version` and a version string in `id name`.
- Cross-platform CI and a release script/workflow.
- README instructions for Cute Chess, Arena, python-chess, and Lichess bots.
- Validation with `cutechess-cli` at sudden-death, Fischer, and repeating
  time controls.

### 4.3 Optional (add only if needed)

- Transposition table plus `Hash`/`Clear Hash`/`hashfull`. This is a search
  strength project, not a protocol requirement. It is required if the goal is
  OpenBench/Fishtest participation; CCRL can test an engine without a hash
  option (an engine author in the CCRL forum thread reported exactly that).
- Pondering (`Ponder` option, `go ponder`, `ponderhit`, `bestmove ... ponder`).
  CCRL runs with pondering off, so this is low priority.
- `MultiPV` for analysis GUIs.
- `go mate`.
- `currmove`/`currmovenumber` after one second.
- `UCI_ShowWDL`.
- A non-standard `d`/`eval`/`perft` debug command. Useful for development,
  but not required by GUIs.

### 4.4 Explicitly out of scope

- Chess960/FRC (`UCI_Chess960`).
- Syzygy/Nalimov/Gaviota tablebases (`SyzygyPath`, `tbhits`).
- Opening books (`OwnBook`).
- Registration and copy protection (`register`, `copyprotection`).
- SMP and `Threads > 1`.
- Strength limiting (`UCI_LimitStrength`, `UCI_Elo`, `Skill Level`).
- `UCI_Opponent`, `UCI_AnalyseMode`, `UCI_EngineAbout`, `UCI_ShowCurrLine`,
  `UCI_ShowRefutations`, `currline`, `refutation`.
- Node-as-time (`nodestime`).
- Variants (`UCI_Variant`).
- Network play. The engine speaks UCI; Lichess-bot or a GUI owns the network.

Rationale: every item in this list is optional in practice, and several
require changes that are much larger than the protocol layer. Implementing
them half-heartedly would be worse than not advertising them.

## 5. Architecture

### 5.1 Package and file layout

```
cmd/uci/main.go          thin entry point: version, bench, run UCI
uci/engine.go            Engine state, Run loop, lifecycle, concurrency
uci/command.go           line/token parsing and command dispatch
uci/options.go           option definitions, setoption parsing, defaults
uci/position.go          position command, FEN padding, move application
uci/search.go            start/stop search, info formatting, final info
uci/time.go              time allocation from UCI clocks
uci/output.go            synchronized, flushed stdout writer
uci/bench.go             deterministic bench positions and output
uci/*_test.go            unit, transcript, and integration tests

data/assets.go           //go:embed of the eight runtime CSVs
chess/data_fs.go         fs.FS loaders for attack tables and magics
chess/ucimove.go         FindLegalMove / ApplyUCIMove (notation -> flags)
chess/search.go          iterative deepening, limits, cancellation, PV
```

`cmd/uci` is deliberately thin. `uci` can be tested with pipes and buffers
without spawning a process. `chess` remains protocol-agnostic.

Dependency direction:

```
cmd/uci -> uci -> chess
             \-> data (embedded FS)
```

`chess` does not import `uci` or the embedded data. The UCI layer supplies
an `fs.FS` to the table builders.

### 5.2 Engine state

```go
type Engine struct {
    in       *bufio.Reader
    out      *lineWriter        // mutex + flush
    errOut   io.Writer

    mu       sync.Mutex         // guards position and options
    position *chess.Position
    posValid bool
    options  Options            // snapshot before each search
    debug    bool               // command-loop only

    searching bool
    cancel    context.CancelFunc
    done      chan struct{}     // closed when the search goroutine exits
    quitting  bool
}
```

States:

- **starting**: no `uci` yet. Only `uci`, `isready`, `quit`, `debug` are
  meaningful.
- **idle**: position known, no search. Almost every command is allowed.
- **searching**: `go` accepted. Only `stop`, `ponderhit`, `isready`,
  `debug`, `quit`, and another `uci` handshake are allowed. Other commands
  are ignored (with an `info string` in debug mode) because the
  specification says they are not supposed to arrive while searching.

The search goroutine owns the actual search and is the only writer of
`bestmove`. The command loop owns position and option mutation. This
division removes most races.

### 5.3 Command loop

```go
func (e *Engine) Run(ctx context.Context) error {
    for {
        line, err := e.in.ReadString('\n')
        if len(line) > 0 {
            e.handle(ctx, strings.TrimRight(line, "\r\n"))
        }
        if err != nil {
            e.quit()          // EOF or read error: cancel search, wait, return
            return nil
        }
    }
}
```

- Read with `bufio.Reader.ReadString`, not a small `Scanner` buffer. Long
  `position` lines with many moves should not hit a token limit.
- Trim `\r\n` and surrounding whitespace. Ignore empty lines.
- If `quit` or EOF: cancel the search, wait for `done`, then return. Never
  `os.Exit` from the search goroutine.
- `isready` calls `ensureInitialized`, which builds the attack tables once,
  then sends `readyok`. It does not wait for an active search.
- `uci` prints the handshake immediately; initialization is deferred to
  `isready`/`position`/`go` so boot stays fast.
- `ucinewgame` clears game state (and a future TT). It must work even if the
  GUI never sends it.

### 5.4 Output

One `lineWriter` owns stdout:

```go
func (w *lineWriter) line(format string, args ...any) {
    w.mu.Lock()
    defer w.mu.Unlock()
    fmt.Fprintf(w.buf, format+"\n", args...)
    w.buf.Flush()
}
```

Every protocol line goes through this writer and is flushed immediately.
This guarantees:

- `readyok` cannot be interleaved inside an `info` line.
- Lines reach the GUI without waiting for a buffer to fill.
- Write errors are detected; on error, cancel the search and stop.

No `log.Print` or `fmt.Print` may write to stdout. `log` goes to stderr, or
is replaced with a debug-only helper.

## 6. Detailed design

### 6.1 Handshake

On `uci`:

```
id name gochess <version>
id author <author>
option name Threads type spin default 1 min 1 max 1
option name Move Overhead type spin default 50 min 0 max 5000
uciok
```

Add `Hash`/`Clear Hash` only when a TT exists. Add `Ponder` only when
pondering is implemented. Do not print a banner before `uci` and do not print
anything after `uciok` unless it is an `info` line.

`id name` should include a version so testers can distinguish releases.

### 6.2 `setoption`

Parsing rules:

- Strip the `setoption` token.
- Find a case-insensitive `name` token followed by the option name. The name
  runs until a case-insensitive ` value ` token or end of line.
- The value runs to end of line and may contain spaces.
- Match known option names case-insensitively.
- Unknown options are accepted and ignored; log to stderr in debug mode.
- Unknown values are clamped or ignored, never fatal.
- `button` options have no value.

Examples:

```
setoption name Hash value 256
setoption name Clear Hash
setoption name Move Overhead value 100
setoption name UCI_Chess960 value true
```

The last one is not advertised; respond with an `info string` saying Chess960
is unsupported and keep standard chess. Do not silently pretend to support it.

### 6.3 `position`

Grammar:

```
position startpos [moves <move> ...]
position fen <6 FEN fields> [moves <move> ...]
```

Implementation:

1. Tokenize with `strings.Fields`.
2. If the first token is `startpos`, use `chess.NewStartingPosition()`.
3. If it is `fen`, consume FEN fields until `moves` or end of line.
   - 6 fields: parse as-is.
   - 4 fields: append `0 1`.
   - 5 fields: append `1`.
   - Other counts: report an error.
4. If `moves` is present, apply each token:
   - Parse with `chess.ParseUCIMove`.
   - Find the matching legal move by from/to/promotion, ignoring flags.
   - Apply the **generated** legal move, not the parsed notation-only move.
   - Stop and report on the first invalid move.
5. Store the resulting position only if the whole command succeeded.
   On any failure, mark the position invalid. A later `go` must not search a
   stale position; reply with `bestmove 0000` and an `info string`.

If `go` arrives before any successful `position`, default to the starting
position and log it. If a `position` command failed, do not search the stale
position; reply `bestmove 0000` with an `info string`.

The matching step is essential. For example, `e2e4` parses to a move with no
flags, while legal move generation attaches `doublePushFlag`. `ApplyMove`
compares the full struct, so the parsed move is rejected. The same applies to
captures, en passant, castling, and promotions.

Recommended `chess` helper:

```go
// FindLegalMove matches a notation-only move against the legal moves,
// ignoring flags that UCI notation cannot express.
func (b *Position) FindLegalMove(candidate Move) (Move, bool)

// ApplyUCIMove parses and applies a UCI move in one step.
func (b *Position) ApplyUCIMove(uci string) (*Position, Move, error)
```

`cmd/web` already contains this matching logic; move it into `chess` and reuse
it there.

Castling is sent as `e1g1`, `e1c1`, `e8g8`, `e8c8` in standard chess. Chess960
king-takes-rook notation is out of scope. A king-to-rook token may optionally
be mapped to the corresponding castling move for tolerant GUIs, but that is
not required by the standard.

### 6.4 `go` parsing

Parse tokens left to right:

| Token | Type | Handling |
| --- | --- | --- |
| `searchmoves` | rest of line | List of candidate root moves; filter to legal moves |
| `ponder` | flag | Ponder mode; optional implementation |
| `wtime`, `btime`, `winc`, `binc` | int ms | Clock; clamp negatives to 0 |
| `movestogo` | int | Positive move count; 0/negative means unknown |
| `depth` | int plies | Positive; 0/negative ignored |
| `nodes` | uint64 | Positive; 0 ignored |
| `mate` | int moves | Optional; otherwise ignore |
| `movetime` | int ms | Exact search time; 0/negative ignored |
| `infinite` | flag | No automatic exit except terminal positions |

If no limit is present, treat the search as infinite and wait for `stop`.
Unknown tokens are ignored. Invalid numbers are ignored and reported in debug
mode rather than crashing.

If several limits are present, the search stops at whichever is reached first
(depth, nodes, movetime, or clock deadline). `infinite` takes precedence for
*exit*: the engine may stop deepening at another limit, but it must not send
`bestmove` until `stop` (terminal positions excepted).

If a `go` arrives while a search is already running, ignore it (log in debug
mode). The GUI is supposed to send `stop` first. This avoids ambiguous
"which bestmove belongs to which go" behaviour.

### 6.5 Search API changes

Keep the existing fixed-depth API for `cmd/search`, tests, and the web demo,
but add a limit-aware API:

```go
type SearchLimits struct {
    MaxDepth int           // 0 = no limit
    MaxNodes uint64        // 0 = no limit
    Deadline time.Time     // zero = no deadline
    RootMoves []Move       // nil = all legal moves
}

type SearchInfo struct {
    Depth, SelDepth int
    Score           int
    Nodes           uint64
    Time            time.Duration
    PV              []Move
}

type SearchResult struct {
    BestMove   Move
    PonderMove Move   // optional; valid when PV has at least two moves
    Score      int
    HasMove    bool
    Depth      int    // deepest completed depth
    SelDepth   int
    Nodes      uint64
    PV         []Move
}

func (b *Position) SearchWithLimits(ctx context.Context, limits SearchLimits, info func(SearchInfo)) SearchResult
```

`Position.Search(depth)` becomes a wrapper:

```go
func (b *Position) Search(depth int) SearchResult {
    if depth < 1 {
        panic("chess: search depth must be at least 1")
    }
    return b.SearchWithLimits(context.Background(), SearchLimits{MaxDepth: depth}, nil)
}
```

This preserves existing scores, determinism, and tests while adding the new
capability. `SearchResult.Depth` changes from "requested depth" to "deepest
completed depth"; with no stop they are equal.

#### Iterative deepening

- Start at depth 1 and increase to `MaxDepth` (or until stopped).
- After each completed iteration, call `info` with the iteration's depth,
  score, nodes, time, and PV.
- Keep the best move and PV from the last completed iteration.
- If an iteration is interrupted, discard its partial result. If no iteration
  completed at all, use the first legal move as a fallback.
- Order the next iteration with the previous PV move first. This is optional
  but cheap and improves pruning.
- If `RootMoves` is non-empty, search only those moves. If the filtered list
  is empty, return `HasMove=false`.

#### Cancellation and limits

- Check `ctx.Err()` and the deadline every N nodes (256-1024; benchmark both)
  and at every root move boundary.
- Check `MaxNodes` at the same points.
- Return promptly: the UCI layer should be able to turn `stop` into a
  `bestmove` within a few milliseconds.
- Never block on a channel per node. `ctx.Err()` is cheap enough when sampled.
- Ensure a stopped search does not emit an `info` line for an incomplete
  iteration.

#### PV

Add a triangular PV table to `searcher`:

```go
var pvTable [maxPly + 1][maxPly + 1]Move
var pvLength [maxPly + 1]int
```

When a move improves alpha at a node, copy the child PV behind that move.
At the root, store the resulting line in the iteration result. Copy the PV
into `SearchResult.PV` before returning, because the table is reused by the
next iteration.

A `[129][129]Move` table is roughly 400 KB with the current `Move` layout;
that is acceptable for one searcher per search. If memory matters, use a
triangular allocation or a single flat slice, but do not optimise before
measuring.

#### Scores and mate reporting

- Keep `SearchResult.Score` from the side to move (negamax convention).
- UCI `info score cp` uses this value directly.
- For mate scores, call `chess.ScoreToMate` and emit `score mate <moves>`.
  `ScoreToMate` already returns moves, not plies.
- At a terminal root, `HasMove=false`; the UCI layer emits `score mate 0`
  when in check and `score cp 0` for stalemate, then `bestmove 0000`.
- Do not send a mate score as a large `cp` value.

#### Seldepth

Track the maximum ply reached by the search and report it as `seldepth`.
This is optional but cheap; remember to include `depth` in the same info line.

### 6.6 Time management

Implement allocation in `uci/time.go`, not in `chess`. The search only respects
a deadline. This keeps the chess package reusable and makes the formulas easy
to unit test.

Inputs:

- `movetime` (exact)
- `wtime`, `btime`, `winc`, `binc`, `movestogo`
- `depth`, `nodes`, `infinite`, `ponder`
- `Move Overhead` option (default 50 ms)

Recommended algorithm:

```
overhead = clamp(Move Overhead, 0, 5000)

if movetime > 0:
    budget = max(1, movetime - overhead)
    soft = budget
    hard = budget
else if remaining > 0:
    mtg = movestogo if 1 <= movestogo <= 50 else 30
    inc = max(0, increment)
    soft = remaining/mtg + 3*inc/4

    // Do not burn the whole clock on one move. The exact cap is a tuning
    // choice; safety matters more than strength here.
    cap = remaining / 4
    if movestogo > 0 {
        cap = remaining / 2
    }
    soft = min(soft, cap)

    hard = min(remaining - overhead, 2*soft)
    hard = max(hard, soft)

    if remaining <= 2*overhead + 10 {
        soft = 1
        hard = 1
    }
else:
    no time limit (depth/nodes/infinite only)
```

The search uses `soft` to decide whether to start another iteration and `hard`
as the context deadline. Check the hard deadline during search; after each
completed iteration, stop if elapsed >= soft. This is a simple, conservative
scheme. It is not Stockfish's time manager, and it does not need to be.

Notes:

- `movestogo` is a hint. A value of 1 at the end of a repeating control does
  not tell the engine whether a new time control starts after the move; be
  conservative.
- A 50 ms overhead is reasonable for local GUIs and online play. Very fast
  controls may benefit from 10-20 ms; make it configurable.
- If the engine is in check or has only one legal move, it may still spend
  time searching. A future optimisation can return immediately, but it is not
  required for compliance.
- For `go infinite`, set no deadline. On `stop`, cancel the context and use the
  last completed iteration.
- For `go ponder`, no deadline until `ponderhit`. The simplest correct
  implementation is to cancel and restart a timed search on `ponderhit`; a
  more advanced implementation updates the deadline in place. Pondering is
  optional for the first release.

### 6.7 `info` formatting

A completed iteration:

```
info depth 5 seldepth 9 score cp 34 nodes 123456 nps 456789 time 270 pv e2e4 e7e5 g1f3
```

A mate:

```
info depth 4 seldepth 6 score mate 2 nodes 12345 nps 400000 time 30 pv d1h5 g6h5
```

A terminal position:

```
info depth 0 score mate 0
bestmove 0000
```

or

```
info depth 0 score cp 0
bestmove 0000
```

Rules:

- `nps` uses elapsed milliseconds, clamped to at least 1 to avoid division by
  zero.
- `time` is milliseconds since the `go` command.
- `pv` is a space-separated list of UCI moves. Omit it if empty.
- Send a final `info` immediately before `bestmove`, even if the last
  iteration's info was already sent. The final line should reflect the move
  being returned.
- `bestmove` is exactly one token (plus optional `ponder`). Never include
  comments or extra fields.
- If the search was stopped before any iteration completed, emit
  `score cp 0` (or omit the score) and the fallback move. Do not emit a
  bogus mate score.
- Do not send `lowerbound`/`upperbound`: the search uses a full window and
  never returns a bound-only score. If aspiration windows are added later,
  these tokens become relevant.

A complete minimum session looks like this:

```text
GUI:    uci
engine: id name gochess 0.1.0
engine: id author <author>
engine: option name Threads type spin default 1 min 1 max 1
engine: option name Move Overhead type spin default 50 min 0 max 5000
engine: uciok
GUI:    isready
engine: readyok
GUI:    ucinewgame
GUI:    position startpos moves e2e4 e7e5
GUI:    go wtime 300000 btime 300000 winc 2000 binc 2000
engine: info depth 1 ... score cp 12 nodes 42 nps 42000 time 1 pv g1f3
engine: info depth 5 ... score cp 18 nodes 12345 nps 411500 time 30 pv g1f3 b8c6
engine: bestmove g1f3
GUI:    quit
```

The engine must not send anything before `uci`, and the final `bestmove` must
be legal in the position established by `position`.

### 6.8 Options

First release:

| Option | Type | Default | Range | Advertise in first release? |
| --- | --- | --- | --- | --- |
| `Threads` | spin | 1 | 1..1 | Yes (single-threaded) |
| `Move Overhead` | spin | 50 | 0..5000 | Yes |
| `Hash` | spin | 16 | 1..1024 | No; accept and ignore until a TT exists |
| `Clear Hash` | button | - | - | No; accept and ignore |
| `Ponder` | check | false | - | No; do not advertise until implemented |

Decision: **do not advertise `Hash` or `Clear Hash` in the first release** if
there is no transposition table. Accept `setoption name Hash ...` and ignore
it so a GUI that always sets Hash does not break. The UCI specification says
Hash "should be supported by all engines", but a no-op option is worse than
an honest omission; CCRL can test an engine without a hash option. Add a real
TT and then advertise Hash. If OpenBench is an immediate goal, move the TT
work into Phase 4 and advertise Hash at that point.

Parsing details:

- Match names case-insensitively.
- Trim spaces around numeric values.
- Clamp to the advertised range; report the clamp in debug mode.
- Accept `true`/`false` case-insensitively for check options.
- A button option has no value; do not error if a value is present.
- Do not change options while a search is running; ignore with a debug message
  or queue until idle. The specification says the GUI only sends setoption
  while the engine is waiting, so this is defensive.
- `Threads > 1` clamps to 1 and logs. Do not silently claim multi-threading.
- `UCI_Chess960=true` is unsupported: log it and keep standard chess.
  Do not advertise the option.

### 6.9 Boot and embedded data

The binary must be self-contained. A GUI may copy only the executable to a
different directory, or run it with an arbitrary working directory. Relative
`data/` paths will then fail, and `loadAttackTable` currently calls
`log.Fatal`, which kills the engine.

Plan:

1. Add `data/assets.go`:

   ```go
   // Package data embeds the runtime attack tables so the engine binary is
   // self-contained.
   package data

   import "embed"

   //go:embed knight_attacks.csv king_attacks.csv white_pawn_move.csv black_pawn_move.csv white_pawn_attacks.csv black_pawn_attacks.csv rook_magics.csv bishop_magics.csv
   var FS embed.FS
   ```

   The `go:embed` directive requires the files to live under the package
   directory. `data/` is already the canonical location, so this adds a Go
   file to the existing data folder instead of duplicating or moving CSVs.
   The package name can be `data`; the import path is
   `github.com/ethankuehler/gochess/data`.

2. Add `fs.FS`-based loaders in `chess`:

   ```go
   func BuildAllAttacksFromFS(fsys fs.FS) error
   func loadAttackTableFS(fsys fs.FS, name string) ([]BitBoard, error)
   func loadMagicEntriesFS(fsys fs.FS, name string) ([]MagicEntry, error)
   func buildSliderAttacksFS(fsys fs.FS, name string, dirs Directions) ([]MagicEntry, [][]BitBoard, error)
   ```

   For the UCI binary, missing or invalid embedded data is a startup error,
   not a silent random magic search. Keeping the random fallback would make
   startup time unpredictable and mask packaging mistakes.

3. Keep the existing disk-based `BuildAllAttacks` as a wrapper for tests and
   development commands. It can use `os.DirFS("data")` or continue to call
   the file loaders. Do not break `cmd/perft`, `cmd/search`, `cmd/web`, or
   `cmd/genmagics`.

4. `uci.Run` calls `chess.BuildAllAttacksFromFS(data.FS)` during
   initialization. If it fails, report `info string` and make `go` reply
   `bestmove 0000` rather than panic. Embedded data should make this path
   unreachable in a correct build.

5. Add a test that runs the built binary from an empty temporary directory
   with no `data/` tree. It must complete a handshake and a search.

6. Update `AGENTS.md` and the README: the UCI binary embeds
   `data/*.csv`; other commands still read `data/` from the repo root;
   `cmd/genmagics` still writes `data/`.

Alternative considered: copy the CSV files next to the binary in a release
zip. Rejected because GUIs often copy only the executable and because a
missing data directory produces a fatal error at the worst time.

### 6.10 Bench

`bench` is not a UCI command in the specification, but it is a de facto
standard for online testing (OpenBench/Fishtest-style frameworks). It is also
useful as a smoke test.

Requirements:

- Fixed position list, fixed depth, single thread, no time limit.
- Clear the transposition table (when one exists) before the run.
- Deterministic node count across repeated runs on the same build.
- Print a final line that matches `(\d+) nodes (\d+) nps` on stdout, e.g.
  `4712710 nodes 1323423 nps`.
- Exit 0 after printing.
- Do not print the UCI handshake or a banner. Extra progress lines may go to
  stderr.
- `./gochess bench` and the UCI command `bench` should both work. OpenBench
  invokes a public engine as `./engine bench`; private engines may use
  `"bench" "quit"` arguments. Ignore extra CLI arguments after `bench` and
  exit after printing.

Design:

- Put the position list in `uci/bench.go` as a `[]string` of FENs. Include
  the starting position, Kiwipete, a tactical middlegame, an endgame, and a
  mate position. Do not depend on `data/perft.csv` at runtime; the bench must
  work from any directory.
- Default depth 5 (or 4 if the target runtime is too long). Measure before
  freezing the number.
- Accumulate `nodes` over all positions and print `nodes` plus
  `nodes*1000/elapsedMS` for nps.
- Add a test that runs bench twice and compares node totals.

If OpenBench is a goal, also add a `Makefile` that accepts `CC=`, `EXE=`, and
`GOFLAGS=` and builds `cmd/uci`, plus a CI workflow that produces binaries for
the systems listed in the OpenBench configuration. That is packaging work,
not protocol work.

### 6.11 Debug and logging

- `debug off` by default.
- `debug on`: for every command received, send
  `info string <command>` (or log to stderr). Do not echo the engine's own
  `info`/`bestmove` lines.
- Parse errors, ignored commands, clamps, and unsupported options go to
  stderr. In debug mode, also send a short `info string` so the user can see
  them in the GUI.
- Never write a plain error message to stdout. GUIs treat stdout as protocol.
- Recover panics in the search goroutine:

  ```go
  defer func() {
      if r := recover(); r != nil {
          fmt.Fprintf(errOut, "search panic: %v\n", r)
          e.finishSearch(fallbackBestMove, ...)
      }
  }()
  ```

  A panic should not take down a tournament game silently. Send a legal
  fallback move if possible and log the panic to stderr.

## 7. Phases and work breakdown

### Phase 0: Foundations (no UCI process yet)

- `data/assets.go` with `//go:embed`.
- `fs.FS` loaders for the six attack tables and two magic files.
- `chess.FindLegalMove` / `chess.ApplyUCIMove`, reused by `cmd/web`.
- Search refactor: iterative deepening, limits, cancellation, PV, seldepth,
  fallback best move. Preserve `Search(depth)` behaviour.
- Tests: embed loading, move matching for every special move, PV legality,
  stop/cancel, depth/node limits, determinism.

Exit criteria: `go test -race ./...` passes; a tiny in-process program can
search a position with a deadline and get a legal best move/PV.

Effort: M.

### Phase 1: Protocol MVP

- `uci` package skeleton with `Engine`, `Run`, command loop, line writer.
- Handshake, `isready`, `debug`, `ucinewgame`, `position`, `go depth`,
  `go movetime`, `stop`, `quit`.
- `bestmove` exactly once; terminal handling with `0000`.
- `info depth/score/nodes/nps/time/pv`.
- `cmd/uci/main.go`.
- Transcript tests with pipes.
- Subprocess test: handshake, search, stop, quit, EOF.
- CWD-independence test from a temporary directory.

Exit criteria: a scripted transcript produces the expected handshake, info,
and bestmove; no stdout pollution; the binary works outside the repo.

Effort: M.

### Phase 2: Clock handling and robustness

- `go wtime/btime/winc/binc/movestogo`, `go infinite`, `go nodes`,
  `go searchmoves`.
- `uci/time.go` with unit tests for the allocation table.
- `isready` during search; `stop` when idle; `quit` during search; EOF;
  repeated `go`; `stop` immediately after `go`.
- Output mutex and write-error handling.
- Race tests for search/command concurrency.
- Time-forfeit test: 100 self-play games at 10+0.1 and 40/60+0.6 via
  `cutechess-cli`; zero time losses, illegal moves, or disconnects.

Exit criteria: clocks are respected, stop is responsive, and the race
detector is clean under interleaved commands.

Effort: M.

### Phase 3: Online/submission polish

- Options: `Threads` (1..1), `Move Overhead`.
- `bench` command/argument and deterministic node count.
- `-version` and version string.
- Cross-compile release builds for linux/amd64, linux/arm64, windows/amd64,
  darwin/amd64, darwin/arm64.
- README: GUI setup, supported commands/options, time controls, bench,
  known limitations.
- AGENTS.md update.
- Validation in Cute Chess, python-chess, and at least one GUI (Arena or
  Banksia). Optional: Lichess-bot smoke test.
- CCRL submission checklist.

Build example (repeat per OS/arch):

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags="-s -w -X github.com/ethankuehler/gochess/uci.Version=0.1.0" \
  -o dist/gochess-0.1.0-linux-amd64 ./cmd/uci
```

Unsigned macOS binaries may be quarantined by Gatekeeper; document that users
may need to allow the binary, or sign/notarize it in a release workflow.

Exit criteria: a self-contained binary plays clean games in two GUIs and
passes the `cutechess-cli` match; the README explains how to run it.

Effort: S-M.

### Phase 4: Optional compatibility/strength work

Pick only what the target needs:

- Transposition table + Zobrist hashing + `Hash`, `Clear Hash`, `hashfull`.
  This is also a strength improvement, but it is a separate search project
  with its own correctness tests (mate-score adjustment, collisions,
  deterministic bench with a cleared table).
- Pondering (`Ponder`, `go ponder`, `ponderhit`, `bestmove ... ponder`).
- `MultiPV`.
- `go mate`, `currmove`/`currmovenumber`, `UCI_ShowWDL`.
- OpenBench Makefile/workflow.

Exit criteria: depends on the chosen feature. Do not let this phase delay the
UCI MVP.

Effort: L for the TT; S-M for ponder/MultiPV.

## 8. Test plan

### 8.1 Unit tests

- **Command parser**: `uci`, `isready`, `debug on/off`, `quit`, empty lines,
  leading/trailing whitespace, CRLF, unknown commands.
- **`setoption` parser**: names with spaces, `value` with spaces, case
  differences, button with no value, unknown option, numeric clamping.
- **`position` parser**: startpos, 6-field FEN, 4-field FEN, 5-field FEN,
  no moves, many moves, malformed FEN, illegal move.
- **Move matching**: quiet move, double push, capture, en passant, castling
  both sides, all four promotions, promotion capture, illegal move,
  ambiguous underpromotion.
- **`go` parser**: every limit alone and in combinations, unknown tokens,
  invalid numbers, `searchmoves` consuming the rest.
- **Time allocation**: sudden death, Fischer increment, repeating
  `movestogo`, low time, `movetime`, no clock, negative/zero inputs. Assert
  `0 < soft <= hard <= remaining - overhead` (with sensible exceptions at
  the tiny-time boundary).
- **Info formatting**: cp, positive mate, negative mate, stalemate,
  checkmate, PV, nps/time, no PV.
- **Options**: defaults, ranges, case-insensitive names, unsupported options.

### 8.2 Search tests

- Iterative deepening reaches the requested depth; `Search(depth)` still
  matches the old scores on the existing fixtures.
- Cancel mid-search returns a legal move from the last completed iteration.
- Cancel before depth 1 returns a legal fallback.
- Node limit stops near the requested count.
- PV is legal and applyable at every ply.
- Mate scores and `ScoreToMate` conversion remain correct.
- `seldepth >= depth` where applicable.
- Determinism: same position, same limits, same best move/score/nodes
  (without a TT).
- No position mutation.
- Existing alpha-beta/minimax and perft tests still pass.

### 8.3 Protocol transcript tests (in-process)

Use `io.Pipe` or a pair of pipes and run `Engine.Run` in a goroutine. A helper
sends lines and reads with a timeout.

Cases:

```
uci                       -> id..., option..., uciok
isready                   -> readyok
position startpos
go depth 2                -> info..., bestmove <legal>
position fen <4 fields>
go movetime 50            -> bestmove within a reasonable bound
position startpos moves e2e4 e7e5 g1f3
go depth 1                -> bestmove
position ... <checkmate>  -> info score mate 0, bestmove 0000
position ... <stalemate>  -> info score cp 0, bestmove 0000
go infinite               -> no bestmove
stop                      -> bestmove appears
isready (during search)   -> readyok appears before bestmove
quit                      -> process/loop exits
EOF                       -> loop exits cleanly
```

Also test:

- `go depth 2` twice: exactly two bestmoves, no extra.
- `stop` when idle: no output.
- `quit` during search: exits without hanging; no panic.
- `position` during search: ignored, current search unaffected.
- `setoption` during search: ignored, current search unaffected.
- `uci` during search: handshake still emitted, search continues.
- `ucinewgame` clears TT (when implemented) and resets state.
- `debug on` emits `info string` and does not affect bestmove.
- `bestmove` is legal in the current position for every tested `go`.
- No line on stdout begins with an unknown token.

### 8.4 Subprocess/integration tests

- Build `cmd/uci` once into a temporary directory in `TestMain` (or use a
  prebuilt binary from `go test`).
- Run it with `cmd.Dir = t.TempDir()` and no `data/` tree. Verify handshake,
  `position startpos`, `go depth 3`, `bestmove`, `quit`, exit 0.
- Send CRLF line endings on Windows; verify no `\r` leaks into FEN/moves.
- Close stdin and assert the process exits within a short timeout.
- Send `go infinite`, then `quit`; assert exit within a short timeout.
- Read stdout and stderr separately; assert stdout contains only protocol
  lines and stderr contains diagnostics.
- Run the same binary from the repo root and from a directory containing a
  decoy `data/` directory; results should be identical (embedded data wins).

### 8.5 External validation

Cute Chess:

```sh
cutechess-cli \
  -engine cmd=./gochess name=gochess proto=uci \
  -engine cmd=./gochess name=gochess2 proto=uci \
  -each tc=10+0.1 -games 100 -repeat -recover \
  -pgnout /tmp/gochess-uci.pgn
```

Repeat with a repeating control (`tc=40/60+0.6`) and a Fischer control
(`tc=2+1`). Check the summary for time losses, illegal moves, disconnects,
and crashes. Add `-debug` if a failure needs a protocol trace.

python-chess:

```python
import chess
import chess.engine

with chess.engine.SimpleEngine.popen_uci("./gochess") as engine:
    engine.ping()
    board = chess.Board()
    for _ in range(20):
        result = engine.play(board, chess.engine.Limit(time=0.1))
        board.push(result.move)
    info = engine.analyse(board, chess.engine.Limit(depth=4))
    print(info)
```

This checks handshake, `isready`, `position`, `go movetime`, `go depth`,
`bestmove`, and `quit`. It is also close to what Lichess-bot does.

GUI smoke tests:

- Arena (Windows): install as a UCI engine, play 10+0 and 2+1.
- Banksia or Scid vs PC (macOS): same.
- Verify the engine appears, options show only the supported ones, and a game
  completes without an illegal move dialog.

CCRL:

- Provide the binary, source link, author name/country, origin (from scratch),
  and an estimated rating if available. CCRL sets Hash 256/512 and pondering
  off; the engine does not need a book or tablebases. An engine without a
  hash option has been accepted before, but a TT makes the engine more
  competitive.

OpenBench (only if targeted):

- `./gochess bench` prints `N nodes M nps` and exits 0.
- `Threads` and `Hash` options are present and accepted.
- The engine exits on stdin EOF.
- A Makefile accepts `CC=`/`EXE=` and produces the expected binary names.

### 8.6 CI

Keep the existing `gofmt`, build, vet, and `go test -race ./...` steps. Add:

- A `cmd/uci` build step (covered by `go build ./...`).
- A subprocess integration test guarded by `testing.Short()` if the build
  step is slow.
- A matrix for Linux and Windows at minimum; macOS is useful for release
  confidence.
- A release workflow or script for the five target binaries.

Timing tests are the most likely source of CI flakiness. Use generous bounds
(e.g., `go movetime 100` must return within 1 second), and skip the longest
self-play checks in normal CI; run them manually before a release.

## 9. Gotchas and decisions

1. **`ApplyMove` is not a UCI-move API.** Add `FindLegalMove` and match by
   from/to/promotion. Do not loosen `ApplyMove` to accept flagless moves; that
   would hide bugs in other callers.
2. **`ParseFEN` is strict.** Pad 4/5-field FENs in the UCI adapter. Keep the
   strict parser for internal correctness.
3. **Data paths.** Embedding is mandatory for a portable binary. Do not rely
   on `os.Getwd` or an install directory.
4. **`bestmove` for no legal moves.** Use `0000` (the spec's null move).
   Cute Chess and python-chess accept it; Stockfish's `(none)` is also
   accepted, so keep the value in one constant for easy testing.
5. **`go` alone means infinite.** There is no time/depth/node limit, so wait
   for `stop`. Document this in the README.
6. **Terminal positions in infinite mode.** If there are no legal moves,
   reply immediately with `bestmove 0000` even for `go infinite`/`go ponder`;
   there is nothing to search or ponder.
7. **`isready` during search.** Answer it from the command loop, protected by
   the output mutex. Never stop the search to answer it.
8. **Exactly one `bestmove`.** Only the search goroutine writes it. `stop`
   cancels; it does not write a second move. `quit`/EOF may skip `bestmove`
   because the GUI is gone, but must not hang.
9. **Don't advertise unsupported options.** Accept unknown `setoption`
   commands, but only print `option` lines for features that work. A GUI that
   sees `Ponder` will try to ponder.
10. **Score point of view.** `SearchResult.Score` is side-to-move relative;
    UCI wants engine-relative, which is the same at the root. Do not flip it
    again in the UCI layer.
11. **Mate distances.** `ScoreToMate` returns moves. Do not divide by two
    again.
12. **Time forfeits.** Reserve overhead, use a hard deadline, check it during
    search, and keep a legal fallback. A weak move is better than a loss on
    time.
13. **Stop granularity.** Check cancellation every few hundred nodes, not
    once per iteration. A depth-8 iteration can take a long time.
14. **Partial iterations.** Do not report a PV/score from an iteration that
    was cut off. Use the last completed iteration.
15. **PV legality.** A PV is a sequence of moves from the root; verify that
    every move applies. A bug here makes GUIs show nonsense or crash.
16. **No stdout logs.** `log.Fatal` in the data loaders is especially
    dangerous for a GUI process: it exits without `bestmove`. Convert the UCI
    data path to returned errors.
17. **`ucinewgame` is optional.** Do not require it before `position`.
18. **`position` while searching.** Ignore it (or stop and restart only if a
    GUI is known to do this); the specification says it should not arrive.
19. **`setoption` while searching.** Ignore or queue; do not resize a hash
    table under a running search.
20. **Case and whitespace.** Token matching for commands can be
    case-insensitive in practice; option names/values must be. Trim `\r`.
21. **Draw history.** The engine does not know repetition or insufficient
    material. The GUI adjudicates. Do not claim a draw in `info`; just search.
22. **Fifty-move rule.** The search already returns 0 at
    `halfmoveClock >= 100`. The root position still needs a legal move and a
    `bestmove`.
23. **Search max ply.** `maxPly` is 128. ID should stop at `MaxDepth` or
    `maxPly`, whichever is smaller. Do not allocate unbounded PV tables.
24. **Bench determinism.** Clear the TT (when present), use one thread, fixed
    depth, fixed positions, and no time limit. Do not include the current
    date or random seeds.
25. **Windows paths and CRLF.** Test on Windows or with CRLF input. Avoid
    `os.Chdir` in the UCI binary; it is process-global and surprising.
26. **Exit codes.** `quit`/EOF should exit 0. A failed initialization may exit
    non-zero with a stderr message, but a GUI-launched engine should not print
    a Go panic trace to stdout.
27. **Unknown leading tokens.** Ignore the whole unknown line rather than
    rescanning for a known command later in the line. The specification's
    `joho debug on` example is rarely exercised, and speculative rescanning
    can misread a FEN or an option value.

## 10. Definition of done

- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean.
- [ ] `go test -race ./...` passes, including the new UCI tests.
- [ ] `./gochess` responds to `uci` with `id`, options, and `uciok` within a
      reasonable time, and prints no banner before `uci`.
- [ ] `isready` works before initialization and while searching.
- [ ] `position startpos` and `position fen ...` work, including 4-field FENs
      and castling, en passant, double pushes, and promotions.
- [ ] `go depth`, `go movetime`, `go wtime/btime/winc/binc/movestogo`,
      `go infinite`, `go nodes`, and `go searchmoves` each produce exactly one
      legal `bestmove`.
- [ ] `stop` returns a `bestmove` promptly; `quit` and EOF exit cleanly.
- [ ] Checkmate and stalemate produce `bestmove 0000` with the correct info.
- [ ] `info` lines have legal PVs and correct cp/mate signs and units.
- [ ] 100 self-play games at 10+0.1 and at 40/60+0.6 finish with zero time
      losses, illegal moves, disconnects, or crashes.
- [ ] The binary runs from a temporary directory with no `data/` tree.
- [ ] `bench` is deterministic and prints `N nodes M nps`.
- [ ] Release binaries exist for Linux, Windows, and macOS (amd64/arm64).
- [ ] README explains GUI setup, supported options, time controls, and known
      limitations.
- [ ] AGENTS.md reflects the new packages, embedded data, and test commands.
- [ ] CCRL submission information (name, country, origin, source, estimated
      rating) is ready.

## 11. Open questions

These are the decisions worth settling before Phase 1. Defaults are
recommended in brackets.

1. **Primary online target**: CCRL, OpenBench/Fishtest, or Lichess bots?
   [CCRL + generic GUIs first.] This decides whether TT/`Hash` and `bench`
   are Phase 3 or Phase 4.
2. **Engine display name and author string**: `gochess <version>` /
   `<author>`? [Use the repo name and the author's preferred name.]
3. **Versioning**: start at `0.1.0`? [Yes, independent of the repo's commit
   history.]
4. **Hash/TT now or later**: add a TT in this project or keep UCI purely
   protocol-level? [Later, unless OpenBench is the target.]
5. **Ponder/MultiPV**: implement in the first release? [No.]
6. **No-move token**: `0000` or `(none)`? [0000, with a constant for easy
   testing.]
7. **Binary name**: `gochess`, `gochess-uci`, or versioned release names?
   [Build as `gochess`; release artifacts include OS/arch/version.]
8. **FEN tolerance**: accept 4/5-field FENs? [Yes, pad and log in debug mode.]
9. **Debug echo**: echo every received command in debug mode? [Yes, as
   `info string`; keep it off by default.]

## 12. References

- UCI specification, April 2006 (text copy):
  <https://sources.debian.org/data/main/t/tagua/1.0~alpha2-15/doc/uci-engine-interface.txt>
- UCI overview, Chess Programming Wiki:
  <https://chessprogramming.org/UCI>
- Stockfish `timeman.cpp` (reference for clock allocation ideas):
  <https://github.com/official-stockfish/Stockfish/blob/master/src/timeman.cpp>
- Stockfish `uci.cpp` (reference for command handling and `(none)`):
  <https://github.com/official-stockfish/Stockfish/blob/master/src/uci.cpp>
- Cute Chess UCI engine code (accepts both `(none)` and `0000`):
  <https://github.com/cutechess/cutechess/blob/master/projects/lib/src/uciengine.cpp>
- python-chess engine/UCI support:
  <https://github.com/niklasf/python-chess/blob/master/chess/engine.py>
- OpenBench engine requirements (Threads, Hash, deterministic bench, EOF):
  <https://github.com/AndyGrant/OpenBench/wiki/Configuring-New-Engines>
- CCRL testing conditions (time controls, hash, pondering off, no own book):
  <https://www.computerchess.org/about.html>
- CCRL forum thread on submitting an engine:
  <https://kirill-kryukov.com/chess/discussion-board/viewtopic.php?t=11745>
