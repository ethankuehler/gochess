# GoChess - Chess Engine in Go

A chess engine implementation in Go using bitboard representation and magic bitboards for efficient move generation.

## Features

- ✅ Bitboard-based board representation
- ✅ Magic bitboards for sliding piece move generation  
- ✅ FEN notation support (import/export positions)
- ✅ Pre-computed attack tables for fast move generation
- ✅ Unicode chess piece display
- 🚧 Legal move generation (in progress)
- 🚧 Check/checkmate detection (planned)
- 🚧 Position evaluation (planned)
- 🚧 Search algorithm (planned)

## Installation

```bash
go get github.com/ethankuehler/gochess
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/ethankuehler/gochess/chess"
)

func main() {
    // Initialize attack tables (call once at startup)
    chess.BuildAllAttacks()
    
    // Create a board from FEN notation
    board, err := chess.NewBoardFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
    if err != nil {
        panic(err)
    }
    
    // Display the board with Unicode pieces
    fmt.Println(board.StringUni())
    
    // Get possible moves for a bishop at c1
    shift, _ := chess.ShiftFromAlg("c1")
    occupied := board.Occupied(chess.BOTH)
    moves := chess.GetBishopAttack(shift, occupied)
    fmt.Println(moves.String())
}
```

## Demo

Run the interactive demo to visualize piece moves:

```bash
# Show bishop moves from c1
go run cmd/demo/main.go --piece=bishop --square=c1

# Show black rook moves from a8
go run cmd/demo/main.go --piece=rook --square=a8 --color=black

# Show queen moves with custom FEN position
go run cmd/demo/main.go --piece=queen --square=d4 --fen="8/8/8/8/3Q4/8/8/8 w - - 0 1"

# Available pieces: bishop, rook, queen, knight, king, pawn
```

## Development

### Project Structure

```
gochess/
├── chess/                    # Core chess library
│   ├── BitBoard.go          # Board representation and FEN parsing
│   ├── Move.go              # Move handling and notation
│   ├── Magic.go             # Magic bitboard algorithms
│   ├── Const.go             # Constants and type definitions
│   ├── Utils.go             # File I/O utilities
│   ├── generate_magics.go   # Magic number generation
│   └── *_test.go            # Comprehensive test suite
├── cmd/                     # Command-line tools
│   ├── demo/               # Interactive piece movement demo
│   └── generate_magic/     # Magic number generator utility
├── data/                    # Pre-computed attack tables (CSV)
│   ├── rook_magic.csv
│   ├── bishop_magic.csv
│   └── *_attacks.csv
└── main.go                  # Example usage
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./chess

# Run tests with coverage
go test -cover ./chess

# Run specific test
go test -v ./chess -run TestNewBoardFEN
```

### Generating Magic Numbers

If you need to regenerate the magic numbers for sliding pieces:

```bash
go run cmd/generate_magic/main.go
```

This will create/update:
- `data/rook_magic.csv`
- `data/bishop_magic.csv`

**Note:** This process can take several minutes as it uses random search to find optimal magic numbers.

### Code Quality

```bash
# Format code
go fmt ./...

# Run static analysis
go vet ./...

# Build all packages
go build ./...
```

## Architecture

### Bitboards

The engine uses 64-bit unsigned integers (bitboards) to represent chess positions. Each bit corresponds to a square on the board (bit 0 = a1, bit 63 = h8).

```
Bit Mapping:
56 57 58 59 60 61 62 63    (8th rank: a8-h8)
48 49 50 51 52 53 54 55    (7th rank: a7-h7)
...
8  9  10 11 12 13 14 15    (2nd rank: a2-h2)
0  1  2  3  4  5  6  7     (1st rank: a1-h1)
```

### Magic Bitboards

For efficient sliding piece (rook, bishop, queen) move generation, the engine uses magic bitboards. This technique provides O(1) move generation by:

1. Masking relevant occupancy bits
2. Multiplying by a pre-computed "magic number"
3. Shifting to produce an index into a lookup table

Magic numbers are pre-computed and stored in CSV files.

### Board State

The `BoardState` struct stores:
- 12 bitboards for pieces (6 types × 2 colors)
- En passant target square
- Castling rights and turn information (packed in 8 bits)
- Half-move and full-move clocks

This compact representation enables fast position evaluation and move generation.

## API Reference

### Creating Boards

```go
// Default starting position
board := chess.NewBoardDefault()

// From FEN notation
board, err := chess.NewBoardFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
```

### Board Display

```go
// ASCII display
fmt.Println(board.String())

// Unicode display
fmt.Println(board.StringUni())

// FEN notation
fmt.Println(board.FEN())
```

### Move Generation

```go
// Initialize attack tables (once at startup)
chess.BuildAllAttacks()

// Get occupied squares
occupied := board.Occupied(chess.BOTH)

// Get rook attacks from square
shift, _ := chess.ShiftFromAlg("a1")
rookMoves := chess.GetRookAttack(shift, occupied)

// Get bishop attacks
bishopMoves := chess.GetBishopAttack(shift, occupied)

// Get queen attacks (rook + bishop)
queenMoves := chess.GetQueenAttack(shift, occupied)

// Get knight attacks
knightMoves := chess.KNIGHT_ATTACKS[shift]

// Get king attacks  
kingMoves := chess.KING_ATTACKS[shift]

// Get pawn attacks/moves
whitePawnAttacks := chess.WHITE_PAWN_ATTACKS[shift]
whitePawnMoves := chess.WHITE_PAWN_MOVES[shift]
```

### Square Notation Conversion

```go
// Algebraic notation to shift (0-63)
shift, err := chess.ShiftFromAlg("e4")

// Algebraic notation to bitboard
loc, err := chess.LocFromAlg("e4")

// Bitboard to algebraic notation
alg := chess.AlgFromLoc(loc)

// Shift to coordinates
coords := chess.CoordsFromShift(shift) // {file: 4, rank: 3}
```

## Performance

The bitboard representation and magic bitboards provide excellent performance:

- Board representation: 264 bytes per position
- Attack generation: O(1) for all piece types
- No heap allocations in hot paths
- Suitable for high-speed position evaluation

## Project Status

**Current:** This is a work-in-progress chess engine focused on learning Go and chess programming concepts. The foundation is solid with efficient board representation and move generation.

**Next Steps:**
1. Complete legal move generation
2. Add check/checkmate detection  
3. Implement position evaluation
4. Add search algorithm (minimax/alpha-beta)
5. Create a playable interface

## Contributing

This is a personal learning project, but suggestions and feedback are welcome! Feel free to:
- Open issues for bugs or suggestions
- Submit pull requests for improvements
- Use the code for your own learning

## Resources

Useful resources for chess programming:
- [Chess Programming Wiki](https://www.chessprogramming.org/)
- [Bitboard representation](https://www.chessprogramming.org/Bitboards)
- [Magic Bitboards](https://www.chessprogramming.org/Magic_Bitboards)
- [FEN notation](https://en.wikipedia.org/wiki/Forsyth%E2%80%93Edwards_Notation)

## License

[Add license information]

## Author

Ethan Kuehler

---

*"If people are writing chess engines in Python and Java, why not Go? It can't be that bad, can it?"*

**Fun fact:** While Go isn't traditionally used for chess engines (C++ and Rust are more common due to lower-level control), Go's simplicity, strong standard library, and good performance make it a reasonable choice for a chess engine project!
