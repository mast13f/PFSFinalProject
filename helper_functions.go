package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// 0-1 clamp
func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// random number generator helper
func rngOrDefault(rng *rand.Rand) *rand.Rand {
	if rng != nil {
		return rng
	}
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}

// calculate distance between two positions
func dist(a, b OrderedPair) float64 {
	dx := a.x - b.x
	dy := a.y - b.y
	return math.Sqrt(dx*dx + dy*dy)
}

func printStats(day int, env *Environment, tightened bool) {
	infFrac, totalInfected, totalVaccinated, _, _, n := ComputePopulationStats(env)

	healthyCount := 0
	susceptibleCount := 0
	recoveredCount := 0
	deadCount := 0

	for _, ind := range env.population {
		if ind == nil {
			continue
		}
		switch ind.healthStatus {
		case Healthy:
			healthyCount++
		case Susceptible:
			susceptibleCount++
		case Infected:
			// already counted in totalInfected, but we keep per-status counts for clarity
		case Recovered:
			recoveredCount++
		case Dead:
			deadCount++
		}
	}

	fmt.Printf(
		"%d, %d, %d, %d, %d, %d, %.4f, %d, %.3f, %.3f, %.3f, %v\n",
		day,
		healthyCount,
		susceptibleCount,
		totalInfected,
		recoveredCount,
		deadCount,
		infFrac,
		totalVaccinated,
		env.hygieneLevel,
		env.vaccinationRate,
		env.socialDistanceThreshold,
		tightened,
	)

	_ = n
}
