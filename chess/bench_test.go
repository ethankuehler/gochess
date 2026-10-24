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

func BenchmarkPerftStartPosition(b *testing.B) {
	BuildAllAttacks()
	position, err := ParseFEN(benchmarkFEN)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	var nodes uint64
	for i := 0; i < b.N; i++ {
		nodes = position.Perft(4)
	}
	b.ReportMetric(float64(nodes), "nodes/op")
	if elapsed := b.Elapsed().Seconds(); elapsed > 0 {
		b.ReportMetric(float64(nodes)*float64(b.N)/elapsed, "nodes/s")
	}
}
