# Chess Move Generation Implementation Plan

## Overview
This document outlines the work needed to complete the chess move generation system using magic bitboards. The project uses pre-computed attack tables for simple pieces and magic bitboard hashing for sliding pieces.

## Current Status

### ✅ Completed Components

#### 1. Knight Moves
- **Status**: COMPLETE
- **Implementation**: Pre-computed attack table loaded from CSV
- **File**: `data/knight_attacks.csv` (64 entries)
- **Function**: `BuildKnightAttacks()` in `Magic.go`
- **Global Variable**: `KNIGHT_ATTACKS []BitBoard`
- **Details**: Knights have fixed L-shaped move patterns that don't depend on board occupancy, making them ideal for simple lookup tables.

#### 2. King Moves
- **Status**: COMPLETE
- **Implementation**: Pre-computed attack table loaded from CSV
- **File**: `data/king_attacks.csv` (64 entries)
- **Function**: `BuildKingAttacks()` in `Magic.go`
- **Global Variable**: `KING_ATTACKS []BitBoard`
- **Details**: Kings move one square in any direction. Like knights, these patterns are constant regardless of board occupancy.

#### 3. Pawn Moves
- **Status**: COMPLETE
- **Implementation**: Pre-computed tables for both colors (forward movement)
- **Files**: 
  - `data/white_pawn_move.csv` (64 entries)
  - `data/black_pawn_move.csv` (64 entries)
- **Function**: `BuildPawnMoves()` in `Magic.go`
- **Global Variables**: 
  - `WHITE_PAWN_MOVES []BitBoard`
  - `BLACK_PAWN_MOVES []BitBoard`
- **Details**: Pawns have directional movement (white moves up, black moves down). Tables include single and double-step moves from starting positions.

#### 4. Pawn Attacks
- **Status**: COMPLETE
- **Implementation**: Pre-computed tables for both colors (diagonal captures)
- **Files**:
  - `data/white_pawn_attacks.csv` (64 entries)
  - `data/black_pawn_attacks.csv` (64 entries)
- **Function**: `BuildPawnAttacks()` in `Magic.go`
- **Global Variables**:
  - `WHITE_PAWN_ATTACKS []BitBoard`
  - `BLACK_PAWN_ATTACKS []BitBoard`
- **Details**: Pawn captures are diagonal and directional, different from forward movement.

#### 5. Supporting Infrastructure
- **RayCast Function**: COMPLETE
  - Generates sliding piece attacks by casting rays in specified directions
  - Handles blockers correctly (includes blocker square, stops beyond it)
  - Well-tested with comprehensive test suite in `Magic_test.go`
  - Parameters: starting position, blockers, mask, ray directions
  
- **Coordinate System**: FIXED
  - `Coordinates` struct now uses proper field names: `rank` and `file`
  - All coordinate conversions working correctly
  - `CoordsFromShift()` and `ShiftFromCoords()` functions operational

- **CSV Loading**: COMPLETE
  - `LoadAttacks()` function in `Utils.go` handles CSV parsing
  - Reads 64-entry attack tables efficiently
  - Error handling for file I/O and data validation

### ✅ Completed Components (Updated)

#### 1. Knight Moves
- **Status**: COMPLETE
- **Implementation**: Pre-computed attack table loaded from CSV
- **File**: `data/knight_attacks.csv` (64 entries)
- **Function**: `BuildKnightAttacks()` in `Magic.go`
- **Global Variable**: `KNIGHT_ATTACKS []BitBoard`
- **Details**: Knights have fixed L-shaped move patterns that don't depend on board occupancy, making them ideal for simple lookup tables.

#### 2. King Moves
- **Status**: COMPLETE
- **Implementation**: Pre-computed attack table loaded from CSV
- **File**: `data/king_attacks.csv` (64 entries)
- **Function**: `BuildKingAttacks()` in `Magic.go`
- **Global Variable**: `KING_ATTACKS []BitBoard`
- **Details**: Kings move one square in any direction. Like knights, these patterns are constant regardless of board occupancy.

#### 3. Pawn Moves
- **Status**: COMPLETE
- **Implementation**: Pre-computed tables for both colors (forward movement)
- **Files**: 
  - `data/white_pawn_move.csv` (64 entries)
  - `data/black_pawn_move.csv` (64 entries)
