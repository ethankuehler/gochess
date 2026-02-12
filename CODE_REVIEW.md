# Code Review: gochess - Chess Engine in Go

**Reviewer:** GitHub Copilot  
**Date:** February 12, 2026  
**Repository:** ethankuehler/gochess  
**Lines of Code:** ~3,233 lines of Go code

---

## Executive Summary

This is a well-structured chess engine project written in Go. The project demonstrates solid understanding of chess programming concepts, including bitboard representation, magic bitboards for sliding piece move generation, and FEN notation. The code is in active development with good test coverage for core functionality.

**Overall Assessment:** ⭐⭐⭐⭐☆ (4/5)

### Key Strengths
- ✅ Clean bitboard-based chess representation
- ✅ Efficient magic bitboard implementation for sliding pieces
- ✅ Good test coverage (29 tests, all passing)
- ✅ Well-organized package structure
- ✅ No external dependencies (pure Go)
- ✅ FEN notation support for position import/export

### Critical Issues Fixed
- 🔧 **FIXED:** Missing `RANK_FILE_SIZE` constant causing build failures

### Areas for Improvement
- 📝 Documentation needs improvement
- 🧪 Move generation is incomplete (`LegalMoves()` is stubbed)
- 🎨 Some naming inconsistencies (typos in variable/function names)
- 🔒 Minor security considerations
- ⚡ Performance optimization opportunities

---

## Detailed Findings

### 1. **Architecture & Design** ⭐⭐⭐⭐⭐

#### Strengths:
- **Bitboard Representation:** Excellent use of `uint64` bitboards for efficient board state representation
- **Magic Bitboards:** Sophisticated implementation for sliding piece move generation
- **Separation of Concerns:** Clean separation between:
  - Core chess logic (`chess/` package)
  - Command-line tools (`cmd/demo`, `cmd/generate_magic`)
  - Pre-computed data (`data/` directory)
- **Type Safety:** Good use of custom types (`BitBoard`, `Shift`, `Piece`, `Colour`)

#### Code Example (Well-designed):
```go
type BoardState struct {
    pieces          [12]BitBoard // Elegant: 6 white + 6 black pieces
    enpassant       BitBoard
    encoding        uint8        // Compact: turn + castling rights
    halfmove_clock  uint16
    fullmove_number uint16
}
```

#### Recommendations:
- Consider adding an interface for different board representations if you want to experiment with alternatives
- The magic bitboard generation could be moved to a build-time tool to avoid runtime initialization

---

### 2. **Code Quality** ⭐⭐⭐⭐☆

#### Strengths:
- Code passes `go vet` with no issues
- Consistent formatting (mostly)
- Good use of Go idioms
- Proper error handling in most places

#### Issues Found:

##### a) **Spelling Errors** (Minor, but affects professionalism)
```go
// BitBoard.go:87 - "piceces" should be "pieces"
idx := slices.Index(PICECES_SYM, string(v))

// BitBoard.go:97 - "invald" should be "invalid"
return nil, errors.New("invald FEN, piece placement invalid")

// BitBoard.go:106 - "invald" should be "invalid"  
return nil, errors.New("invald FEN, invalid turn")

// BitBoard_test.go:15 - "Baord" should be "Board"
func TestNewBaordFEN(t *testing.T) {

// BitBoard.go:205 - "bord" should be "board"
// Returns a string showing the location of every piece on the bord

// BitBoard.go:222 - "determins" should be "determines"
// determins if there is a piece at the location loc

// BitBoard.go:268 - "centrin" should be "certain"
// Returns the position of all piceces of a centrin colour and type.

// BitBoard.go:268 - "piceces" should be "pieces"

// main.go:22 - "bishope" should be "bishop"
fmt.Println("testing bishope moves at c1")

// main.go:23 - "locaton" should be "location"
loc, err := chess.ShiftFromAlg("c1") // locaton of bishop
```

**Impact:** Low priority but affects code professionalism and search-ability

**Recommendation:** Run a spell checker on comments and variable names

##### b) **Inconsistent Naming**
```go
// Variable names use inconsistent conventions
var PICECES_SYM = []string{...}     // Should be PIECES_SYM
var UNI_PICECES_SYM = []string{...} // Should be UNI_PIECES_SYM
```

