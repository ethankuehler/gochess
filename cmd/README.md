# Command-Line Utilities Documentation

This document explains the command-line utilities in the `cmd/` directory that support the GoChess engine development.

## Overview

The `cmd/` directory contains two utility programs:

1. **`generate_magic`** - Generates magic numbers for sliding piece attack generation
2. **`generate_test_data`** - Generates comprehensive test data files for the chess engine

These tools are used during development to create the pre-computed data files that the chess engine relies on for fast move generation.

---

## cmd/generate_magic

### Purpose

Generates "magic numbers" used in the magic bitboard technique for fast sliding piece (rook and bishop) attack generation.

### What are Magic Numbers?

Magic numbers are special 64-bit values used to hash blocker configurations into unique indices for attack lookup tables. Finding these numbers requires brute-force search, which is why this utility exists - to pre-compute them once and save them for reuse.

### Location

```
cmd/generate_magic/main.go
```

### Usage

```bash
cd /home/runner/work/gochess/gochess
go run cmd/generate_magic/main.go
```

### What It Does

1. **Initializes Simple Piece Attack Tables**
   - Loads knight, king, and pawn attack patterns
   - These are needed for the generation process

2. **Generates Rook Magic Numbers** (64 entries, one per square)
   - Uses random search to find valid magic numbers
   - Tests each candidate to ensure no collisions
   - Can take several seconds to complete
   - Prints progress: "Finding magic for rook square N..."

3. **Saves Rook Magics**
   - Writes to `data/rook_magic.csv`
   - Format: `square,mask,magic,index_bits`

4. **Generates Bishop Magic Numbers** (64 entries, one per square)
   - Similar process to rooks but for diagonal movement
   - Prints progress: "Finding magic for bishop square N..."

5. **Saves Bishop Magics**
   - Writes to `data/bishop_magic.csv`
   - Same CSV format as rooks

### Output Files

- **`data/rook_magic.csv`** - Magic numbers for rook attack generation
- **`data/bishop_magic.csv`** - Magic numbers for bishop attack generation

### Sample Output

```
Generating Rook Magic Numbers...
Finding magic for rook square 0...
  Found: 0xAC000D00928000 (bits: 15)
Finding magic for rook square 1...
  Found: 0x5228000506001400 (bits: 15)
...
Rook magics saved successfully!

Generating Bishop Magic Numbers...
Finding magic for bishop square 0...
  Found: 0x605200809002800 (bits: 6)
...
Bishop magics saved successfully!

All magic numbers generated and saved!
```

### When to Use

- **After modifying the magic bitboard algorithm** - If you change how magic numbers are validated or indexed
- **After changing the coordinate system** - If the bitboard layout changes
- **When the CSV files are missing** - To regenerate from scratch
- **For experimentation** - To try different magic number search strategies

### Performance

- **Rooks**: ~10-30 seconds (64 squares × random search)
- **Bishops**: ~5-10 seconds (fewer relevant squares per position)
- **Total**: ~15-40 seconds depending on luck with random search

### Technical Details

**Magic Number Properties:**
- Must create unique hash values for all possible blocker configurations
- Typically sparse numbers (many zero bits) work best
- Found through random trial-and-error with validation

**Algorithm:**
```
For each square (0-63):
  1. Calculate relevant occupancy mask
  2. Generate random sparse 64-bit number
  3. Test if it creates unique hashes for all blocker patterns
  4. If valid, save it; otherwise, try another random number
```

---

## cmd/generate_test_data

### Purpose

Generates comprehensive test data files used by the chess engine's test suite. This ensures that after code changes (especially to the coordinate system or attack generation), all test data matches the current implementation.

### Location

```
cmd/generate_test_data/main.go
```

### Usage

```bash
cd /home/runner/work/gochess/gochess
go run cmd/generate_test_data/main.go
```

### What It Does

This utility generates 10 different test data files by querying the current chess engine implementation:

#### 1. Pawn Attack Tables
**Files:**
- `data/white_pawn_attacks.csv`
- `data/black_pawn_attacks.csv`

**Content:** Pre-computed diagonal attack patterns for pawns
- White pawns attack diagonally upward (northeast/northwest)
- Black pawns attack diagonally downward (southeast/southwest)
- 64 entries (one per square)

#### 2. Pawn Move Tables
**Files:**
- `data/white_pawn_move.csv`
- `data/black_pawn_move.csv`

**Content:** Pre-computed forward movement patterns
- Single-step moves (always)
- Double-step moves (from starting rank only)
- 64 entries per color

#### 3. Knight Attack Table
**File:** `data/knight_attacks.csv`

**Content:** Pre-computed L-shaped move patterns
- Knights move in an L-shape: 2 squares in one direction, 1 square perpendicular
- Not affected by blockers (knights jump over pieces)
- 64 entries

#### 4. King Attack Table
**File:** `data/king_attacks.csv`

**Content:** Pre-computed one-square moves in all directions
- Kings move one square in any direction (8 possibilities from center)
- Not affected by blockers (only immediate squares)
- 64 entries