- **Function**: `BuildPawnMoves()` in `Magic.go`
- **Global Variables**: 
  - `WHITE_PAWN_MOVES []BitBoard`
  - `BLACK_PAWN_MOVES []BitBoard`
- **Details**: Pawns have directional movement (white moves up, black moves down). Tables include single and double-step moves from starting positions.

#### 4. Pawn Attacks
- **Status**: COMPLETE
- **Implementation**: Pre-computed tables for both colors (diagonal captures)
- **Files**:
  - `data/white_pawn_attacks.csv` (64 entries)
  - `data/black_pawn_attacks.csv` (64 entries)
- **Function**: `BuildPawnAttacks()` in `Magic.go`
- **Global Variables**:
  - `WHITE_PAWN_ATTACKS []BitBoard`
  - `BLACK_PAWN_ATTACKS []BitBoard`
- **Details**: Pawn captures are diagonal and directional, different from forward movement.

#### 5. Rook Move Generation
- **Status**: ✅ COMPLETE (with flexible loading options)
- **Current State**:
  - ✅ `GetRookAttack()` function implemented and tested
  - ✅ `GetRookMask()` function implemented
  - ✅ `BuildRookAttacks()` function implemented
  - ✅ `BuildRookAttacksWithOption()` function with auto-generation support
  - ✅ `GenerateRookMagics()` function in `generate_magics.go`
  - ✅ `TryRookMagic()` function implemented (validates magic numbers)
  - ✅ `FindMagic()` function implemented (searches for valid magic numbers)
  - ✅ `ROOK_RAY` directions defined: `{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}`
  - ✅ RayCast works correctly for rook movement patterns (tested)
  - ✅ Magic numbers generated and loaded from CSV
  - ✅ Attack tables populated
  - ✅ CSV data file exists: `data/rook_magic.csv`
  - ✅ Command-line tool for regenerating magics: `cmd/generate_magic/main.go`
  
- **Global Variables**:
  - `ROOK_MAGIC []MagicEntry` (loaded from CSV or generated)
  - `ROOK_ATTACKS [][]BitBoard` (populated at runtime)

- **Loading Options**:
  - `BuildRookAttacks()` - Loads from CSV, panics if file doesn't exist
  - `BuildRookAttacksWithOption(true)` - Auto-generates and saves if CSV doesn't exist
  - `cmd/generate_magic` - CLI tool to regenerate magic numbers

#### 6. Bishop Move Generation
- **Status**: ✅ COMPLETE (with flexible loading options)
- **Current State**:
  - ✅ `GetBishopAttack()` function implemented and tested
  - ✅ `GetBishopMask()` function implemented
  - ✅ `BuildBishopAttacks()` function implemented
  - ✅ `BuildBishopAttacksWithOption()` function with auto-generation support
  - ✅ `GenerateBishopMagics()` function in `generate_magics.go`
  - ✅ `BISHOP_RAY` directions defined: `{{1, 1}, {-1, -1}, {1, -1}, {-1, 1}}`
  - ✅ RayCast works correctly for bishop movement patterns (tested)
  - ✅ Magic numbers generated and loaded from CSV
  - ✅ Attack tables populated
  - ✅ CSV data file exists: `data/bishop_magic.csv`
  - ✅ Command-line tool for regenerating magics: `cmd/generate_magic/main.go`

- **Global Variables**:
  - `BISHOP_MAGIC []MagicEntry` (loaded from CSV or generated)
  - `BISHOP_ATTACKS [][]BitBoard` (populated at runtime)

- **Loading Options**:
  - `BuildBishopAttacks()` - Loads from CSV, panics if file doesn't exist
  - `BuildBishopAttacksWithOption(true)` - Auto-generates and saves if CSV doesn't exist
  - `cmd/generate_magic` - CLI tool to regenerate magic numbers

#### 7. Queen Move Generation
- **Status**: ✅ COMPLETE
- **Current State**:
  - ✅ `GetQueenAttack()` function implemented
  - ✅ Combines rook and bishop attacks: `GetRookAttack() | GetBishopAttack()`
  - ✅ Tested and working correctly
  - ❌ No separate CSV needed (leverages rook + bishop tables)
  