##### c) **Panic Usage**
```go
// BitBoard.go:271-272
if colour == BOTH || piece == ALL {
    panic("Invalid input in GetPieces, cannot be BOTH or ALL.")
}
```

**Issue:** Using `panic()` for input validation in library code
**Recommendation:** Return an error instead to allow callers to handle it gracefully

##### d) **Infinite Loop with `for true`**
```go
// Magic.go:165
for true {  // Should be: for { }
    moves := RayCast(loc, blockers, mask, ROOK_RAY)
    // ...
    if blockers == 0 {
        break
    }
}
```

**Recommendation:** Use `for { }` instead of `for true` (more idiomatic Go)

---

### 3. **Testing** ⭐⭐⭐⭐☆

#### Strengths:
- 29 comprehensive tests covering:
  - FEN parsing and round-trip conversion
  - Bitboard coordinate mapping
  - Piece placement
  - Move generation for all piece types
  - Magic bitboard raycasting
- All tests pass ✅
- Good use of table-driven tests with CSV data files
- Tests are well-named and descriptive

#### Test Results:
```
✅ TestNewBaordFEN
✅ TestOccupied  
✅ TestBitboardCoordinateMapping
✅ TestStartingPositionPiecePlacement
✅ TestManualPiecePlacement
✅ TestInvalidFENRejection
✅ TestMinimalPosition
✅ TestEnPassantEncoding
✅ TestCastlingRightsEncoding
✅ TestHalfmoveFullmoveClock
✅ TestMoveRoundTrip
✅ TestComplexPositionNoBitCollisions
... (and 17 more)
```

#### Areas for Improvement:
1. **Test Coverage:** No tests for:
   - `LegalMoves()` function (currently stubbed)
   - Move validation
   - Check/checkmate detection
   - Game state transitions

2. **Edge Cases:** Consider adding tests for:
   - Castling validation
   - En passant capture validation
   - Promotion handling
   - Threefold repetition
   - Fifty-move rule

3. **Benchmark Tests:** Add performance benchmarks for:
   - Magic bitboard lookups
   - Move generation
   - FEN parsing

**Example Recommendation:**
```go
func BenchmarkGetRookAttack(b *testing.B) {
    BuildAllAttacks()
    board := NewBoardDefault()
    occupied := board.Occupied(BOTH)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = GetRookAttack(0, occupied)
    }
}
```

---

### 4. **Documentation** ⭐⭐⭐☆☆

#### Strengths:
- Most exported functions have comments
- Complex algorithms (magic bitboards, raycasting) have explanatory comments
- README.md exists with project description

#### Issues:

##### a) **README is Too Casual**
Current README:
> "This is my own personal project for a chess engine written in Go. I wanted to use this as an opportunity to learn Go and see what it has to offer over something better for this use case, like C++ or Rust. I understand that Go isn't the right tool for the job, but if people are writing chess engines in Python and Java, why not Go? It can't be that bad, can it? btw this is not close to done.
> 
> Anyway, meow.
> Ethan."

**Recommendation:** While the casual tone is fine for a personal project, consider adding:
- Installation instructions
- Usage examples
- Project status/roadmap
- Architecture overview
- Performance characteristics

##### b) **Missing Package Documentation**
```go
// Add to chess/BitBoard.go:
// Package chess implements a chess engine using bitboard representation.
// It provides efficient move generation, position evaluation, and game state management.
package chess
```

##### c) **Incomplete Function Documentation**
Some functions lack parameter descriptions:
```go
// Better documentation example:
// GetRookAttack returns the attack bitboard for a rook at the given location.
// Uses magic bitboard technique for O(1) lookup.
// Parameters:
//   - loc: Square position of the rook (0-63)
//   - board: BitBoard representing all occupied squares
// Returns: BitBoard with all squares the rook can attack
func GetRookAttack(loc Shift, board BitBoard) BitBoard {
    // ... (already well-documented in the code!)
}
```

**Note:** The magic bitboard functions are actually well-documented! Keep that standard for other functions.

---

### 5. **Performance** ⭐⭐⭐⭐☆

