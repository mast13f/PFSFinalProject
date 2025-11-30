package main

import "math/rand"

// initialize disease function
// takes input of Disease field and returns a pointer
// Once disease is initialized, it cannot be changed
func initializeDisease(name string, transmissionRate, transmissionDistance, recoveryRate, mortalityRate float64, latentPeriod, infectiousPeriod, immunityDuration int) *Disease {
	return &Disease{
		name:                 name,
		transmissionRate:     transmissionRate,
		transmissionDistance: transmissionDistance,
		recoveryRate:         recoveryRate,
		mortalityRate:        mortalityRate,
		latentPeriod:         latentPeriod,
		infectiousPeriod:     infectiousPeriod,
		immunityDuration:     immunityDuration,
	}
}

// initialize enviornment function
// takes input of enviornment field and returns a pointer
// This will be used throughout the model, contains information of every individual and enviornment
// The enviornment is like the 'world' for the model
func initializeEnvironment(
	popSize int,
	areaSize float64,
	initialInfected int,
	socialDistanceThreshold float64,
	hygieneLevel float64,
	mobilityRate float64,
	vaccinationRate float64,
	medicalCareLevel float64,
	medicalCapacity int,
	disease *Disease,
) *Environment {

	env := &Environment{
		population:              make([]*Individual, popSize),
		areaSize:                areaSize,
		socialDistanceThreshold: socialDistanceThreshold,
		hygieneLevel:            hygieneLevel,
		mobilityRate:            mobilityRate,
		vaccinationRate:         vaccinationRate,
		medicalCareLevel:        medicalCareLevel,
		medicalCapacity:         medicalCapacity,
	}

	// Fill population with initialized individuals
	for i := 0; i < popSize; i++ {
		person := initializeIndividual(env)
		env.population[i] = person
	}

	applyInitialInfections(env, initialInfected, disease)

	return env
}

// initialize individual function
// randomly initialize individual, all its fields are randomized
func initializeIndividual(env *Environment) *Individual {
	// Random position in the map
	pos := OrderedPair{
		x: rand.Float64() * env.areaSize,
		y: rand.Float64() * env.areaSize,
	}

	// Random age 0–90 for now
	age := rand.Intn(91)

	// Hygiene + distancing compliance (0–1)
	hygiene := rand.Float64()
	socialDistance := rand.Float64()

	//Everyone starts healthy
	//Apply initial infection later
	health := Healthy

	// Random movement type at initialization
	var mt moveType
	switch r := rand.Float64(); {
	case r < 0.01:
		mt = Flight
	case r < 0.05:
		mt = Train
	default:
		mt = Walk
	}

	movement := NewMovementPattern(mt, env)

	return &Individual{
		gender:                   randomGender(),
		age:                      age,
		healthStatus:             health,
		disease:                  nil,
		daysInfected:             0,
		daysSinceRecovery:        0,
		daysSinceVacination:      0,
		vaccinated:               false,
		hygieneLevel:             hygiene,
		socialDistanceCompliance: socialDistance,
		movementPattern:          movement,
		position:                 pos,
		inHospital:               false,
	}
}

// A simple helper for gender assignment
func randomGender() string {
	if rand.Intn(2) == 0 {
		return "Male"
	}
	return "Female"
}

// apply initial infections will infected individual at random
// number of total infected will be initialInfected(int)
// if initialInfected>populationSize, then everyone is infected
func applyInitialInfections(env *Environment, initialInfected int, disease *Disease) {
	popSize := len(env.population)

	if initialInfected >= popSize {
		initialInfected = popSize
	}

	// Create shuffled list of indexes to infect at random
	indexes := rand.Perm(popSize)

	for i := 0; i < initialInfected; i++ {
		person := env.population[indexes[i]]
		person.healthStatus = Infected
		person.disease = disease
		person.daysInfected = 0
		person.inHospital = true
	}
}
