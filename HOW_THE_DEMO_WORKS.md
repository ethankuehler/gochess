# How The Demo Works

This document explains how the demo in `main.go` works and what it demonstrates about the GoChess engine.

## Quick Start

Run the demo with:
```bash
go run main.go
```

## What The Demo Does

The demo showcases the core functionality of the GoChess chess engine by:

1. **Loading a chess position** from FEN notation
2. **Displaying the board** visually with Unicode chess pieces
3. **Demonstrating attack generation** for a bishop piece using magic bitboards

## Step-by-Step Breakdown

### Step 1: Initialize Attack Tables

```go
chess.BuildAllAttacks()
```

This crucial first step loads all pre-computed attack tables into memory:
- **Knight attacks** - Pre-computed L-shaped move patterns for all 64 squares
- **King attacks** - Pre-computed one-square moves in all directions
- **Pawn moves** - Forward movement patterns for both white and black pawns
- **Pawn attacks** - Diagonal capture patterns for both colors
- **Rook magic bitboards** - Efficient sliding piece attack generation
- **Bishop magic bitboards** - Diagonal sliding piece attack generation

These tables enable O(1) (constant time) lookup of piece attacks, which is critical for fast move generation.

### Step 2: Load a Chess Position

```go
b, err := chess.NewBoardFEN("rnbqkb1r/1p2pppp/p2p1n2/8/3NP3/2N5/PPP2PPP/R1BQKB1R w KQkq - 0 6")
```

This loads a specific chess position using **FEN (Forsyth-Edwards Notation)**. Let's decode this FEN string:

- `rnbqkb1r/1p2pppp/p2p1n2/8/3NP3/2N5/PPP2PPP/R1BQKB1R` - Piece placement (rank 8 to rank 1)
  - Lowercase = black pieces (r=rook, n=knight, b=bishop, q=queen, k=king, p=pawn)
  - Uppercase = white pieces
  - Numbers = empty squares
  - `/` = separates ranks
- `w` - White to move
- `KQkq` - Castling rights (both sides can castle kingside and queenside)
- `-` - No en passant square available
- `0` - Halfmove clock (for 50-move rule)
- `6` - Fullmove number (move 6)

### Step 3: Display the Board

```go
fmt.Println(b.StringUni())
```

**Output:**
```
♜  ♞  ♝  ♛  ♚  ♝  _  ♜ 
 _  ♟  _  _  ♟  ♟  ♟  ♟ 
 ♟  _  _  ♟  _  ♞  _  _ 
 _  _  _  _  _  _  _  _ 
 _  _  _  ♘  ♙  _  _  _ 
 _  _  ♘  _  _  _  _  _ 
 ♙  ♙  ♙  _  _  ♙  ♙  ♙ 
 ♖  _  ♗  ♕  ♔  ♗  _  ♖  w KQkq - 0 6
```

This shows a visual representation using Unicode chess symbols:
- ♔♕♖♗♘♙ = White pieces (King, Queen, Rook, Bishop, Knight, Pawn)
- ♚♛♜♝♞♟ = Black pieces
- `_` = Empty square

### Step 4: Reconstruct FEN

```go
fmt.Println(b.FEN())
```

**Output:**
```
rnbqkb1r/1p2pppp/p2p1n2/8/3NP3/2N5/PPP2PPP/R1BQKB1R w KQkq - 0 6
```

This demonstrates that the engine can both parse and generate FEN notation, proving the internal board representation is correct.

### Step 5: Demonstrate Bishop Attack Generation

The demo tests the bishop attack generation system by examining the bishop at square c1:

```go
loc, err := chess.ShiftFromAlg("c1")  // Convert "c1" to internal position (5)
```

**Convert algebraic notation to position:**
- `c1` (algebraic notation) → position `5` (internal representation)
- The board uses 0-63 indexing where 0=a1, 7=h1, 56=a8, 63=h8

```go
blockers := b.Occupied(chess.BOTH)
fmt.Printf("blockers: %064b\n", blockers)
```

**Get all occupied squares as a 64-bit bitboard:**
```
blockers: 1111110101001111100101000000000000011000001000001110011110111101
```

Each bit represents a square:
- `1` = square is occupied
- `0` = square is empty

The binary representation shows the entire board state in a single 64-bit number. This is the power of **bitboards** - ultra-fast bit manipulation operations.

```go
mask := chess.GetBishopAttack(loc, blockers)
fmt.Println(mask.String())
```

**Calculate which squares the bishop can attack:**
```
 .  .  .  .  .  .  .  . 
 .  .  .  .  .  .  .  . 
 .  .  .  .  .  .  .  . 
 .  .  .  .  .  .  1  . 
 .  .  .  .  .  1  .  . 
 .  .  .  .  1  .  .  . 
 .  1  .  1  .  .  .  . 
 .  .  .  .  .  .  .  . 
```

The bishop at c1 can attack:
- **Diagonal squares**: b2, d2, a3, e3, f4, g5
- Blocked by other pieces (cannot jump over them)

This uses **magic bitboards** - a highly optimized technique that:
1. Uses a pre-computed "magic number" for each square
2. Multiplies the blocker pattern by the magic number
3. Uses the result as an index into a lookup table
4. Returns the attack pattern in O(1) time

## Key Concepts Demonstrated

### 1. FEN Notation
**Forsyth-Edwards Notation** is the standard way to describe a chess position as a text string. The engine can:
- Parse FEN to create board positions
- Generate FEN from board positions
- This enables easy position setup and debugging

