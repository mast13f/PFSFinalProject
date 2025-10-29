package main

type Disease struct{
	name string
	transmissionRate float64
	transmissionDistance float64
	recoveryRate float64
	mortalityRate float64
	latentPeriod int
	infectiousPeriod int
	immunityDuration int	
}

type Individual struct{
	gender string
	age int
	healthStatus string
	disease *Disease
	daysInfected int
	movementPattern *MovementPattern
	position OrderedPair
}
//David u can decide how to structure this
type MovementPattern struct{

}

type Environment struct{
	population []*Individual
	areaSize float64
	socialDistancingCompliance float64
	hygieneLevel float64
	mobilityRate float64
	vaccinationRate float64
	medicalCapacity int
}

type OrderedPair struct{
	x float64
	y float64
}