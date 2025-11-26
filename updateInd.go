package main

import (
	"errors"
	"math"
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

// UpdatePopulationHealthStatus updates the health status of the whole population for one time step.
// Behavior:
//  - RNG is defaulted if nil.
//  - Vaccination rollout (environment-level) is performed once per generation.
//  - Probabilities a,b,c,d,e are computed for each individual using computeA..computeE.
//  - Each individual's health status is updated by calling UpdateIndividualHealthStatus(env, ind, ...).
func UpdatePopulationHealthStatus(env *Environment, rng *rand.Rand) error {
	if env == nil || len(env.population) == 0 {
		return errors.New("empty environment or population")
	}
	rng = rngOrDefault(rng)

	// 1) Perform environment-level vaccination rollout once per generation.
	//    This avoids repeatedly attempting rollout for each individual.
	_, _ = UpdateVaccination(env, rng)

	// 2) Calculate total infected count for overload consideration.
	infectedTotal := 0
	for _, p := range env.population {
		if p != nil && p.healthStatus == Infected {
			infectedTotal++
		}
	}

	// probs holds computed probabilities for each individual before applying updates.
	type probs struct {
		a, b, c, d, e float64
	}
	ps := make([]probs, len(env.population))

	// 3) Compute transition probabilities for all individuals (read-only phase).
	//    Computing first prevents within-step dependencies caused by ordering.
	for i, ind := range env.population {
		if ind == nil {
			ps[i] = probs{}
			continue
		}
		var a, b, c, d, e float64
		switch ind.healthStatus {
		case Healthy:
			a = computeA(env, ind)
		case Susceptible:
			b = computeB(env, ind)
		case Infected:
			c = computeC(env, ind, infectedTotal)
			d = computeD(env, ind)
		case Recovered:
			e = computeE(env, ind)
		case Dead:
			// no change
		default:
			return errors.New("unknown health status")
		}
		ps[i] = probs{a: a, b: b, c: c, d: d, e: e}
	}

	// 4) Update statuses for each individual using the precomputed probabilities.
	//    Pass env into UpdateIndividualHealthStatus so it can update behavior and timers.
	for i, ind := range env.population {
		if ind == nil {
			continue
		}
		if err := UpdateIndividualHealthStatus(env, ind, ps[i].a, ps[i].b, ps[i].c, ps[i].d, ps[i].e, rng); err != nil {
			// propagate or log error; here we return to make failures explicit
			return err
		}
	}

	return nil
}

// UpdateIndividualHealthStatus updates the health status of a single individual
// using probabilities a,b,c,d,e. In addition, this function integrates behavioral
// updates by calling hygiene, social-distance compliance updates, and advances
// vaccination/recovery timers. env may be nil; if env is provided, additional
// environment-aware updates will be applied.
//
// Note: callers should update to pass env (non-nil) so that social compliance
// and vaccination rollout logic can run.
func UpdateIndividualHealthStatus(env *Environment, ind *Individual, a, b, c, d, e float64, rng *rand.Rand) error {
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

	// ---------------------------
	// Pre-update behavioral hooks
	// ---------------------------
	// Update personal hygiene based on internal rules / stochasticity.
	// This will change ind.hygieneLevel which may affect future computeX values.
	// Assumes updateHygieneLevel(ind *Individual) exists (or variant with env if you used that).
	updateHygieneLevel(env,ind,rng)

	// Update social-distance compliance and adjust movementPattern.
	// If env is nil, skip this step (requires environment context).
	if env != nil {
		// updateSocialDistanceCompliance may return a movement probability which the
		// caller's movement logic can use (ignored here).
		_, _ = updateSocialDistanceCompliance(env, ind, rng)
	}

	// If env is provided, attempt to vaccinate as part of roll-out logic.
	// Note: UpdateVaccination(env, rng) is implemented at environment level and
	// should ideally be called once per generation, not per individual. Here we do
	// not call it to avoid repeated rollout. Instead we increment vaccination timers below.
	// If you implemented a per-individual vaccination attempt helper (e.g., tryVaccinateIndividual),
	// you may call it here.

	// --------------------------------
	// Core health-state stochastic update
	// --------------------------------
	prevStatus := ind.healthStatus

	switch prevStatus {
	case Healthy:
		if draw(rng) < a {
			ind.healthStatus = Susceptible
			// reset recovery counter (not relevant now)
			ind.daysSinceRecovery = 0
		}
	case Susceptible:
		if draw(rng) < b {
			ind.healthStatus = Infected
			ind.daysInfected = 0 // reset counter on becoming infected
			// when infected, daysSinceRecovery should reset
			ind.daysSinceRecovery = 0
		} else {
			ind.healthStatus = Healthy
			// if recovered before and moved to Susceptible, keep daysSinceRecovery as-is
		}
	case Infected:
		r := draw(rng)
		if r < c {
			ind.healthStatus = Dead
			// death: freeze counters
		} else if r < c+d {
			ind.healthStatus = Recovered
			ind.daysSinceRecovery = 0
			// clear infection counter as they've recovered
			ind.daysInfected = 0
		} else {
			// remains infected: increment infection duration
			ind.daysInfected++
		}
	case Recovered:
		if draw(rng) < e {
			ind.healthStatus = Healthy
			ind.daysSinceRecovery = 0
		} else {
			// remain recovered -> increment days since recovery
			ind.daysSinceRecovery++
		}
	case Dead:
		// no change; counters remain as-is
	default:
		return errors.New("unknown health status")
	}

	// -----------------------
	// Post-update bookkeeping
	// -----------------------

	// Increment vaccination timer if vaccinated
	if ind.vaccinated {
		ind.daysSinceVacination++ // note: uses existing field name; consider renaming to daysSinceVaccination
	}

	// If individual was recovered and now healthy (immunity lost), keep daysSinceRecovery=0 as already set.
	// If recovered and remained recovered, we already incremented.

	// If the individual just became infected this step, daysInfected was already set to 0 above.
	// If they remained infected we incremented daysInfected above.

	// If individual is dead, no further behavioral updates should occur.
	if ind.healthStatus == Dead {
		return nil
	}

	// Optionally: if env is provided and you want to perform per-individual vaccination attempts
	// (instead of environment-level rollout), you can call a helper here. For now we do not
	// attempt per-individual vaccination to avoid double-counting global rollout logic.

	return nil
}


// helper function to validate probability values and draw random float
func validProb(p float64) bool    { return p >= 0.0 && p <= 1.0 }
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
// computeA: Healthy -> Susceptible
// Added vaccination time-based protection (initial protection period + waning period)
// Returns probability in [0,1].
func computeA(env *Environment, ind *Individual) float64 {
	if ind == nil || ind.healthStatus != Healthy {
		return 0
	}

	// Base threshold: environment-level distancing requirement
	R := env.socialDistanceThreshold
	if R <= 0 && ind.disease != nil && ind.disease.transmissionDistance > 0 {
		R = 0.5 * ind.disease.transmissionDistance
	}
	if R <= 0 {
		R = 1.0
	}

	// Individual compliance reduces effective contact distance
	compliance := clamp01(ind.socialDistanceCompliance)
	Reff := R * (1 - 0.6*compliance)

	neighbors := infectedNeighbors(env, ind, Reff)

	// If no infectious neighbors or hygiene is high, infection trigger is unlikely
	if len(neighbors) == 0 || env.hygieneLevel >= 0.5 {
		return 0.0
	}

	// ===============================
	// Vaccination time-based immunity
	// ===============================
	// delayDays: days of strong protection after vaccination
	delayDays := 30.0
	// waningDays: days required for protection to decrease from 100% to 0%
	waningDays := 180.0

	protection := 0.0
	if ind.vaccinated {
		d := float64(ind.daysSinceVacination)
		if d <= delayDays {
			// Full protection during initial period
			protection = 1.0
		} else {
			// Linear waning after initial protection period
			decayProgress := (d - delayDays) / waningDays
			if decayProgress < 0 {
				decayProgress = 0
			}
			if decayProgress > 1 {
				decayProgress = 1
			}
			protection = 1.0 - decayProgress
		}
		protection = clamp01(protection)
	}

	// Effective susceptibility after accounting for protection
	effectiveSusceptibility := 1.0 - protection

	// Final probability that the individual becomes susceptible
	return clamp01(effectiveSusceptibility)
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
		// Vaccinated individuals halve the risk (can be adjusted/refined)
		vaxFactor = 0.5
	} else {
		// Population-level average protection: reduce exposure risk according to coverage rate
		vaxFactor = 1.0 - 0.5*clamp01(env.vaccinationRate)
	}

	// Social hygiene reduces effective contact
	hygieneFactor := 1.0 - 0.4*clamp01(env.hygieneLevel)

	// Social distancing compliance reduces effective contact rate
	compliance := clamp01(ind.socialDistanceCompliance)
	complianceFactor := 1.0 - 0.4*compliance

	neighbors := infectedNeighbors(env, ind, 3*D0) // Influence radius is 3*D0
	fail := 1.0
	for _, nb := range neighbors {
		// The closer the distance, the closer the value is to 1
		decay := math.Exp(-nb.d / D0)
		pi := baseBeta * decay * vaxFactor * hygieneFactor * complianceFactor
		pi = clamp01(pi)
		fail *= (1 - pi)
	}
	return clamp01(1 - fail)
}


