package main

type Disease struct {
	name                 string
	transmissionRate     float64
	transmissionDistance float64
	recoveryRate         float64
	mortalityRate        float64
	latentPeriod         int
	infectiousPeriod     int
	immunityDuration     int
}

type HealthStatus string

const (
	Healthy     HealthStatus = "Healthy"
	Susceptible HealthStatus = "Susceptible"
	Infected    HealthStatus = "Infected"
	Recovered   HealthStatus = "Recovered"
	Dead        HealthStatus = "Dead"
)

type Individual struct {
	gender                   string
	age                      int
	healthStatus             HealthStatus
	disease                  *Disease
	daysInfected             int
	daysSinceRecovery        int
	daysSinceVacination	     int
	vaccinated               bool
	hygieneLevel             float64
	socialDistanceCompliance float64
	movementPattern          *MovementPattern
	position                 OrderedPair
	inHospital               bool
}

// David u can decide how to structure this
type MovementPattern struct {
	moveType   moveType
	moveRadius float64
}

type moveType string

const (
	Walk   moveType = "Walk"
	Train  moveType = "Train"
	Flight moveType = "Flight"
)

type Environment struct {
	population               []*Individual
	areaSize                 float64
	socialDistanceThreshold  float64
	hygieneLevel             float64
	mobilityRate             float64
	vaccinationRate          float64
	medicalCareLevel         float64
	medicalCapacity          int
}

type OrderedPair struct {
	x float64
	y float64
}
