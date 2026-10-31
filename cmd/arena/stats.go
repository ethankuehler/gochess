package main

import (
	"math"
	"math/rand/v2"
	"sort"
)

func eloDiff(score float64) float64 {
	switch {
	case score <= 0:
		return math.Inf(-1)
	case score >= 1:
		return math.Inf(1)
	}
	return -400 * math.Log10(1/score-1)
}

func score(wins, draws, losses int) float64 {
	n := wins + draws + losses
	if n == 0 {
		return 0
	}
	return (float64(wins) + 0.5*float64(draws)) / float64(n)
}

// bootstrapElo resamples pairs to preserve colour-reversed game dependence.
func bootstrapElo(pairScores []float64, seed uint64) (elo, low, high float64) {
	if len(pairScores) == 0 {
		return 0, 0, 0
	}
	var total float64
	for _, s := range pairScores {
		total += s
	}
	elo = eloDiff(total / float64(len(pairScores)) / 2)

	rng := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	const draws = 10000
	samples := make([]float64, draws)
	for i := range samples {
		var sum float64
		for range pairScores {
			sum += pairScores[rng.IntN(len(pairScores))]
		}
		samples[i] = eloDiff(sum / float64(len(pairScores)) / 2)
	}
	sort.Float64s(samples)
	low = samples[draws*25/1000]
	high = samples[draws*975/1000-1]
	return elo, low, high
}
