package main

type Disease struct{
	Name string
	TransmissionRate float64
	RecoveryRate float64
	MortalityRate float64
	LatentPeriod int
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