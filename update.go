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

func UpdatePopulationHealthStatus(env *Environment, rng *rand.Rand) error {
	if env == nil || len(env.population) == 0 {
		return errors.New("empty environment or population")
	}
	rng = rngOrDefault(rng)

	// calculate total infected count for overload consideration
	infectedTotal := 0
	for _, p := range env.population {
		if p != nil && p.healthStatus == Infected {
			infectedTotal++
		}
	}

	type probs struct {
		a, b, c, d, e float64
	}
	ps := make([]probs, len(env.population))

	// calculate params
	for i, ind := range env.population {
		if ind == nil {
			ps[i] = probs{}
			continue
		}
		var a, b, c float64
		switch ind.healthStatus {
		case Healthy:
			a = computeA(env, ind)
		case Susceptible:
			b = computeB(env, ind)
		case Infected:
			c = computeC(env, ind, infectedTotal)
		}
		// Pei you can start from here to add d,e
		case Recovered:
			
		ps[i] = probs{a: a, b: b, c: c, d: d, e: e}
	}

	// update statuses
	for i, ind := range env.population {
		if ind == nil {
			continue
		}
		_ = UpdateIndividualHealthStatus(ind, ps[i].a, ps[i].b, ps[i].c, ps[i].d, ps[i].e, rng)
	}

	return nil
}


func UpdateIndividualHealthStatus(ind *Individual, a, b, c, d, e float64, rng *rand.Rand) error {
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


// find infected neighbors within radius r
func infectedNeighbors(env *Environment, who *Individual, r float64) []struct {
	infected *Individual
	d        float64
} {
	out := make([]struct {
		infected *Individual
		d        float64
	}, 0)
	for _, other := range env.population {
		if other == nil || other == who || other.healthStatus != Infected {
			continue
		}
		if d := dist(who.position, other.position); d <= r {
			out = append(out, struct {
				infected *Individual
				d        float64
			}{infected: other, d: d})
		}
	}
	return out
}

// ---------------- A/B/C/D/E calculation ----------------

// A: Healthy→Susceptible Trigger condition:
// "As long as" the distance to any infected individual < socialDistanceThreshold 
// and environmental hygiene < 0.5
// Here, "as long as" is implemented as: if the condition is met, then a=1; otherwise a=0.
// For better interpretability, social distancing compliance is also included: 
// higher compliance increases the effective threshold, making people more dispersed 
// and thus reducing the probability of being affected.
func computeA(env *Environment, ind *Individual) float64 {
	if ind == nil || ind.healthStatus != Healthy {
		return 0
	}
	// Base threshold: Prefer using the value provided by the environment; 
	// otherwise, fall back to half of the disease transmission distance
	R := env.socialDistanceThreshold
	if R <= 0 && ind.disease != nil && ind.disease.transmissionDistance > 0 {
		R = 0.5 * ind.disease.transmissionDistance
	}
	if R <= 0 {
		R = 1.0 
	}
	// Let high compliance effectively reduce the probability of "close contact": scale by (1 - compliance*0.6)
	Reff := R * (1 - 0.6*clamp01(env.socialDistancingCompliance))

	neighbors := infectedNeighbors(env, ind, Reff)
	if len(neighbors) > 0 && env.hygieneLevel < 0.5 {
		return 1.0
	}
	return 0.0
}

// B: Susceptible→Infected
// Basis: transmissionRate, distance to each infected individual, vaccination.
// Multiple exposure sources use independent failure stacking: P(infection) = 1 - Π(1 - p_i)
// Distance decay uses exp(-d / D0), where D0 = transmissionDistance (interpretable, monotonic)
// Vaccination: use environment coverage or individual flag to reduce effective transmission rate.
func computeB(env *Environment, ind *Individual) float64 {
	if ind == nil || ind.healthStatus != Susceptible || ind.disease == nil {
		return 0
	}
	D0 := ind.disease.transmissionDistance
	if D0 <= 0 {
		D0 = 1.0
	}
	baseBeta := clamp01(ind.disease.transmissionRate)

	// Vaccine effect (simple linear reduction): if individual field exists, use it; 
	// otherwise use environment coverage as expectation
	vaxFactor := 1.0
	if ind.vaccinated {
		vaxFactor = 0.5 // Vaccinated individuals halve the risk (can be adjusted/refined)
	} else {
		// Population-level average protection: reduce exposure risk according to coverage rate
		vaxFactor = 1.0 - 0.5*clamp01(env.vaccinationCoverage)
	}

	// Social hygiene/compliance further reduces effective contact
	hygieneFactor := 1.0 - 0.4*clamp01(env.hygieneLevel)
	complianceFactor := 1.0 - 0.4*clamp01(env.socialDistancingCompliance)

	neighbors := infectedNeighbors(env, ind, 3*D0) // Influence radius is 3*D0
	fail := 1.0
	for _, nb := range neighbors {
		decay := math.Exp(-nb.d / D0) // The closer the distance, the closer the value is to 1
		pi := baseBeta * decay * vaxFactor * hygieneFactor * complianceFactor
		pi = clamp01(pi)
		fail *= (1 - pi)
	}
	return clamp01(1 - fail)

	c := base * ageMult * overloadMult * careFactor
	return clamp01(c)
}
//
// C: Infected→(death/recover/remain infected) Here we only calculate "death probability c"
// Basis: base mortality, age, overload (infectedTotal > capacity), medical care level.
// Interpretable approach:
//   c = baseMort * ageMult * overloadMult * (1 - 0.6*careLevel)
// Where ageMult: <40:0.6, 40-60:1.0, >60:1.6 (example)
// overloadMult: not overloaded=1, overloaded is linearly amplified by ratio (1 + overloadRatio)
func computeC(env *Environment, ind *Individual, infectedTotal int) float64 {
	if ind == nil || ind.healthStatus != Infected || ind.disease == nil {
		return 0
	}

	base := clamp01(ind.disease.mortalityRate)

	// Age adjustment (example, can be replaced with a finer curve)
	ageMult := 1.0
	switch {
	case ind.age < 40:
		ageMult = 0.6
	case ind.age <= 60:
		ageMult = 1.0
	default:
		ageMult = 1.6
	}

	// Overload adjustment
	overloadMult := 1.0
	if env.medicalCapacity > 0 && infectedTotal > env.medicalCapacity {
		ratio := float64(infectedTotal-env.medicalCapacity) / float64(env.medicalCapacity)
		if ratio < 0 {
			ratio = 0
		}
		overloadMult = 1.0 + ratio // Linear amplification
	}

	// Medical care level adjustment (the higher, the lower the mortality)
	careLevel := clamp01(env.medicalCareLevel)
	careFactor := 1.0 - 0.6*careLevel

	c := base * ageMult * overloadMult * careFactor
	return clamp01(c)
}

