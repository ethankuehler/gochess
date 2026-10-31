package chess

import (
	"encoding/csv"
	"log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Chdir("..")
	BuildAllAttacks()
	os.Exit(m.Run())
}

func TestParseFEN(t *testing.T) {
	file, err := os.Open("data/FEN.csv")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV file: %v", err)
	}

	for _, record := range records {
		var b, err = ParseFEN(record[0])
		if err != nil {
			t.Errorf("ERROR: could not decode FEN %s", record[0])
			continue
		}
		out := b.FEN()
		if out != record[0] {
			t.Errorf("FEN does not match, input = %s, output = %s", record[0], out)
		}
	}

}

func TestOccupancy(t *testing.T) {
	// Open the CSV file
	file, err := os.Open("data/FEN.csv")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// Read all the records from the CSV
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV file: %v", err)
	}

	// Loop through and print each record
	for _, record := range records {
		var b, err = ParseFEN(record[0])
		if err != nil {
			t.Errorf("ERROR: could not decode FEN %s", record[0])
			continue
		}
		occupied_white := b.Occupancy(White)
		var comp_white BitBoard = 0
		for i := Pawn; i <= King; i++ {
			comp_white |= b.PieceBitboard(White, Piece(i))
		}
		if occupied_white != comp_white {
			t.Errorf("ERROR: White Occupancy %b, comp %b", occupied_white, comp_white)
		}

		occupied_black := b.Occupancy(Black)
		var comp_black BitBoard
		comp_black = 0
		for i := Pawn; i <= King; i++ {
			comp_black |= b.PieceBitboard(Black, Piece(i))
		}
		if occupied_black != comp_black {
			t.Errorf("ERROR: Black Occupancy %b, comp %b", occupied_black, comp_black)
		}

		occupied_both := b.Occupancy(Both)
		if occupied_both != (comp_white | comp_black) {
			t.Errorf("ERROR: Both Occupancy %b, comp %b", occupied_black, comp_black)
		}

	}

}

func TestSquaresToBitBoard(t *testing.T) {
	t.Run("converts square list", func(t *testing.T) {
		got, err := SquaresToBitBoard([]string{"a1", "d4", "h8"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		a1, _ := BitBoardFromAlgebraic("a1")
		d4, _ := BitBoardFromAlgebraic("d4")
		h8, _ := BitBoardFromAlgebraic("h8")
		want := a1 | d4 | h8
		if got != want {
			t.Fatalf("expected %d, got %d", want, got)
		}
	})

	t.Run("handles duplicates", func(t *testing.T) {
		got, err := SquaresToBitBoard([]string{"a1", "a1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want, _ := BitBoardFromAlgebraic("a1")
		if got != want {
			t.Fatalf("expected %d, got %d", want, got)
		}
	})

	t.Run("empty input returns empty board", func(t *testing.T) {
		got, err := SquaresToBitBoard([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 0 {
			t.Fatalf("expected empty board, got %d", got)
		}
	})

	t.Run("invalid square returns error", func(t *testing.T) {
		_, err := SquaresToBitBoard([]string{"a1", "z9"})
		if err == nil {
			t.Fatal("expected error for invalid square")
		}
	})
}

func TestNewStartingPosition(t *testing.T) {
	const want = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	if got := NewStartingPosition().FEN(); got != want {
		t.Fatalf("default board FEN = %q, want %q", got, want)
	}
}

func TestPieceSymbolsFollowStandardOrder(t *testing.T) {
	want := []string{"P", "N", "B", "R", "Q", "K", "p", "n", "b", "r", "q", "k"}
	wantUnicode := []string{"♙", "♘", "♗", "♖", "♕", "♔", "♟", "♞", "♝", "♜", "♛", "♚"}

	if len(pieceSymbols) != len(want) {
		t.Fatalf("pieceSymbols length = %d, want %d", len(pieceSymbols), len(want))
	}
	for i := range want {
		if pieceSymbols[i] != want[i] {
			t.Errorf("pieceSymbols[%d] = %q, want %q", i, pieceSymbols[i], want[i])
		}
		if unicodePieceSymbols[i] != wantUnicode[i] {
			t.Errorf("unicodePieceSymbols[%d] = %q, want %q", i, unicodePieceSymbols[i], wantUnicode[i])
		}
	}
	if pieceSymbols[Knight] != "N" || pieceSymbols[Bishop] != "B" {
		t.Errorf("Knight/Bishop symbols are swapped: N=%q B=%q", pieceSymbols[Knight], pieceSymbols[Bishop])
	}
}

func TestParseFENValid(t *testing.T) {
	cases := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"4k3/8/8/8/8/8/8/4K3 w - - 99 100",
		"4k3/8/8/8/8/8/8/4K3 b - e3 0 1",
		"4k3/8/8/8/8/8/8/4K3 w - e6 12 34",
	}

	for _, fen := range cases {
		b, err := ParseFEN(fen)
		if err != nil {
			t.Errorf("ParseFEN(%q) returned error: %v", fen, err)
			continue
		}
		if got := b.FEN(); got != fen {
			t.Errorf("FEN round-trip: input %q, output %q", fen, got)
		}
	}
}

func TestParseFENEnPassantSquare(t *testing.T) {
	b, err := ParseFEN("4k3/8/8/8/8/8/8/4K3 b - e3 0 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, _ := BitBoardFromAlgebraic("e3")
	if b.enPassantSquare != want {
		t.Errorf("en passant square = %d, want %d", b.enPassantSquare, want)
	}
}

func TestParseFENRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		fen  string
	}{
		{"empty", ""},
		{"missing fields", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0"},
		{"extra fields", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1 extra"},
		{"seven ranks", "rnbqkbnr/pppppppp/8/8/8/8/RNBQKBNR w KQkq - 0 1"},
		{"nine ranks", "rnbqkbnr/pppppppp/8/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{"short rank", "rnbqkbnr/ppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{"long rank", "rnbqkbnrr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{"invalid piece", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNx w KQkq - 0 1"},
		{"digit nine", "rnbqkbnr/pppppppp/9/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{"digit zero", "rnbqkbnr/pppppppp/08/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{"no white king", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQ1BNR w KQkq - 0 1"},
		{"no black king", "rnbq1bnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{"two white kings", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNK w KQkq - 0 1"},
		{"nine white pawns", "rnbqkbnr/pppppppp/P7/8/8/8/PPPPPPPP/1NBQKBNR w KQkq - 0 1"},
		{"too many white pieces", "4k3/8/Q7/8/8/8/PPPPPPPP/QQQQQQQK w - - 0 1"},
		{"pawn on first rank", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPP1/RNBQKBNP w KQkq - 0 1"},
		{"pawn on last rank", "rnbqkbnP/ppppppp1/8/8/8/8/PPPPPPP1/RNBQKBNR w KQkq - 0 1"},
		{"invalid turn", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR x KQkq - 0 1"},
		{"invalid castling", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkx - 0 1"},
		{"duplicate castling", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KK - 0 1"},
		{"short en passant", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq e 0 1"},
		{"bad en passant", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq e9 0 1"},
		{"en passant wrong rank", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq e4 0 1"},
		{"negative halfmove", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - -1 1"},
		{"bad halfmove", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - x 1"},
		{"halfmove overflow", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 65536 1"},
		{"zero fullmove", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 0"},
		{"negative fullmove", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 -1"},
		{"bad fullmove", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 x"},
		{"fullmove overflow", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 65536"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseFEN(tc.fen); err == nil {
				t.Errorf("ParseFEN(%q) succeeded, want an error", tc.fen)
			}
		})
	}
}
