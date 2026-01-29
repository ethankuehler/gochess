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

### 🚧 In Progress / Incomplete Components

#### 1. Rook Move Generation
- **Status**: PARTIAL - Infrastructure exists, needs completion
- **Current State**:
  - ✅ `GetRookAttack()` function defined and commented
  - ✅ `GetRookMask()` function implemented
  - ✅ `TryRookMagic()` function implemented (validates magic numbers)
  - ✅ `FindMagic()` function implemented (searches for valid magic numbers)
  - ✅ `ROOK_RAY` directions defined: `{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}`
  - ✅ RayCast works correctly for rook movement patterns (tested)
  - ❌ `BuildRookAttacks()` function NOT implemented
  - ❌ Magic numbers NOT generated/loaded
  - ❌ Attack tables NOT populated
  - ❌ CSV data file does NOT exist
  
- **Global Variables Declared**:
  - `ROOK_MAGIC []MagicEntry` (empty)
  - `ROOK_ATTACKS [][]BitBoard` (empty)

#### 2. Bishop Move Generation
- **Status**: PARTIAL - Infrastructure exists, needs completion
- **Current State**:
  - ✅ `GetBishopAttack()` function defined and commented
  - ✅ `BISHOP_RAY` directions defined: `{{1, 1}, {-1, -1}, {1, -1}, {-1, 1}}`
  - ✅ RayCast works correctly for bishop movement patterns (tested)
  - ⚠️ `GetBishopMask()` function declared but returns 0 (TODO comment present)
  - ❌ `BuildBishopAttacks()` function NOT implemented
  - ❌ Magic numbers NOT generated/loaded
  - ❌ Attack tables NOT populated
  - ❌ CSV data file does NOT exist

- **Global Variables Declared**:
  - `BISHOP_MAGIC []MagicEntry` (empty)
  - `BISHOP_ATTACKS [][]BitBoard` (empty)

#### 3. Queen Move Generation
- **Status**: NOT STARTED
- **Current State**:
  - ❌ No functions defined
  - ❌ No global variables declared
  - ❌ No Ray directions defined (should combine rook + bishop rays)
  - ❌ `BuildQueenAttacks()` function NOT implemented
  - ❌ CSV data file does NOT exist

- **Note**: Queens move like rooks and bishops combined, so the implementation can leverage both `GetRookAttack()` and `GetBishopAttack()`.

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

## Implementation Steps

### Phase 1: Bishop Mask Function
**Priority**: HIGH - Required before Bishop magic generation

1. **Implement `GetBishopMask(coord Coordinates) BitBoard`**
   - Location: `Magic.go`
   - Generate diagonal mask for a given square
   - Include all diagonal squares passing through the position
   - Typically exclude edge squares for optimization (common in magic bitboard implementations)
   - Similar to `GetRookMask()` but for diagonals

   **Algorithm**:
   ```go
   func GetBishopMask(coord Coordinates) BitBoard {
       rank, file := coord.rank, coord.file
       var mask BitBoard = 0
       
       // For each of the 4 diagonal directions
       for _, dir := range BISHOP_RAY {
           rankDelta, fileDelta := dir[0], dir[1]
           r, f := int(rank), int(file)
           
           // Move in direction until edge
           for {
               r += rankDelta
               f += fileDelta
               
               // Stop at board edges
               if r < 0 || r >= 8 || f < 0 || f >= 8 {
                   break
               }
               
               // Add square to mask (optionally exclude edges)
               square := Shift(f + r*8)
               mask |= BitBoard(1) << square
           }
       }
       
       return mask
   }
   ```

### Phase 2: Generate Magic Numbers

**Priority**: HIGH - Can be done offline or at runtime

**Option A: Runtime Generation** (slower startup, no CSV needed)
1. Create `GenerateRookMagics() []MagicEntry` function
2. For each square (0-63):
   - Get occupancy mask using `GetRookMask()`
   - Count bits in mask to determine table size
   - Use `FindMagic()` to search for valid magic number
   - Store in ROOK_MAGIC array
3. Similarly, create `GenerateBishopMagics() []MagicEntry`

**Option B: Pre-compute and Store in CSV** (faster startup, recommended)
1. Write utility program to generate magic numbers offline
2. Save to CSV files:
   - `data/rook_magic.csv`: columns = square, mask, magic_number, index_bits
   - `data/bishop_magic.csv`: columns = square, mask, magic_number, index_bits
3. Load at runtime like other attack tables

### Phase 3: Generate and Store Attack Tables

**Priority**: HIGH - Required for move generation

1. **Create `BuildRookAttacks()` function**
   ```go
   func BuildRookAttacks() {
       // Initialize arrays
       ROOK_MAGIC = make([]MagicEntry, 64)
       ROOK_ATTACKS = make([][]BitBoard, 64)
       
       // For each square
       for square := Shift(0); square < 64; square++ {
           coord := CoordsFromShift(square)
           mask := GetRookMask(coord)
           
           // Load or generate magic entry
           magic := loadOrGenerateRookMagic(square, mask)
           ROOK_MAGIC[square] = magic
           
           // Generate all attack patterns for this square
           tableSize := 1 << magic.Index
           ROOK_ATTACKS[square] = make([]BitBoard, tableSize)
           
           // Iterate through all possible blocker configurations
           blockers := BitBoard(0)
           for {
               // Generate attacks for this blocker configuration
               attacks := RayCast(square, blockers, mask, ROOK_RAY)
               
               // Store in table at hashed index
               index := MagicIndex(magic, blockers)
               ROOK_ATTACKS[square][index] = attacks
               
               // Next blocker configuration (Carry-Rippler trick)
               blockers = (blockers - mask) & mask
               if blockers == 0 {
                   break
               }
           }
       }
   }
   ```

2. **Create `BuildBishopAttacks()` function**
   - Similar to `BuildRookAttacks()` but using bishop functions
   - Use `GetBishopMask()`, `BISHOP_RAY`, etc.

3. **Update `BuildAllAttacks()`**
   ```go
   func BuildAllAttacks() {
       BuildKnightAttacks()
       BuildKingAttacks()
       BuildPawnMoves()
       BuildPawnAttacks()
       BuildRookAttacks()    // UNCOMMENT
       BuildBishopAttacks()  // UNCOMMENT
       // BuildQueenAttacks() is optional if using rook+bishop combination
   }
   ```

### Phase 4: Queen Move Generation

**Priority**: MEDIUM - Can be implemented multiple ways

**Option A: Combine Rook + Bishop** (simplest, no new tables needed)
```go
func GetQueenAttack(loc Shift, board BitBoard) BitBoard {
    rookAttacks := GetRookAttack(loc, board)
    bishopAttacks := GetBishopAttack(loc, board)
    return rookAttacks | bishopAttacks
}
```

**Option B: Separate Queen Tables** (uses more memory but simpler lookup)
- Similar to rook/bishop implementation
- QUEEN_RAY would be all 8 directions combined
- Would need separate QUEEN_MAGIC and QUEEN_ATTACKS arrays

**Recommendation**: Use Option A (combine rook + bishop) as it:
- Reuses existing infrastructure
- Saves memory
- Simpler to implement and maintain
- Only slightly slower (one extra bitwise OR operation)

### Phase 5: Integration with Move Generation

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
