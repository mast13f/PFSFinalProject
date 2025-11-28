package main

import (
	"math/rand"
)

func infectOneRandom(env *Environment, dis *Disease) {
	n := len(env.population)
	if n == 0 {
		return
	}

	for {
		idx := rand.Intn(n)
		ind := env.population[idx]
		if ind == nil {
			continue
		}
		if ind.healthStatus == Dead || ind.healthStatus == Infected {
			continue
		}

		ind.healthStatus = Infected
		ind.daysInfected = 0
		ind.disease = dis
		break
	}
}

func attachDiseaseToAll(env *Environment, dis *Disease) {
	for _, ind := range env.population {
		if ind == nil {
			continue
		}
		ind.disease = dis
	}
}
