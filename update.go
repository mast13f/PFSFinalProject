package main

import (
	"errors"
	"math/rand"
	"time"
)

// UpdateHealthStatus applies one step of stochastic state transition.
// Rules:
// - Healthy -> Susceptible with prob a; else stays Healthy
// - Susceptible -> Infected with prob b; else Healthy
// - Infected -> Dead with prob c; else Recovered with prob d; else stays Infected
// - Recovered -> Healthy with prob e; else stays Recovered
// - Dead -> stays Dead
//
// Constraints: 0<=a,b,c,d,e<=1 and c+d<=1.
func UpdateHealthStatus(ind *Individual, a, b, c, d, e float64, rng *rand.Rand) error {
	if ind == nil {
		return errors.New("nil individual")
	}
	if !validProb(a) || !validProb(b) || !validProb(c) || !validProb(d) || !validProb(e) || c+d > 1.0 {
		return errors.New("invalid probabilities: ensure 0<=a,b,c,d,e<=1 and c+d<=1")
	}
	// default RNG if none provided
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	switch ind.healthStatus {
	case Healthy:
		if draw(rng) < a {
			ind.healthStatus = Susceptible
		}
	case Susceptible:
		if draw(rng) < b {
			ind.healthStatus = Infected
			ind.daysInfected = 0 // reset counter on becoming infected
		} else {
			ind.healthStatus = Healthy
		}
	case Infected:
		r := draw(rng)
		if r < c {
			ind.healthStatus = Dead
		} else if r < c+d {
			ind.healthStatus = Recovered
		} else {
			// remains infected
			ind.daysInfected++
		}
	case Recovered:
		if draw(rng) < e {
			ind.healthStatus = Healthy
		}
	case Dead:
		// no change
	default:
		return errors.New("unknown health status")
	}
	return nil
}
// helper function to validate probability values and draw random float
func validProb(p float64) bool { return p >= 0.0 && p <= 1.0 }
func draw(rng *rand.Rand) float64 { return rng.Float64() }



// if within transmission distance range, then use UpdateHealthStatus to determine if transmission occurs
// 
