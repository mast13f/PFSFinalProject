package main

import (
	"fmt"
	"image"
	"math/rand"
	"time"
)

func main() {

	disease := initializeDisease(
		"DemoDisease",
		0.8,  // transmissionRate
		2.0,  // transmissionDistance
		0.05, // recoveryRate
		0.01, // mortalityRate
		3,    // latentPeriod (unused yet, but kept for completeness)
		10,   // infectiousPeriod
		90,   // immunityDuration (days)
	)

	globalRng := rand.New(rand.NewSource(time.Now().UnixNano()))

	const (
		popSize            = 1000
		areaSize           = 100.0
		numDays            = 200
		initialInfected    = 10
		initialVaccRate    = 0.20 //
		initialHygiene     = 0.1
		initialSDThreshold = 2.0 //
		mobilityRate       = 1.0 //
		medicalCareLevel   = 0.7 //
	)

	medicalCapacity := int(0.1 * float64(popSize))

	env := initializeEnvironment(
		popSize,
		areaSize,
		initialSDThreshold,
		initialHygiene,
		mobilityRate,
		initialVaccRate,
		medicalCareLevel,
		medicalCapacity,
	)

	const (
		canvasWidth    = 800 // 800x800
		pointRadius    = 3.0 //
		frameFrequency = 2   //
		gifDelay       = 5   // GIF framedelay
		gifFilename    = "env_sim.gif"
	)

	var frames []image.Image

	fmt.Printf("Day, Healthy, Susceptible, Infected, Recovered, Dead, InfectedFrac, Vaccinated, EnvHygiene, EnvVaxRate, SDThreshold, PolicyTightened\n")

	attachDiseaseToAll(env, disease)
	for i := 0; i < initialInfected; i++ {
		infectOneRandom(env, disease)
	}

	printStats(0, env, false)

	for day := 1; day <= numDays; day++ {
		if err := UpdatePopulationHealthStatus(env, globalRng); err != nil {
			fmt.Printf("error in UpdatePopulationHealthStatus on day %d: %v\n", day, err)
			return
		}

		infFrac, tightened, err := UpdateEnvironment(env, globalRng)
		if err != nil {
			fmt.Printf("error in UpdateEnvironment on day %d: %v\n", day, err)
			return
		}

		for _, ind := range env.population {
			if ind == nil || ind.healthStatus == Dead {
				continue
			}
			ind.updateMove(env)
		}

		_ = infFrac
		printStats(day, env, tightened)
		if day%frameFrequency == 0 {
			frames = append(frames, env.DrawToCanvas(canvasWidth, pointRadius))
		}
	}
	if err := SaveEnvironmentGIF(gifFilename, frames, gifDelay); err != nil {
		fmt.Println("failed to save gif:", err)
	} else {
		fmt.Println("GIF saved to:", gifFilename)
	}

}