#### 5. Raycast Test Data
**File:** `data/raycast_tests.csv`

**Content:** Test cases for the RayCast function
- Tests rook movement with various blocker configurations
- Tests bishop movement with various blocker configurations
- Includes corner cases, edges, and center positions
- Format: `name,piece_type,piece_square,fen_blockers,expected_squares`

**Example test cases:**
- `rook_center_no_blockers` - Rook at d4 with empty board
- `rook_center_one_blocker_up` - Rook at d4 with blocker above
- `bishop_center_no_blockers` - Bishop at d4 with empty board

#### 6. FEN Test Data
**File:** `data/FEN.csv`

**Content:** Standard chess positions in FEN notation
- Starting position
- Common opening positions
- Empty board
- Used to test FEN parsing and generation

**Example positions:**
```
rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1
rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1
```

#### 7. Magic Number Files (via generate_magic)
**Files:**
- `data/rook_magic.csv`
- `data/bishop_magic.csv`

**Note:** This utility also calls the magic number generation functions to regenerate these files.

### Output Files Summary

| File | Purpose | Entries |
|------|---------|---------|
| `white_pawn_attacks.csv` | White pawn diagonal captures | 64 |
| `black_pawn_attacks.csv` | Black pawn diagonal captures | 64 |
| `white_pawn_move.csv` | White pawn forward moves | 64 |
| `black_pawn_move.csv` | Black pawn forward moves | 64 |
| `knight_attacks.csv` | Knight L-shaped moves | 64 |
| `king_attacks.csv` | King one-square moves | 64 |
| `raycast_tests.csv` | RayCast function test cases | ~19 |
| `FEN.csv` | Standard chess positions | ~7 |
| `rook_magic.csv` | Rook magic numbers | 64 |
| `bishop_magic.csv` | Bishop magic numbers | 64 |

### Sample Output

```
Generating test data files...
Generated pawn attacks files
Generated pawn moves files
Generated knight attacks file
Generated king attacks file
Generated raycast tests file
Generated FEN tests file
Generating Rook Magic Numbers...
Finding magic for rook square 0...
  Found: 0xAC000D00928000 (bits: 15)
...
Rook magics saved successfully!
Generating Bishop Magic Numbers...
...
Bishop magics saved successfully!

All test data files generated successfully!
```

### When to Use

**Critical scenarios:**
- **After coordinate system changes** - When the algebraic notation mapping changes (e.g., reversing file order)
- **After RayCast modifications** - When attack generation logic changes
- **After FEN parsing changes** - When position encoding/decoding is modified
- **When test data is missing** - To regenerate all test files from scratch
- **Before major releases** - To ensure test data matches the current implementation

**Example from repository history:**
When the COLUMNS array was reversed from `['h','g','f','e','d','c','b','a']` to `['a','b','c','d','e','f','g','h']`, this utility was used to regenerate all position-dependent test data to match the new coordinate system.

### Performance

- **Simple attack tables** (pawns, knights, kings): < 1 second
- **Raycast test generation**: < 1 second
- **FEN test generation**: < 1 second
- **Magic number generation**: ~15-40 seconds
- **Total**: ~20-45 seconds

### Technical Details

**How it works:**
1. Initializes the chess engine with `BuildAllAttacks()`
2. For each square (0-63), queries the engine for attack patterns
3. Writes the results to CSV files
4. Generates test cases using the current RayCast implementation
5. Ensures test data is self-consistent with the engine's behavior

**Why regeneration is important:**
- Test data must match the engine's actual behavior
- Changes to internal representations (bitboards, coordinates) require updated test data
- Prevents false test failures due to stale test data
- Ensures tests validate current implementation, not historical behavior

---

## Common Workflows

### Workflow 1: Clean Regeneration

Start fresh and regenerate all data files:

```bash
# Remove old data files
rm data/*.csv

# Regenerate magic numbers
go run cmd/generate_magic/main.go

# Regenerate all test data (includes magic numbers again)
go run cmd/generate_test_data/main.go

# Run tests to verify
go test ./chess -v
```

### Workflow 2: After Coordinate System Change

When you modify how positions are represented:

```bash
# Regenerate all test data to match new coordinates
go run cmd/generate_test_data/main.go

# Verify all tests pass
go test ./chess -v
```

### Workflow 3: Magic Number Experimentation

When tweaking the magic bitboard algorithm:

```bash
# Regenerate just the magic numbers
go run cmd/generate_magic/main.go

# Test the new magic numbers
go test ./chess -run TestBuildRookAttacks -v
go test ./chess -run TestBuildBishopAttacks -v
```

### Workflow 4: Adding New Test Cases

When adding new test scenarios:

1. Edit `cmd/generate_test_data/main.go`
2. Add new test cases to the appropriate function
3. Run the generator:
   ```bash
   go run cmd/generate_test_data/main.go
   ```
4. Verify the new test data:
   ```bash
   go test ./chess -v
   ```

---

## Understanding the Data Files

### CSV Format Examples