// C: Infected→(death/recover/remain infected) Here we only calculate "death probability c"
// Basis: base mortality, age, overload (infectedTotal > capacity), medical care level, vaccinationStatus
// Interpretable approach:
//
//	c = baseMort * ageMult * overloadMult * (1 - 0.6*careLevel)
//
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

	// Vaccination adjustment:
	// Use only individual vaccination status (no population-level coverage info)
	vaxFactor := 1.0
	if ind.vaccinated {
		// vaccinated individuals have substantially reduced mortality risk
		vaxFactor = 0.5
	}

	c := base * ageMult * overloadMult * careFactor * vaxFactor
	return clamp01(c)
}

// D: Infected→Recovered
// Basis: base recovery rate, age, medical care level, days infected.
// Interpretable approach:
//
//	d = baseRec * ageMult * (1 + 0.5*careLevel)
//
// Where ageMult: <40:1.4, 40-60:1.0, >60:0.7 (example)
func computeD(env *Environment, ind *Individual) float64 {
	if ind == nil || ind.healthStatus != Infected || ind.disease == nil {
		return 0
	}

	baseRec := clamp01(ind.disease.recoveryRate)
	// Age adjustment (example, can be replaced with a finer curve)
	ageMult := 1.0
	switch {
	case ind.age < 40:
		ageMult = 1.4
	case ind.age <= 60:
		ageMult = 1.0
	default:
		ageMult = 0.7
	}

	// Medical care level adjustment (the higher, the higher the recovery rate)
	careLevel := clamp01(env.medicalCareLevel)
	careFactor := 1.0 + 0.5*careLevel

	// Days infected adjustment: longer infection duration increases recovery chance
	timeFactor := 1.0
	if ind.daysInfected > 0 {
		timeFactor += math.Log(float64(ind.daysInfected)+1) / 10.0 // logarithmic increase
	}

	d := baseRec * ageMult * careFactor * timeFactor
	return clamp01(d)
}