- **Details**: Queens move like rooks and bishops combined. The implementation efficiently reuses both attack tables without additional storage.

#### 8. Supporting Infrastructure
- **RayCast Function**: COMPLETE
  - Generates sliding piece attacks by casting rays in specified directions
  - Handles blockers correctly (includes blocker square, stops beyond it)
  - Well-tested with comprehensive test suite in `Magic_test.go`
  - Parameters: starting position, blockers, mask, ray directions
  
- **Coordinate System**: FIXED
  - `Coordinates` struct now uses proper field names: `rank` and `file`
  - All coordinate conversions working correctly
  - `CoordsFromShift()` and `ShiftFromCoords()` functions operational

- **CSV Loading**: COMPLETE
  - `LoadAttacks()` function in `Utils.go` handles CSV parsing
  - `LoadMagicsFromCSV()` in `generate_magics.go` loads magic numbers
  - `SaveRookMagicsToCSV()` and `SaveBishopMagicsToCSV()` for persistence
  - Error handling for file I/O and data validation

### 🚧 Remaining Work

## Magic Bitboard Technique

### Concept
Magic bitboards provide O(1) lookup time for sliding piece attacks by using perfect hashing. Instead of calculating attacks on-the-fly, we:
1. Pre-compute all possible attack patterns for every square and blocker configuration
2. Use a "magic number" to hash blocker configurations to unique indices
3. Store attack patterns in a lookup table indexed by the hash

### How It Works

#### 1. Relevant Occupancy Mask
For each square, determine which squares can affect the piece's movement:
- **Rook**: All squares on the same rank and file
- **Bishop**: All squares on the same diagonals
- Note: Edge squares are typically excluded from the mask (see magic bitboard optimization)

#### 2. Magic Number
A special 64-bit number that, when multiplied with occupancy patterns and shifted, produces unique indices for different blocker configurations. Finding these numbers requires brute-force search.

#### 3. Hash Function
```go
func MagicIndex(entry MagicEntry, board BitBoard) uint64 {
    blockers := board & entry.Mask        // Isolate relevant squares
    hash := uint64(blockers) * entry.Magic // Multiply by magic number
    index := hash >> (64 - entry.Index)    // Shift to get table index
    return index
}
```

#### 4. Attack Table Structure
```go
// For each square (0-63):
ROOK_MAGIC[square] = MagicEntry{
    Mask:  BitBoard,  // Relevant occupancy mask
    Magic: uint64,    // The magic number
    Index: Shift,     // Number of bits in the index (table size = 2^Index)
}

// Attack lookup table (2D array):
ROOK_ATTACKS[square][hash_index] = attack_bitboard
```

### Memory Requirements
- **Rook**: ~800 KB total (varies by square, ~12 bits average per square)
- **Bishop**: ~40 KB total (fewer relevant squares on diagonals, ~9 bits average)
- **Queen**: Can reuse rook + bishop tables OR use combined approach

## Implementation Steps (Completed)

All phases of the magic bitboard implementation have been completed. Here's what was implemented:

### ✅ Phase 1: Bishop Mask Function (COMPLETE)
The `GetBishopMask(coord Coordinates) BitBoard` function has been implemented in `Magic.go`. It generates diagonal masks for bishops at any square, excluding edge squares for optimization as per standard magic bitboard practice.

### ✅ Phase 2: Magic Number Generation (COMPLETE)

**Both options have been implemented:**

**Option A: Runtime Generation**
- `GenerateRookMagics()` in `generate_magics.go` - Generates magic numbers for all 64 rook squares
- `GenerateBishopMagics()` in `generate_magics.go` - Generates magic numbers for all 64 bishop squares
- Uses brute-force random search to find valid magic numbers
- Can be invoked via `BuildRookAttacksWithOption(true)` or `BuildBishopAttacksWithOption(true)`

**Option B: Pre-computed CSV Files** (Recommended for production)
- CSV files exist and are ready to use:
  - `data/rook_magic.csv` - Contains 64 magic entries for rooks
  - `data/bishop_magic.csv` - Contains 64 magic entries for bishops
