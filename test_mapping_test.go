package main

import (
"fmt"
"gochess/chess"
"testing"
)

func TestMapping(t *testing.T) {
// Test h8 mapping
h8, _ := chess.LocFromAlg("h8")
fmt.Printf("h8 location: %d (binary: %064b)\n", h8, h8)
fmt.Printf("1 << 63: %d (binary: %064b)\n", uint64(1)<<63, uint64(1)<<63)
fmt.Printf("h8 == (1 << 63): %v\n\n", h8 == chess.BitBoard(uint64(1)<<63))

// Test a1 mapping
a1, _ := chess.LocFromAlg("a1")
fmt.Printf("a1 location: %d (binary: %064b)\n", a1, a1)
fmt.Printf("a1 == 1: %v\n\n", a1 == 1)

// Test d4
d4Shift, _ := chess.ShiftFromAlg("d4")
fmt.Printf("d4 shift: %d (3 + 3*8 = %d)\n", d4Shift, 3 + 3*8)
}