// E: Recovered→Healthy
// Basis: days since recovery, vaccination status (bool)
// Immunity decays over time: modeled as increasing probability with days since recovery
// Vaccination boosts immunity: vaccinated recovered individuals have lower chance of losing immunity
func computeE(env *Environment, ind *Individual) float64 {
	if ind == nil || ind.healthStatus != Recovered || ind.disease == nil {
		return 0
	}

	// Base immunity loss rate: starts low, increases with days since recovery
	baseE := 0.01 // base 1% chance
	if ind.daysSinceRecovery > 0 {
		baseE += math.Log(float64(ind.daysSinceRecovery)+1) / 50.0 // logarithmic increase
	}

	// Vaccination adjustment: vaccinated recovered individuals have reduced immunity loss
	vaxFactor := 1.0
	if ind.vaccinated {
		vaxFactor = 0.5 // halve the chance of losing immunity
	}

	e := baseE * vaxFactor
	return clamp01(e)
}
// neighborsWithin: return neighbors (including non-infected) within radius r
func neighborsWithin(env *Environment, who *Individual, r float64) []*Individual {
	out := make([]*Individual, 0)
	if env == nil || who == nil || r <= 0 {
		return out
	}
	for _, other := range env.population {
		if other == nil || other == who {
			continue
		}
		if d := dist(who.position, other.position); d <= r {
			out = append(out, other)
		}
	}
	return out
}

