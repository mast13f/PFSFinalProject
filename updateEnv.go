
package main

import (
	"errors"
	"math"
	"math/rand"
	"time"
)




// UpdateEnvironment performs a full environment-level update for one timestep.
// It computes population statistics, updates policy-level social distance threshold,
// updates environmental hygiene level, and synchronizes the environment vaccination rate.
// Returns:
//   infectedFraction: current fraction of infected individuals (0..1)
//   tightened: whether social distance policy was tightened during this update
//   err: error, if any
func UpdateEnvironment(env *Environment, rng *rand.Rand) (float64, bool, error) {
	if env == nil || len(env.population) == 0 {
		return 0, false, errors.New("empty environment or population")
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// 1) Compute population-level statistics
	infectedFraction, _, totalVaccinated, popHygieneMean, avgTransDist, popSize :=
		ComputePopulationStats(env)

	// 2) Update social distance threshold (policy decision)
	tightened, err := updateSocialDistanceThreshold(env, infectedFraction, avgTransDist)
	if err != nil {
		return infectedFraction, tightened, err
	}

	// 3) Update environment-level hygiene
	_, err = updateEnvHygieneLevel(env, popHygieneMean, infectedFraction, rng)
	if err != nil {
		return infectedFraction, tightened, err
	}

	// 4) Update environment-level vaccination rate (smooth real coverage)
	_, err = updateEnvVaccinationRate(env, totalVaccinated, popSize)
	if err != nil {
		return infectedFraction, tightened, err
	}

	return infectedFraction, tightened, nil
}


// ComputePopulationStats computes useful aggregated statistics from the population.
// Returns:
//   infectedFraction: fraction infected (0..1)
//   totalInfected: integer infected count
//   totalVaccinated: integer vaccinated count
//   popHygieneMean: mean of individual's hygieneLevel (0..1)
//   avgTransDist: average transmissionDistance among individuals with disease info (fallback 1.0)
//   n: population size (count of non-nil individuals)
func ComputePopulationStats(env *Environment) (infectedFraction float64, totalInfected int, totalVaccinated int, popHygieneMean float64, avgTransDist float64, n int) {
	if env == nil || len(env.population) == 0 {
		return 0, 0, 0, 0, 1.0, 0
	}
	sumHyg := 0.0
	sumTrans := 0.0
	transCount := 0
	total := 0
	infected := 0
	vaccinated := 0

	for _, ind := range env.population {
		if ind == nil {
			continue
		}
		total++
		if ind.healthStatus == Infected {
			infected++
		}
		if ind.vaccinated {
			vaccinated++
		}
		sumHyg += clamp01(ind.hygieneLevel)
		if ind.disease != nil && ind.disease.transmissionDistance > 0 {
			sumTrans += ind.disease.transmissionDistance
			transCount++
		}
	}

	popHyg := 0.0
	if total > 0 {
		popHyg = sumHyg / float64(total)
	}
	avgTD := 1.0
	if transCount > 0 {
		avgTD = sumTrans / float64(transCount)
	}
	infFrac := 0.0
	if total > 0 {
		infFrac = float64(infected) / float64(total)
	}

	return infFrac, infected, vaccinated, popHyg, avgTD, total
}

// updateSocialDistanceThreshold updates env.socialDistanceThreshold based on
// infectedFraction and avgTransDist. It applies smoothing and overload tightening.
// Returns true if the policy was effectively tightened (candidate < previous).
func updateSocialDistanceThreshold(env *Environment, infectedFraction float64, avgTransDist float64) (bool, error) {
	if env == nil {
		return false, errors.New("nil environment")
	}
	if avgTransDist <= 0 {
		avgTransDist = 1.0
	}

	// Decide factor buckets driven by infection prevalence
	var factor float64
	switch {
	case infectedFraction < 0.01:
		factor = 2.0
	case infectedFraction < 0.05:
		factor = 1.5
	case infectedFraction < 0.10:
		factor = 1.0
	case infectedFraction < 0.20:
		factor = 0.7
	default:
		factor = 0.4
	}

	// If medical capacity overloaded, tighten further
	if env.medicalCapacity > 0 && infectedFraction > 0 {
		// approximate infected count from fraction and capacity
		approxInfected := int(math.Round(infectedFraction * float64(len(env.population))))
		if approxInfected > env.medicalCapacity {
			ratio := float64(approxInfected-env.medicalCapacity) / float64(env.medicalCapacity)
			// convert overload to multiplier in (0.75,1]
			extraTighten := 1.0 - clamp01(ratio*0.25)
			factor *= extraTighten
		}
	}

	// candidate threshold derived from avgTransDist scaled by factor
	candidate := avgTransDist * factor

	// smoothing (exponential) to avoid policy oscillations
	alpha := 0.25
	prev := env.socialDistanceThreshold
	if prev <= 0 {
		env.socialDistanceThreshold = candidate
	} else {
		env.socialDistanceThreshold = prev*(1.0-alpha) + candidate*alpha
	}

	// enforce reasonable bounds relative to avgTransDist
	minThresh := 0.1 * avgTransDist
	if minThresh < 0.1 {
		minThresh = 0.1
	}
	maxThresh := 4.0 * avgTransDist
	env.socialDistanceThreshold = math.Max(env.socialDistanceThreshold, minThresh)
	env.socialDistanceThreshold = math.Min(env.socialDistanceThreshold, maxThresh)

	// tightened if candidate is less than previous (before smoothing)
	tightened := false
	if prev > 0 && candidate < prev {
		tightened = true
	}
	return tightened, nil
}

// updateEnvHygieneLevel updates env.hygieneLevel using population mean hygiene,
// infection-driven campaign boost, and small stochasticity. The function returns the new value.
func updateEnvHygieneLevel(env *Environment, popHygieneMean float64, infectedFraction float64, rng *rand.Rand) (float64, error) {
	if env == nil {
		return 0, errors.New("nil environment")
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// blending weight for population mean
	beta := 0.35

	// campaign base grows with infection prevalence
	campaignBase := 0.0
	if infectedFraction > 0.4 {
		campaignBase = clamp01(infectedFraction * 1.5)
	}

	// policyStrictness derived from current socialDistanceThreshold relative to avg cap
	// If env.socialDistanceThreshold is small => strict policy => higher push for hygiene
	// Use a safe denominator (if zero, treat as permissive)
	ref := 1.0
	if env.socialDistanceThreshold > 0 {
		ref = env.socialDistanceThreshold
	}
	// map to [0,1]
	policyStrictness := clamp01(1.0 - (env.socialDistanceThreshold/(4.0*ref)))
	policyBoost := 0.2 * policyStrictness

	campaignBoost := clamp01(campaignBase*0.6 + policyBoost*0.4)

	// public noise to model variation
	publicNoise := (rng.Float64()*2 - 1) * 0.02

	newVal := env.hygieneLevel*(1.0-beta) + popHygieneMean*beta + campaignBoost + publicNoise
	env.hygieneLevel = clamp01(newVal)
	return env.hygieneLevel, nil
}

// updateEnvVaccinationRate dynamically updates env vaccination rate
// based on population actual coverage and initial rate, with smooth adjustment
func updateEnvVaccinationRate(env *Environment, totalVaccinated int, populationSize int) (float64, error) {
	if env == nil {
		return 0, errors.New("nil environment")
	}
	if populationSize <= 0 {
		return 0, errors.New("invalid population size")
	}

	// Actual coverage in population
	actualRate := float64(totalVaccinated) / float64(populationSize)

	// Smooth update: combine current env rate and actual population coverage
	smoothK := 0.3 // lower = slower adjustment
	env.vaccinationRate = env.vaccinationRate*(1.0-smoothK) + actualRate*smoothK

	// Clamp within [0,1]
	env.vaccinationRate = clamp01(env.vaccinationRate)

	return env.vaccinationRate, nil
}
