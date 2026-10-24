package chess

import "testing"

const benchmarkFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

func BenchmarkRookAttack(b *testing.B) {
	BuildRookAttacks()
	occupancy := BitBoard(0x00FF00FF00FF00FF)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RookAttack(Square(i%squareCount), occupancy)
	}
}

func BenchmarkBishopAttack(b *testing.B) {
	BuildBishopAttacks()
	occupancy := BitBoard(0x00FF00FF00FF00FF)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BishopAttack(Square(i%squareCount), occupancy)
	}
}

func BenchmarkParseUCIMove(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ParseUCIMove("e7e8q"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFEN(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ParseFEN(benchmarkFEN); err != nil {
			b.Fatal(err)
		}
	}
}