**Simple Attack Table (knight_attacks.csv):**
```csv
,start,move
0,1,132096
1,2,329728
2,4,659712
```
- Column 1: Square index (0-63)
- Column 2: Starting position as bitboard (1 << square)
- Column 3: Attack pattern as bitboard

**Magic Numbers (rook_magic.csv):**
```csv
square,mask,magic,index_bits
0,282578800148862,47988619603894272,15
1,565157600297596,5926591906570944512,15
```
- `square`: Position (0-63)
- `mask`: Relevant occupancy mask (which squares matter)
- `magic`: The magic number for hashing
- `index_bits`: Size of the attack table (2^index_bits entries)

**Raycast Tests (raycast_tests.csv):**
```csv
name,piece_type,piece_square,fen_blockers,expected_squares
rook_center_no_blockers,rook,d4,8/8/8/8/8/8/8/8,"a4,b4,c4,d1,d2,d3,d5,d6,d7,d8,e4,f4,g4,h4"
```
- `name`: Test case identifier
- `piece_type`: "rook" or "bishop"
- `piece_square`: Algebraic notation (e.g., "d4")
- `fen_blockers`: Board position with blockers in FEN format
- `expected_squares`: Comma-separated list of squares the piece can attack

---

## Troubleshooting

### Problem: Magic number generation takes forever

**Cause:** Unlucky random search, or validation logic has a bug

**Solution:**
1. Cancel with Ctrl+C
2. Run again (different random seed)
3. If persistent, check the validation functions in `generate_magics.go`

### Problem: Generated test data doesn't match expected values

**Cause:** The engine implementation has changed

**Solution:** This is expected! The generator creates test data that matches the current implementation. If tests fail after regeneration, the implementation likely has a bug, not the test data.

### Problem: File permission errors

**Cause:** `data/` directory doesn't exist or isn't writable

**Solution:**
```bash
mkdir -p data
chmod u+w data
```

### Problem: Import errors when running

**Cause:** Dependencies not installed

**Solution:**
```bash
go mod download
go mod tidy
```

---

## Architecture Integration

### How the Utilities Fit In

```
Development Workflow:
  
  1. Modify chess engine code
     └─> chess/BitBoard.go
     └─> chess/Magic.go
     └─> chess/Move.go
     
  2. Regenerate data files (if needed)
     └─> go run cmd/generate_test_data/main.go
     
  3. Run tests
     └─> go test ./chess -v
     
  4. Tests load pre-computed data
     └─> data/*.csv files
     
  5. Tests validate engine behavior
```

### Data Flow

```
cmd/generate_magic/main.go
  └─> chess.GenerateRookMagics()
  └─> chess.GenerateBishopMagics()
  └─> Saves to data/*.csv
      
chess/Magic.go (at runtime)
  └─> chess.BuildAllAttacks()
  └─> Loads from data/*.csv
  └─> Populates attack tables in memory
  
User code
  └─> chess.GetRookAttack(pos, blockers)
  └─> O(1) lookup in pre-computed tables
```

---

## Best Practices

### When to Regenerate

**Always regenerate when:**
- Changing coordinate system or bitboard layout
- Modifying attack generation algorithms
- Changing FEN parsing logic
- Updating magic bitboard implementation

**Don't regenerate when:**
- Adding unrelated features (like UCI protocol)
- Modifying evaluation functions
- Changing search algorithms
- Updating UI code

### Version Control

**Do commit:**
- Changes to `cmd/generate_magic/main.go`
- Changes to `cmd/generate_test_data/main.go`
- Updated `data/*.csv` files (they're part of the tests)

**Don't commit:**
- Temporary test output
- Backup files (`*.bak`, etc.)
- Build artifacts

### Performance Tips

**For faster magic number generation:**
- The search is random; try running multiple times in parallel
- Consider using pre-computed magic numbers from chess programming literature
- Implement smarter search strategies (though random works fine)

---

## Additional Resources

### Chess Programming References

- [Chess Programming Wiki - Magic Bitboards](https://www.chessprogramming.org/Magic_Bitboards)
  - Detailed explanation of the technique
  - Known good magic numbers for reference
  
- [Magic Number Generation](https://www.chessprogramming.org/Looking_for_Magics)
  - Algorithms for finding magic numbers
  - Optimization techniques

### Related Engine Code

- `chess/generate_magics.go` - Magic number generation functions
- `chess/Magic.go` - Attack generation and lookup
- `chess/Utils.go` - CSV loading utilities
- `chess/Magic_test.go` - Comprehensive test suite

---

## Conclusion

These command-line utilities are essential development tools for the GoChess engine. They automate the tedious process of generating pre-computed data, ensuring that test data stays synchronized with the engine implementation.

**Key takeaways:**
- `generate_magic` creates magic numbers for fast attack generation
- `generate_test_data` creates comprehensive test data files
- Both utilities should be run after significant changes to core algorithms
- The generated CSV files are critical for engine operation and testing

By understanding these utilities, you can maintain and extend the GoChess engine with confidence that your test data accurately reflects the current implementation.