// updateHygieneLevel updates individual's hygieneLevel (0..1) based on:
// - personal baseline and fatigue (hygiene tends to decay slowly)
// - social influence: neighbors' average hygiene
// - vaccination complacency (vaccinated may reduce hygiene a bit)
// - infection/hospital effects (infected or inHospital -> hygiene increases)
// - stochastic variation
//
// NOTE: env and rng may be nil; if rng is nil a default one will be used.
func updateHygieneLevel(env *Environment, ind *Individual, rng *rand.Rand) error {


	// parameters (tunable)
	fatigueDecay := 0.01      // normal hygiene decay per timestep
	socialInfluenceRadius := 2.0 // neighbor search radius (units consistent with position)
	influenceWeight := 0.35   // average neighbor hygiene weight
	infectionBoost := 0.20    // if infected, hygiene improvement boost
	hospitalBoost := 0.30     // if in hospital, stronger hygiene boost
	vaxComplacency := 0.12    // vaccination complacency max reduction
	randomNoise := 0.03       // random noise amplitude

	// clamp current hygiene into [0,1]
	current := clamp01(ind.hygieneLevel)

	// baseline decay (fatigue or normalization)
	afterDecay := current * (1.0 - fatigueDecay)

	// social influence: neighbors' mean hygiene
	neighbors := neighborsWithin(env, ind, socialInfluenceRadius)
	meanNeighbor := 0.0
	if len(neighbors) > 0 {
		sum := 0.0
		for _, n := range neighbors {
			sum += clamp01(n.hygieneLevel)
		}
		meanNeighbor = sum / float64(len(neighbors))
	} else {
		// if no neighbors, fallback to environment hygiene level as proxy
		meanNeighbor = clamp01(env.hygieneLevel)
	}

	// combine afterDecay and neighbor influence
	combined := afterDecay*(1.0-influenceWeight) + meanNeighbor*influenceWeight

	// behavior modifiers
	if ind.inHospital {
		combined = math.Max(combined, clamp01(current+hospitalBoost))
	} else if ind.healthStatus == Infected {
		// infected individuals tend to improve hygiene
		combined = math.Max(combined, clamp01(current+infectionBoost))
	}

	// vaccination complacency reduces hygiene gradually
	if ind.vaccinated {
		// decay factor: 0..1 over 180 days
		// uniform linear decay over 180 days
		decayProgress := float64(ind.daysSinceVacination) / 180.0
		if decayProgress < 0 {
			decayProgress = 0
		}
		if decayProgress > 1 {
			decayProgress = 1
		}
		complacency := vaxComplacency * decayProgress
		combined = combined * (1.0 - complacency)
	}

	// small random fluctuation to avoid determinism
	noise := (rng.Float64()*2 - 1) * randomNoise // in [-randomNoise, +randomNoise]
	combined = combined + noise

	// final clamp and writeback
	ind.hygieneLevel = clamp01(combined)
	return nil
}