### 2. Bitboards
A **bitboard** represents the board state using 64-bit integers:
- Each bit corresponds to one square (bit 0 = a1, bit 63 = h8)
- Ultra-fast operations using CPU bitwise instructions
- Can represent piece locations, attacked squares, or any board property

Example: The white knights in the demo are at positions that would be represented as a bitboard with just 2 bits set.

### 3. Magic Bitboards
**Magic bitboards** are an optimization technique for sliding pieces (rooks, bishops, queens):

**Problem:** Computing which squares a rook or bishop can attack requires considering blockers in multiple directions.

**Solution:** Pre-compute all possible attack patterns and store them in lookup tables. Use a hash function (the "magic number") to quickly find the right pattern.

**Benefits:**
- O(1) lookup time (extremely fast)
- Uses ~800 KB memory for rooks, ~40 KB for bishops
- Critical for fast move generation in chess engines

**How it works:**
```go
// Pseudo-code for magic bitboard lookup
blockers = board & relevant_squares_mask
hash = (blockers * magic_number) >> shift
attacks = lookup_table[square][hash]
```

### 4. Attack Generation
The demo shows how to:
1. Get a piece location from algebraic notation (e.g., "c1")
2. Get the current board occupancy (all pieces)
3. Calculate attack squares using magic bitboards
4. Display the result as a visual bitboard

This attack generation is the foundation for:
- Move generation (finding legal moves)
- Check detection (is the king under attack?)
- Move validation (is a move legal?)
- AI evaluation (controlling key squares)

## Architecture Overview

```
main.go
  └─> chess.BuildAllAttacks()          [Loads pre-computed data from CSV files]
  └─> chess.NewBoardFEN()               [Parses FEN string]
  └─> board.StringUni()                 [Renders Unicode board]
  └─> board.FEN()                       [Generates FEN string]
  └─> chess.ShiftFromAlg()              [Converts algebraic to internal position]
  └─> board.Occupied()                  [Returns bitboard of all pieces]
  └─> chess.GetBishopAttack()           [Magic bitboard lookup]
       └─> MagicIndex()                 [Hash function using magic number]
       └─> BISHOP_ATTACKS[square][hash] [O(1) lookup in pre-computed table]
```

## Data Files

The engine relies on pre-computed data stored in the `data/` directory:

- `knight_attacks.csv` - Knight move patterns (64 entries)
- `king_attacks.csv` - King move patterns (64 entries)
- `white_pawn_move.csv` / `black_pawn_move.csv` - Pawn forward moves
- `white_pawn_attacks.csv` / `black_pawn_attacks.csv` - Pawn captures
- `rook_magic.csv` - Magic numbers for rook attack generation (64 entries)
- `bishop_magic.csv` - Magic numbers for bishop attack generation (64 entries)

These files contain pre-computed values that would be expensive to calculate at runtime. The magic numbers in particular are found through brute-force search and are specific to the magic bitboard algorithm.

## Running the Demo with Different Positions

You can modify `main.go` to test different positions. Try these FEN strings:

**Starting position:**
```go
b, err := chess.NewBoardFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
```

**After 1.e4:**
```go
b, err := chess.NewBoardFEN("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1")
```

**Complex middle game:**
```go
b, err := chess.NewBoardFEN("r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4")
```

## Testing Different Pieces

Modify the demo to test different pieces:

**Test a rook at a1:**
```go
loc, _ := chess.ShiftFromAlg("a1")
mask := chess.GetRookAttack(loc, blockers)
fmt.Println(mask.String())
```

**Test a queen at d4:**
```go
loc, _ := chess.ShiftFromAlg("d4")
mask := chess.GetQueenAttack(loc, blockers)
fmt.Println(mask.String())
```

**Test a knight at g1:**
```go
loc, _ := chess.ShiftFromAlg("g1")
mask := chess.KNIGHT_ATTACKS[loc]
fmt.Println(mask.String())
```

## Performance Characteristics

The demo showcases a highly optimized chess engine design:

- **Attack table initialization:** One-time cost at startup (~0.5 seconds)
- **FEN parsing:** Fast string parsing (microseconds)
- **Attack lookup:** O(1) constant time using pre-computed tables
- **Bitboard operations:** CPU-level bit manipulation (nanoseconds)

This architecture enables:
- Fast move generation (thousands of positions per second)
- Efficient board representation (a few hundred bytes)
- Quick position evaluation for AI search

## Next Steps

The demo shows the foundation of the chess engine. Future enhancements include:

1. **Legal move generation** - Combining attack generation with rules (check, pins, castling)
2. **Move making/unmaking** - Efficiently updating the board state
3. **Position evaluation** - Scoring positions for AI play
4. **Search algorithms** - Minimax, alpha-beta pruning, iterative deepening
5. **Opening book** - Pre-computed optimal opening moves
6. **Endgame tablebases** - Perfect play in simple endgames

## References

- [Chess Programming Wiki](https://www.chessprogramming.org/) - Comprehensive chess programming resource
- [Magic Bitboards](https://www.chessprogramming.org/Magic_Bitboards) - Detailed explanation of the technique
- [FEN Notation](https://www.chessprogramming.org/Forsyth-Edwards_Notation) - Standard for describing positions
- [Bitboard Techniques](https://www.chessprogramming.org/Bitboards) - Board representation methods

## Conclusion

This demo illustrates the sophisticated engineering behind a modern chess engine. By using bitboards and magic bitboards, the GoChess engine achieves the performance necessary for real-time chess play and analysis. The demo is simple but showcases the critical components: position representation, FEN parsing, and efficient attack generation.

The code demonstrates that Go, while not traditionally used for chess engines, can implement these advanced algorithms effectively with clean, readable code.