#### Strengths:
- Bitboard operations are inherently fast (64-bit CPU operations)
- Magic bitboards provide O(1) sliding piece move generation
- Pre-computed attack tables for knights, kings, and pawns
- No heap allocations in hot paths (good for performance)

#### Potential Optimizations:

##### a) **Bit Manipulation Optimization**
```go
// Current (Move.go:39-58)
func (m *Move) String() string {
    scol, srow, ecol, erow := 0, 0, 0, 0
    colMask := COLUMN_MASK
    rowMask := ROW_MASK
    for i := range 8 {
        if (colMask & m.start) > 0 {
            scol = i
        }
        // ... more loops
    }
    return fmt.Sprintf("%c%d%c%d", COLUMNS[scol], srow+1, COLUMNS[ecol], erow+1)
}

// Optimized alternative:
func (m *Move) String() string {
    startShift := bits.TrailingZeros64(uint64(m.start))
    endShift := bits.TrailingZeros64(uint64(m.end))
    
    scol := startShift % 8
    srow := startShift / 8
    ecol := endShift % 8
    erow := endShift / 8
    
    return fmt.Sprintf("%c%d%c%d", COLUMNS[scol], srow+1, COLUMNS[ecol], erow+1)
}
```

##### b) **String Building Optimization**
```go
// Current (BitBoard.go:23-38)
func (b BitBoard) String() string {
    var buffer bytes.Buffer
    for rank := RANK_FILE_SIZE - 1; rank >= 0; rank-- {
        for file := 0; file < RANK_FILE_SIZE; file++ {
            mask := BitBoard(1) << ShiftFromCoords(Coordinates{uint64(file), uint64(rank)})
            if b&mask > 0 {
                buffer.WriteString(" 1 ")
            } else {
                buffer.WriteString(" . ")
            }
        }
        buffer.WriteRune('\n')
    }
    return buffer.String()
}

// Optimization: Use strings.Builder (slightly faster)
func (b BitBoard) String() string {
    var builder strings.Builder
    builder.Grow(200) // Pre-allocate capacity
    
    for rank := RANK_FILE_SIZE - 1; rank >= 0; rank-- {
        for file := 0; file < RANK_FILE_SIZE; file++ {
            shift := Shift(file + rank*8)
            if b&(BitBoard(1)<<shift) != 0 {
                builder.WriteString(" 1 ")
            } else {
                builder.WriteString(" . ")
            }
        }
        builder.WriteByte('\n')
    }
    return builder.String()
}
```

##### c) **Magic Number Loading**
Currently magic numbers are loaded from CSV at runtime. Consider:
- Generating as Go code at build time (faster startup)
- Using `//go:embed` to embed CSV files
- Binary format instead of CSV for faster parsing

---

### 6. **Security** ⭐⭐⭐⭐☆

#### Overall Assessment:
The project is a chess engine with no network I/O or external dependencies, so the attack surface is minimal. However, there are some considerations:

#### Findings:

##### a) **Resource Exhaustion (Low Risk)**
```go
// generate_magics.go:64-74
func findRookMagic(square Shift, mask BitBoard, indexBits Shift) uint64 {
    for {  // Infinite loop until magic found
        testMagic := rand.Uint64() & rand.Uint64() & rand.Uint64()
        // ...
    }
}
```

**Risk:** Infinite loop could hang program if no magic number is found (theoretical, but poor UX)

**Recommendation:** Add iteration counter and timeout:
```go
func findRookMagic(square Shift, mask BitBoard, indexBits Shift) uint64 {
    const maxAttempts = 100_000_000
    for attempt := 0; attempt < maxAttempts; attempt++ {
        testMagic := rand.Uint64() & rand.Uint64() & rand.Uint64()
        magicEntry := MagicEntry{mask, testMagic, indexBits}
        if tryRookMagicForGeneration(square, magicEntry) {
            return testMagic
        }
    }
    panic(fmt.Sprintf("Failed to find magic for square %d after %d attempts", square, maxAttempts))
}
```

##### b) **FEN Parsing (Low Risk)**
```go
// BitBoard.go:92
mask = mask >> (v - 48)  // Subtracting ASCII '0'
```

**Risk:** If `v < 48`, this could shift by a large unsigned value
**Current Status:** Likely safe due to FEN validation, but could be more explicit