// updateSocialDistanceCompliance updates individual's socialDistanceCompliance (0..1)
// and also adjusts the individual's movementPattern.moveRadius and returns a movementProbability
// (probability that individual will move in this timestep).
//
// Factors considered:
// - local social norms (neighbors' compliance)
// - policy signal via env.socialDistanceThreshold (higher threshold => stronger policy => more compliance)
// - vaccination complacency (vaccinated more likely to reduce compliance over time)
// - movement type (Train/Flight make high compliance harder; Walk easier)
// - infection status/hospitalization (if infected or hospitalized -> compliance increases)
// - random variation
func updateSocialDistanceCompliance(env *Environment, ind *Individual, rng *rand.Rand) (float64, error) {

	// parameters (tunable)
	normRadius := 3.0           // neighborhood radius to estimate social norms
	normWeight := 0.4           // neighbors' compliance weight
	policyWeight := 0.4         // environment policy weight
	vaxComplacencyMax := 0.25   // maximum compliance reduction due to vaccination
	infectionComplianceBoost := 0.35
	hospitalComplianceBoost := 0.6
	randomJitter := 0.04

	// current compliance
	current := clamp01(ind.socialDistanceCompliance)

	// neighbors' mean compliance
	neighbors := neighborsWithin(env, ind, normRadius)
	meanNeighborCompliance := 0.0
	if len(neighbors) > 0 {
		sum := 0.0
		for _, n := range neighbors {
			sum += clamp01(n.socialDistanceCompliance)
		}
		meanNeighborCompliance = sum / float64(len(neighbors))
	} else {
		// fallback to a proxy via environment threshold (scaled into [0,1])
		// larger socialDistanceThreshold implies stronger policy expectation -> higher compliance
		// assume threshold up to some sensible cap (e.g., 10 units). normalize:
		threshold := env.socialDistanceThreshold
		meanNeighborCompliance = clamp01(threshold / 10.0)
	}

	// policy signal: map env.socialDistanceThreshold to [0,1] (tunable mapping)
	policySignal := clamp01(env.socialDistanceThreshold / 10.0)

	// base update: blend neighbors norm and policy
	newCompliance := current*(1.0-normWeight-policyWeight) + meanNeighborCompliance*normWeight + policySignal*policyWeight

	// infection / hospitalation increases compliance
	if ind.inHospital {
		newCompliance = math.Max(newCompliance, clamp01(current+hospitalComplianceBoost))
	} else if ind.healthStatus == Infected {
		newCompliance = math.Max(newCompliance, clamp01(current+infectionComplianceBoost))
	}

	// vaccination complacency reduces compliance gradually (longer since vaccination -> more complacency)
	if ind.vaccinated {
		progress := float64(ind.daysSinceVacination) / 180.0
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}
		complacency := vaxComplacencyMax * progress
		newCompliance = newCompliance * (1.0 - complacency)
	}

	// movement type difficulty: public transport makes high compliance harder
	switch ind.movementPattern.moveType {
	case Train:
		// train makes it ~10% harder to achieve same compliance
		newCompliance *= 0.9
	case Flight:
		// flights make it harder (security/boarding behavior), reduce further
		newCompliance *= 0.8
	case Walk:
		// walking tends to make it easier
		newCompliance = newCompliance*(1.0-0.02) + 0.02
	}

	// small randomness
	newCompliance += (rng.Float64()*2 - 1) * randomJitter

	// clamp and write back
	newCompliance = clamp01(newCompliance)
	ind.socialDistanceCompliance = newCompliance

	// adjust movement radius: higher compliance -> smaller movement radius
	// baseline radius depends on moveType (tunable)
	baseRadius := baselineMoveRadius(ind.movementPattern)
	// effective radius reduced by compliance: more compliance -> multiply by (1 - 0.6*compliance)
	effectiveRadius := baseRadius * (1.0 - 0.6*newCompliance)
	if effectiveRadius < 0.01 {
		effectiveRadius = 0.01
	}
	ind.movementPattern.moveRadius = effectiveRadius

	// movement probability: higher compliance -> less likely to move.
	// map compliance to movementProb in [0.15, 1.0] so even very compliant people still sometimes move
	minMoveProb := 0.15
	moveProb := minMoveProb + (1.0-minMoveProb)*(1.0-newCompliance)
	moveProb = clamp01(moveProb)

	return moveProb, nil
}

// baselineMoveRadius returns a sensible default movement radius for given movementPattern
func baselineMoveRadius(mp *MovementPattern) float64 {
	if mp == nil {
		return 1.0
	}
	switch mp.moveType {
	case Walk:
		// walking: small daily radius (e.g., neighborhood)
		if mp.moveRadius > 0 {
			return mp.moveRadius // if user already set explicit baseline, respect it
		}
		return 1.0
	case Train:
		if mp.moveRadius > 0 {
			return mp.moveRadius
		}
		return 5.0
	case Flight:
		if mp.moveRadius > 0 {
			return mp.moveRadius
		}
		return 200.0
	default:
		if mp.moveRadius > 0 {
			return mp.moveRadius
		}
		return 1.0
	}
}