- Format: `square,mask,magic,index_bits`
- Can be loaded via `BuildRookAttacks()` or `BuildBishopAttacks()`
- Can be regenerated using `cmd/generate_magic/main.go`

### ✅ Phase 3: Attack Table Generation (COMPLETE)

**Implemented Functions:**
1. `BuildRookAttacks()` - Loads from CSV, panics if missing
2. `BuildRookAttacksWithOption(autoGenerate bool)` - Flexible loading with optional auto-generation
3. `BuildBishopAttacks()` - Loads from CSV, panics if missing
4. `BuildBishopAttacksWithOption(autoGenerate bool)` - Flexible loading with optional auto-generation
5. `BuildAllAttacks()` - Initializes all attack tables (loads from CSV)
6. `BuildAllAttacksWithOption(autoGenerate bool)` - Initializes with optional auto-generation

**How It Works:**
- Loads magic numbers from CSV files (or generates if autoGenerate=true)
- For each square, allocates attack table of size 2^(index_bits)
- Iterates through all possible blocker configurations using the Carry-Rippler trick
- Computes attacks using RayCast for each configuration
- Stores attacks in lookup table at index computed by MagicIndex function

### ✅ Phase 4: Queen Attacks (COMPLETE)
The `GetQueenAttack(loc Shift, board BitBoard)` function has been implemented. It efficiently combines rook and bishop attacks: `GetRookAttack(loc, board) | GetBishopAttack(loc, board)`. No separate CSV or magic numbers needed.

## Usage Guide

### For End Users (Normal Usage)
```go
import "github.com/ethankuehler/gochess/chess"

func main() {
    // Load all attack tables from pre-computed CSV files
    chess.BuildAllAttacks()
    
    // Now you can use attack generation
    occupied := chess.BitBoard(0x1234567890ABCDEF)
    rookAttacks := chess.GetRookAttack(chess.Shift(27), occupied)   // d4
    bishopAttacks := chess.GetBishopAttack(chess.Shift(27), occupied)
    queenAttacks := chess.GetQueenAttack(chess.Shift(27), occupied)
}
```

### For Developers (Auto-Generation Option)
```go
import "github.com/ethankuehler/gochess/chess"

func main() {
    // Auto-generate magic numbers if CSV files don't exist
    // (Slower on first run, but convenient for development)
    chess.BuildAllAttacksWithOption(true)
    
    // Use attack generation normally
    attacks := chess.GetRookAttack(chess.Shift(0), 0)
}
```

### For Regenerating Magic Numbers
```bash
# Use the CLI tool to regenerate magic numbers
cd cmd/generate_magic
go run main.go

# This will:
# 1. Generate new magic numbers for rooks and bishops
# 2. Save them to data/rook_magic.csv and data/bishop_magic.csv
# 3. Print progress information
```

## Files

### Core Implementation
- `chess/Magic.go` - Attack lookup functions, table building, main runtime code
- `chess/generate_magics.go` - Magic number generation, CSV I/O, validation functions
- `chess/Const.go` - Constants including ROOK_RAY and BISHOP_RAY definitions
- `chess/Utils.go` - CSV loading utilities for simple attack tables

### Data Files
- `data/rook_magic.csv` - Pre-computed rook magic numbers (64 entries)
- `data/bishop_magic.csv` - Pre-computed bishop magic numbers (64 entries)
- `data/knight_attacks.csv` - Knight attack patterns
- `data/king_attacks.csv` - King attack patterns
- `data/white_pawn_attacks.csv` / `data/black_pawn_attacks.csv` - Pawn attacks
- `data/white_pawn_move.csv` / `data/black_pawn_move.csv` - Pawn moves

### Tools
- `cmd/generate_magic/main.go` - CLI tool for generating/regenerating magic numbers

### Tests
- `chess/Magic_test.go` - Comprehensive test suite including:
  - RayCast tests with various configurations
  - Magic number validation
  - Attack generation verification
  - FEN-based integration tests

## Next Steps (Future Work)

### Phase 5: Integration with Legal Move Generation

**Priority**: HIGH - Final goal of the system

1. **Update `BoardState.LegalMoves()`** in `BitBoard.go`
   - Currently returns dummy array with TODO comment
   - Implement full legal move generation using attack tables
   