**Recommendation:**
```go
if v >= '1' && v <= '8' {
    mask = mask >> (v - '0')
} else {
    return nil, fmt.Errorf("invalid FEN: unexpected character '%c'", v)
}
```

##### c) **CSV Parsing (Low Risk)**
```go
// Utils.go:41-42
reader := csv.NewReader(file)
reader.FieldsPerRecord = 0 // Allows variable fields
```

**Risk:** Variable field count could cause index out of bounds
**Current Status:** Handled with length checks, but could be more robust

**Recommendation:** Set expected field count explicitly

##### d) **Panic Instead of Error Returns**
Multiple uses of `panic()` for error conditions that could be handled gracefully:
- `GetPieces()` with invalid parameters
- `ShiftIter()` with invalid algebraic notation
- Magic number generation failures

**Recommendation:** Return errors instead of panicking to improve library usability

---

### 7. **Code Organization** ⭐⭐⭐⭐⭐

#### Excellent Structure:

```
gochess/
├── chess/                    # Core library
│   ├── BitBoard.go          # Board representation
│   ├── Move.go              # Move handling
│   ├── Magic.go             # Magic bitboard algorithms
│   ├── Const.go             # Constants and types
│   ├── Utils.go             # File I/O utilities
│   ├── generate_magics.go   # Magic number generation
│   └── *_test.go            # Comprehensive tests
├── cmd/                     # Command-line tools
│   ├── demo/               # Interactive demo
│   └── generate_magic/     # Magic number generator
├── data/                    # Pre-computed tables (CSV)
│   ├── rook_magic.csv
│   ├── bishop_magic.csv
│   └── *_attacks.csv
├── main.go                  # Example usage
└── display_binary.py        # Python utility for visualization
```

**Strengths:**
- Clear separation between library and executables
- Good file naming conventions
- Logical grouping of related functionality

**Minor Recommendations:**
1. Move `main.go` to `cmd/example/` for consistency
2. Consider moving CSV generation utilities to a separate `internal/` package
3. The Python script could be in a `scripts/` or `tools/` directory

---

### 8. **Incomplete Features** ⭐⭐☆☆☆

#### Critical Incomplete Functionality:

##### a) **Legal Move Generation** 
```go
// BitBoard.go:254-256
func (b *BoardState) LegalMoves() []Move {
    return make([]Move, 1) //TODO: get this working
}
```

**Impact:** This is core functionality for a chess engine!

**Recommendation:** This should be the highest priority for future development. Suggested approach:
1. Generate pseudo-legal moves (already have attack tables)
2. Filter out moves that leave king in check
3. Add special move handling (castling, en passant, promotion)

##### b) **Move Validation**
No function to validate if a move is legal in a given position

**Recommendation:**
```go
func (b *BoardState) IsLegalMove(move Move) bool {
    // Check if move is in list of legal moves
    // Or implement direct validation logic
}

func (b *BoardState) MakeMove(move Move) (*BoardState, error) {
    // Validate and execute move
    // Return new board state (immutable approach)
    // Or modify current state (mutable approach)
}
```

##### c) **Check/Checkmate Detection**
Missing functions:
```go
func (b *BoardState) IsInCheck(colour Colour) bool
func (b *BoardState) IsCheckmate() bool
func (b *BoardState) IsStalemate() bool
```

##### d) **Move Encoding**
```go
// Move.go:11-15
type Move struct {
    start    BitBoard
    end      BitBoard
    encoding uint16   // encoding for move information
}
```

The `encoding` field is defined but never used. Should encode:
- Piece type
- Capture flag
- Promotion piece
- Castling flag
- En passant flag

---

### 9. **Dependencies & Tooling** ⭐⭐⭐⭐⭐

#### Strengths:
- **Zero external dependencies!** (`go.mod` only references Go 1.23.2)
- Uses only standard library
- Good for security and maintenance

#### Go Version:
```go
// go.mod
go 1.23.2
```

**Note:** Using a recent Go version (1.23.2) which includes performance improvements and new features like iterators used in the code.

#### Python Utility:
The `display_binary.py` script uses pandas but is only for development/testing purposes.

**Recommendation:** Document the Python requirements if someone wants to regenerate CSV files:
```bash
# Add to README.md
## Development Requirements (optional)
- Python 3.x with pandas (for regenerating attack tables)
```