// UpdateVaccination attempts to vaccinate unvaccinated individuals in the environment.
// Behavior:
// - env.vaccinationRate is treated as the target fraction of population to be vaccinated.
// - We compute desiredTotal = round(env.vaccinationRate * populationSize).
// - available = desiredTotal - currentVaccinated.
// - Vaccinations per generation are capped by maxDaily (default: 2% of population, at least 1).
// - Each unvaccinated individual has an acceptance probability determined by personal factors:
//     baseAcceptance +/- modifiers (age, compliance, hygiene, inHospital).
// - A person can only be vaccinated once; we set vaccinated=true and daysSinceVacination=0.
// Returns the number of newly vaccinated individuals in this update.
func UpdateVaccination(env *Environment, rng *rand.Rand) (int, error) {


	n := len(env.population)

	// Count current vaccinated
	currentVaccinated := 0
	for _, ind := range env.population {
		if ind != nil && ind.vaccinated {
			currentVaccinated++
		}
	}

	// Desired total vaccinated based on env.vaccinationRate (target coverage)
	desiredTotal := int(math.Round(clamp01(env.vaccinationRate) * float64(n)))

	available := desiredTotal - currentVaccinated
	if available <= 0 {
		// target reached or exceeded, nothing to do
		return 0, nil
	}

	// Limit vaccinations per generation to simulate rollout speed.
	// Default: allow up to 2% of population per generation (at least 1).
	maxDaily := int(math.Max(1.0, math.Round(0.02*float64(n))))
	slots := int(math.Min(float64(available), float64(maxDaily)))
	if slots <= 0 {
		return 0, nil
	}

	// To avoid bias, iterate randomized order of indices
	indices := rand.Perm(n)

	newlyVaccinated := 0

	for _, idx := range indices {
		if slots <= 0 {
			break
		}
		ind := env.population[idx]
		if ind == nil {
			continue
		}
		// Skip already vaccinated
		if ind.vaccinated {
			continue
		}

		// -----------------------
		// Individual acceptance model
		// -----------------------
		// Base acceptance probability (tunable)
		baseAcceptance := 0.55 // baseline willingness to vaccinate

		// Age modifier: older people more likely to accept (e.g., >60 +0.2, 40-60 +0.1)
		ageMod := 0.0
		if ind.age >= 60 {
			ageMod = 0.20
		} else if ind.age >= 40 {
			ageMod = 0.10
		}

		// Social distance compliance correlates with risk-averse behavior -> more likely to vaccinate
		complMod := 0.15 * clamp01(ind.socialDistanceCompliance) // up to +0.15

		// Hygiene level: higher hygiene might slightly correlate with willingness (small effect)
		hygieneMod := 0.08 * clamp01(ind.hygieneLevel)

		// In-hospital or currently infected: more likely to accept vaccination when available
		healthMod := 0.0
		if ind.inHospital {
			healthMod += 0.30
		} else if ind.healthStatus == Infected {
			// infected individuals may be less likely immediately to take vaccine (depends on policy),
			// but we assume modest increase because they may want immunity after recovery.
			healthMod += 0.10
		}

		// Vaccine complacency: people long after vaccination are not relevant (they are unvaccinated here),
		// but we could incorporate community sentiment via env.vaccinationRate (not used here).

		// Combine modifiers
		acceptanceProb := baseAcceptance + ageMod + complMod + hygieneMod + healthMod

		// Clamp to [0,1]
		acceptanceProb = clamp01(acceptanceProb)

		// Draw
		if rng.Float64() < acceptanceProb {
			// Vaccinate this person
			ind.vaccinated = true
			ind.daysSinceVacination = 0
			newlyVaccinated++
			slots--
		}
	}

	return newlyVaccinated, nil
}