2. **Pseudo-legal Move Generation**:
   ```go
   func (b *BoardState) GenerateMoves(colour Colour) []Move {
       var moves []Move
       occupied := b.Occupied(BOTH)
       
       // For each piece type
       for piece := range PiecesIter(colour) {
           positions := b.pieces[piece]
           
           // For each piece of this type
           for positions != 0 {
               // Get least significant bit (piece position)
               square := Shift(bits.TrailingZeros64(uint64(positions)))
               
               // Generate attacks based on piece type
               var attacks BitBoard
               switch piece % 6 { // Use modulo to handle both colors
                   case PAWN:
                       // Handle pawns specially (moves vs attacks, en passant)
                   case KNIGHT:
                       attacks = KNIGHT_ATTACKS[square]
                   case BISHOP:
                       attacks = GetBishopAttack(square, occupied)
                   case ROOK:
                       attacks = GetRookAttack(square, occupied)
                   case QUEEN:
                       attacks = GetQueenAttack(square, occupied)
                   case KING:
                       attacks = KING_ATTACKS[square]
               }
               
               // Remove friendly pieces from attacks
               attacks &= ^b.Occupied(colour)
               
               // Convert attacks to moves
               // ... (iterate through attack bitboard and create Move structs)
               
               // Clear this piece and move to next
               positions &= positions - 1
           }
       }
       
       return moves
   }
   ```

3. **Legal Move Filtering**:
   - Check if move leaves king in check
   - Handle castling legality
   - Validate en passant captures
   - Check for pins and absolute pins

## Testing Strategy

### Unit Tests (Existing)
- ✅ `TestRayCastRookCenter` - Tests rook movement from center
- ✅ `TestRayCastRookWithBlocker` - Tests rook with blockers
- ✅ `TestRayCastBishopCenter` - Tests bishop movement from center
- ✅ `TestRayCastBishopWithBlocker` - Tests bishop with blockers
- ✅ `TestRayCastCorner` - Tests edge cases at corners
- ✅ `TestRayCastBlocked` - Tests fully blocked pieces
- ✅ `TestRayCastFromConfig` - Data-driven tests from CSV (23 test cases)

### Additional Tests Needed

1. **Magic Number Validation**:
   ```go
   func TestRookMagicNumbers(t *testing.T) {
       BuildRookAttacks()
       // Verify no nil entries
       // Test sample positions with various blocker patterns
       // Ensure correct attack generation
   }
   ```

2. **Bishop Mask Tests**:
   ```go
   func TestGetBishopMask(t *testing.T) {
       // Test center squares have full diagonals
       // Test corner squares have single diagonal
       // Test edge squares have limited diagonals
   }
   ```

3. **Queen Attack Tests**:
   ```go
   func TestQueenAttacks(t *testing.T) {
       BuildAllAttacks()
       // Test queen combines rook + bishop movement
       // Test various positions and blocker patterns
   }
   ```

4. **Performance Tests**:
   ```go
   func BenchmarkGetRookAttack(b *testing.B) {
       BuildRookAttacks()
       // Measure lookup performance
   }
   ```

5. **Integration Tests**:
   - Test full move generation from various FEN positions
   - Verify move counts match known positions (perft testing)
   - Test special cases: castling, en passant, promotion

## Data Files Required

### Existing Files ✅
- `data/knight_attacks.csv` (64 entries) - COMPLETE
- `data/king_attacks.csv` (64 entries) - COMPLETE  
- `data/white_pawn_move.csv` (64 entries) - COMPLETE
- `data/black_pawn_move.csv` (64 entries) - COMPLETE
- `data/white_pawn_attacks.csv` (64 entries) - COMPLETE
- `data/black_pawn_attacks.csv` (64 entries) - COMPLETE
- `data/raycast_tests.csv` - Test data for RayCast function - COMPLETE

### Files to Create ⚠️
- `data/rook_magic.csv` - Magic numbers for rooks (64 entries)
  - Columns: square_index, mask, magic_number, index_bits
- `data/bishop_magic.csv` - Magic numbers for bishops (64 entries)
  - Columns: square_index, mask, magic_number, index_bits
- `data/rook_attacks.csv` (optional) - Pre-computed attacks if not generated at runtime
- `data/bishop_attacks.csv` (optional) - Pre-computed attacks if not generated at runtime