---

### 10. **Specific Code Issues**

#### Issue 1: Inconsistent Row Representation
```go
// Const.go uses file/rank with rank = row 0-7
type Coordinates struct {
    file uint64 // file (column) 0-7, corresponds to a-h
    rank uint64 // rank (row) 0-7, corresponds to 1-8
}

// But BitBoard.go:188 uses row 7-0 for display
for row := 7; row >= 0; row-- {
```

**Comment:** This is actually correct (rank 0 = row 1, rank 7 = row 8 in chess notation), but could be clearer in comments.

#### Issue 2: Magic Table Size Calculation
```go
// Magic.go:161
table := make([]BitBoard, 1<<(64-magic.Index)) //TODO: this need to be check to see if its correct
```

**Issue:** The TODO suggests uncertainty. Let me verify:
- If `magic.Index = 12`, then `1 << (64-12) = 1 << 52` which is HUGE
- Should be: `1 << magic.Index`

**This is a BUG!** 🐛

**Correct Code:**
```go
table := make([]BitBoard, 1<<magic.Index)
```

**Impact:** HIGH - This would allocate massive amounts of memory unnecessarily

#### Issue 3: GetBishopMask Edge Exclusion
```go
// Magic.go:125-126
if r > 0 && r < 7 && f > 0 && f < 7 {
    square := Shift(f + r*8)
    mask |= BitBoard(1) << square
}
```