**Note**: Attack tables are typically generated at runtime from magic numbers rather than stored in CSV, as they would be very large files.

## Code Quality Improvements Done ✅

1. **Spelling Corrections**:
   - Fixed "pecies" → "pieces"
   - Fixed "piececs" → "pieces"
   - Fixed "ROOK_ATTTACKS" → "ROOK_ATTACKS"
   - Fixed "certin" → "certain"
   - Fixed "Genrates" → "Generates"
   - Fixed "handel" → "handle"
   - Fixed "algerbraic" → "algebraic"

2. **Coordinate System**:
   - Renamed `Coordinates.row` → `Coordinates.rank`
   - Renamed `Coordinates.col` → `Coordinates.file`
   - Updated all usages throughout codebase
   - Removed confusing swap comments

3. **Documentation**:
   - Added comprehensive comments to all major functions in `Magic.go`
   - Documented `MagicEntry` struct fields
   - Added parameter and return value documentation
   - Explained magic bitboard algorithm in comments
   - Added comments to utility functions in `Utils.go`

## Estimated Effort

### Phase 1: Bishop Mask (1-2 hours)
- Implement `GetBishopMask()`
- Write unit tests
- Verify correctness

### Phase 2: Magic Number Generation (2-4 hours)
- Implement or load magic numbers for rooks
- Implement or load magic numbers for bishops
- Create CSV files if using pre-computed approach
- Test validation

### Phase 3: Attack Table Building (2-3 hours)
- Implement `BuildRookAttacks()`
- Implement `BuildBishopAttacks()`
- Test attack generation
- Performance optimization

### Phase 4: Queen Implementation (1 hour)
- Implement `GetQueenAttack()`
- Test combined attack patterns
- Update documentation

### Phase 5: Move Generation Integration (4-8 hours)
- Implement `BoardState.LegalMoves()`
- Handle special moves (castling, en passant, promotion)
- Legal move validation (check detection)
- Comprehensive testing with known positions
- Performance tuning

**Total Estimated Time**: 10-18 hours

## Next Steps (Priority Order)

1. ✅ Fix spelling errors and coordinate naming - DONE
2. ✅ Add function comments - DONE
3. **Implement `GetBishopMask()` function** - NEXT
4. Generate or load magic numbers for rooks
5. Generate or load magic numbers for bishops
6. Implement `BuildRookAttacks()`
7. Implement `BuildBishopAttacks()`
8. Implement `GetQueenAttack()`
9. Test all sliding piece attacks thoroughly
10. Implement `BoardState.LegalMoves()`
11. Add comprehensive integration tests
12. Performance optimization and benchmarking

## References

### Magic Bitboard Resources
- [Chess Programming Wiki - Magic Bitboards](https://www.chessprogramming.org/Magic_Bitboards)
- [Fancy Magic Bitboards](https://www.chessprogramming.org/Magic_Bitboards#Fancy)
- [Plain Magic Bitboards](https://www.chessprogramming.org/Magic_Bitboards#Plain)

### Move Generation
- [Chess Programming Wiki - Move Generation](https://www.chessprogramming.org/Move_Generation)
- [Perft Testing](https://www.chessprogramming.org/Perft) - For validation
- [Starting Position Perft Results](https://www.chessprogramming.org/Perft_Results)

## Notes

- The existing `RayCast()` function is well-implemented and tested - no changes needed
- The magic bitboard infrastructure (MagicEntry, MagicIndex) is already in place
- Consider using a magic number generator library or pre-computed values to save time
- Queen implementation should use the simple combination approach for maintainability
- Future optimization: Consider "fancy magic bitboards" for better memory efficiency
- The test suite is comprehensive for RayCast - maintain this quality for new functions

## Conclusion

The chess move generation system is approximately **50% complete**. The foundation (RayCast, coordinate system, simple piece tables) is solid and well-tested. The remaining work focuses on:

1. Completing the magic bitboard implementation for rooks and bishops
2. Implementing queen attacks (trivial once rook/bishop are done)
3. Integrating everything into the legal move generator

The magic bitboard technique is well-understood and the code structure supports it. With focused effort on the remaining components, the move generation system can be completed efficiently.