**Issue:** This excludes edge squares from the mask, which is correct for magic bitboards, but the comment doesn't explain why. Edge squares don't affect attack generation (can't be blocked), so they're excluded to reduce table size.

**Recommendation:** Add explanatory comment:
```go
// Exclude edge squares - they can't be blockers (piece slides off board)
// This optimization reduces the number of bits in the mask and table size
if r > 0 && r < 7 && f > 0 && f < 7 {
```

#### Issue 4: GetRookMask Includes All Squares
```go
// Magic.go:98
return (COLUMN_MASK << file) | (ROW_MASK << (rank * 8))
```

**Issue:** Unlike bishop mask, rook mask includes ALL squares on rank/file, including edges. This is inconsistent with the edge-excluding optimization used for bishops.

**Recommendation:** Apply same edge exclusion to rook masks for consistency and smaller table sizes.

---

## Recommendations Summary

### 🔴 Critical Priority
1. **Fix Memory Bug:** Correct magic table size calculation (Issue #2 above)
2. **Implement LegalMoves():** Core functionality is missing
3. **Fix GetPieces() panic:** Return error instead

### 🟡 High Priority  
4. **Add Move Validation:** IsLegalMove(), MakeMove() functions
5. **Check/Checkmate Detection:** Essential for a chess engine
6. **Fix Spelling Errors:** Improves code professionalism
7. **Optimize Rook Mask:** Apply edge exclusion like bishop masks

### 🟢 Medium Priority
8. **Improve Documentation:** Better README, package docs, function comments
9. **Add Benchmarks:** Performance testing for key operations
10. **Handle Move Encoding:** Use the encoding field in Move struct
11. **Error Handling:** Replace panics with proper error returns

### 🔵 Low Priority
12. **Code Style:** Fix `for true` → `for { }`
13. **Naming Consistency:** PICECES_SYM → PIECES_SYM
14. **Optimization:** Use strings.Builder, bits package more
15. **Organization:** Move main.go to cmd/example/

---

## Testing Recommendations

### Add These Tests:
```go
// Test legal move generation
func TestLegalMovesStartingPosition(t *testing.T)
func TestLegalMovesCheckPosition(t *testing.T)
func TestLegalMovesCastling(t *testing.T)
func TestLegalMovesEnPassant(t *testing.T)

// Test move validation
func TestMakeMoveValid(t *testing.T)
func TestMakeMoveInvalid(t *testing.T)
func TestMakeMoveUpdatesState(t *testing.T)

// Test game state detection
func TestIsInCheck(t *testing.T)
func TestIsCheckmate(t *testing.T)
func TestIsStalemate(t *testing.T)

// Performance benchmarks
func BenchmarkGetRookAttack(b *testing.B)
func BenchmarkGetBishopAttack(b *testing.B)
func BenchmarkLegalMoves(b *testing.B)
func BenchmarkFENParsing(b *testing.B)
```

---

## Performance Profiling Suggestions

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profiling  
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof

# Trace analysis
go test -trace=trace.out -bench=.
go tool trace trace.out
```

---

## Suggested Improved README.md

```markdown
# GoChess - Chess Engine in Go

A chess engine implementation in Go using bitboard representation and magic bitboards for efficient move generation.

## Features

- ✅ Bitboard-based board representation
- ✅ Magic bitboards for sliding piece move generation  
- ✅ FEN notation support (import/export positions)
- ✅ Pre-computed attack tables for fast move generation
- 🚧 Legal move generation (in progress)
- 🚧 Check/checkmate detection (planned)
- 🚧 Position evaluation (planned)
- 🚧 Search algorithm (planned)

## Installation

```bash
go get github.com/ethankuehler/gochess
```

## Usage

```go
package main

import (
    "fmt"
    "github.com/ethankuehler/gochess/chess"
)

func main() {
    // Initialize attack tables
    chess.BuildAllAttacks()
    
    // Create a board from FEN notation
    board, err := chess.NewBoardFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
    if err != nil {
        panic(err)
    }
    
    // Display the board
    fmt.Println(board.StringUni())
    
    // Get possible moves for a bishop
    shift, _ := chess.ShiftFromAlg("c1")
    occupied := board.Occupied(chess.BOTH)
    moves := chess.GetBishopAttack(shift, occupied)
    fmt.Println(moves.String())
}
```

## Demo

Run the interactive demo to visualize piece moves:

```bash
go run cmd/demo/main.go --piece=bishop --square=c1
go run cmd/demo/main.go --piece=rook --square=a1 --color=black
```

## Development

### Running Tests
```bash
go test ./...
go test -v ./chess  # Verbose output
```

### Generating Magic Numbers
```bash
go run cmd/generate_magic/main.go
```

## Architecture

- **BitBoards**: 64-bit integers representing board positions
- **Magic Bitboards**: Perfect hashing for O(1) sliding piece move generation
- **Pre-computed Tables**: Knight, king, and pawn attacks stored in CSV files
- **FEN Support**: Standard chess position notation

## Project Status

This is a work-in-progress chess engine. Current focus is on completing legal move generation and implementing game logic.

## License

[Add license information]

## Author

Ethan Kuehler

---

*"If people are writing chess engines in Python and Java, why not Go?"*
```

---

## Conclusion

**Overall:** This is a well-structured chess engine with solid fundamentals. The bitboard representation and magic bitboard implementation demonstrate strong understanding of chess programming. The code quality is good with comprehensive testing.

**Main Concerns:**
1. 🐛 **Critical Bug:** Magic table size calculation is wrong
2. ⚠️ **Incomplete:** Legal move generation is stubbed
3. 📝 **Minor Issues:** Spelling errors and naming inconsistencies

**Recommendations:**
- Fix the critical memory bug immediately
- Focus on completing legal move generation
- Add check/checkmate detection
- Improve documentation
- Consider replacing panics with error returns

**Next Steps:**
1. Fix the magic table size bug
2. Implement LegalMoves() function
3. Add move validation
4. Implement check/checkmate detection
5. Add more comprehensive tests
6. Write better documentation

This is a promising project with a solid foundation. With the critical bug fixed and core functionality completed, it will be a fully functional chess engine!

---

## Code Quality Metrics

| Metric | Score | Notes |
|--------|-------|-------|
| Architecture | ⭐⭐⭐⭐⭐ | Excellent design |
| Code Quality | ⭐⭐⭐⭐☆ | Good, minor issues |
| Testing | ⭐⭐⭐⭐☆ | Good coverage, needs more |
| Documentation | ⭐⭐⭐☆☆ | Needs improvement |
| Performance | ⭐⭐⭐⭐☆ | Good, optimization opportunities |
| Security | ⭐⭐⭐⭐☆ | Low risk, minor issues |
| Completeness | ⭐⭐☆☆☆ | Core features missing |

**Overall: ⭐⭐⭐⭐☆ (4/5)** - Great foundation, needs completion

---

*End of Code Review*
