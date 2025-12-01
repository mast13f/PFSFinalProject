package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all simulation parameters
type Config struct {
	// Disease parameters
	diseaseName          string
	transmissionRate     float64
	transmissionDistance float64
	recoveryRate         float64
	mortalityRate        float64
	latentPeriod         int
	infectiousPeriod     int
	immunityDuration     int

	// Population parameters
	popSize         int
	initialInfected int

	// Environment parameters
	areaSize                float64
	socialDistanceThreshold float64
	hygieneLevel            float64
	mobilityRate            float64
	vaccinationRate         float64
	medicalCareLevel        float64
	medicalCapacity         int // if 0, will be calculated as 10% of popSize

	// Simulation parameters
	numDays int

	// Visualization parameters
	canvasWidth    int
	pointRadius    float64
	frameFrequency int
	gifDelay       int
	gifFilename    string
}

func getDefaultConfig() *Config {
	return &Config{
		// Disease defaults
		diseaseName:          "DemoDisease",
		transmissionRate:     0.8,
		transmissionDistance: 2.0,
		recoveryRate:         0.05,
		mortalityRate:        0.01,
		latentPeriod:         3,
		infectiousPeriod:     10,
		immunityDuration:     90,

		// Population defaults
		popSize:         1000,
		initialInfected: 10,

		// Environment defaults
		areaSize:                100.0,
		socialDistanceThreshold: 2.0,
		hygieneLevel:            0.1,
		mobilityRate:            1.0,
		vaccinationRate:         0.20,
		medicalCareLevel:        0.7,
		medicalCapacity:         0, // will be calculated

		// Simulation defaults
		numDays: 200,

		// Visualization defaults
		canvasWidth:    800,
		pointRadius:    3.0,
		frameFrequency: 2,
		gifDelay:       5,
		gifFilename:    "env_sim.gif",
	}
}

func loadConfigFromFile(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := getDefaultConfig()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			fmt.Printf("Warning: skipping invalid line %d: %s\n", lineNum, line)
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Parse based on key
		switch key {
		// Disease parameters
		case "diseaseName":
			config.diseaseName = value
		case "transmissionRate":
			config.transmissionRate, _ = strconv.ParseFloat(value, 64)
		case "transmissionDistance":
			config.transmissionDistance, _ = strconv.ParseFloat(value, 64)
		case "recoveryRate":
			config.recoveryRate, _ = strconv.ParseFloat(value, 64)
		case "mortalityRate":
			config.mortalityRate, _ = strconv.ParseFloat(value, 64)
		case "latentPeriod":
			config.latentPeriod, _ = strconv.Atoi(value)
		case "infectiousPeriod":
			config.infectiousPeriod, _ = strconv.Atoi(value)
		case "immunityDuration":
			config.immunityDuration, _ = strconv.Atoi(value)

		// Population parameters
		case "popSize":
			config.popSize, _ = strconv.Atoi(value)
		case "initialInfected":
			config.initialInfected, _ = strconv.Atoi(value)

		// Environment parameters
		case "areaSize":
			config.areaSize, _ = strconv.ParseFloat(value, 64)
		case "socialDistanceThreshold":
			config.socialDistanceThreshold, _ = strconv.ParseFloat(value, 64)
		case "hygieneLevel":
			config.hygieneLevel, _ = strconv.ParseFloat(value, 64)
		case "mobilityRate":
			config.mobilityRate, _ = strconv.ParseFloat(value, 64)
		case "vaccinationRate":
			config.vaccinationRate, _ = strconv.ParseFloat(value, 64)
		case "medicalCareLevel":
			config.medicalCareLevel, _ = strconv.ParseFloat(value, 64)
		case "medicalCapacity":
			config.medicalCapacity, _ = strconv.Atoi(value)

		// Simulation parameters
		case "numDays":
			config.numDays, _ = strconv.Atoi(value)

		// Visualization parameters
		case "canvasWidth":
			config.canvasWidth, _ = strconv.Atoi(value)
		case "pointRadius":
			config.pointRadius, _ = strconv.ParseFloat(value, 64)
		case "frameFrequency":
			config.frameFrequency, _ = strconv.Atoi(value)
		case "gifDelay":
			config.gifDelay, _ = strconv.Atoi(value)
		case "gifFilename":
			config.gifFilename = value

		default:
			fmt.Printf("Warning: unknown parameter '%s' on line %d\n", key, lineNum)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Calculate medical capacity if not set
	if config.medicalCapacity == 0 {
		config.medicalCapacity = int(0.1 * float64(config.popSize))
	}

	return config, nil
}

func main() {
	configFile := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	var config *Config

	if *configFile != "" {
		var err error
		config, err = loadConfigFromFile(*configFile)
		if err != nil {
			fmt.Printf("Error loading config file: %v\n", err)
			return
		}
		fmt.Printf("Loaded configuration from: %s\n", *configFile)
	} else {
		config = getDefaultConfig()
		// If medicalCapacity is not specified, set it to 10% of population size.
		config.medicalCapacity = int(0.1 * float64(config.popSize))
	}

	disease := initializeDisease(
		config.diseaseName,
		config.transmissionRate,
		config.transmissionDistance,
		config.recoveryRate,
		config.mortalityRate,
		config.latentPeriod,
		config.infectiousPeriod,
		config.immunityDuration,
	)

	globalRng := rand.New(rand.NewSource(time.Now().UnixNano()))

	env := initializeEnvironment(
		config.popSize,
		config.areaSize,
		config.socialDistanceThreshold,
		config.hygieneLevel,
		config.mobilityRate,
		config.vaccinationRate,
		config.medicalCareLevel,
		config.medicalCapacity,
	)

	// Two types of frames: spatial distribution and pie chart
	var framesSpatial []image.Image
	var framesPie []image.Image

	fmt.Printf("Day, Healthy, Susceptible, Infected, Recovered, Dead, InfectedFrac, Vaccinated, EnvHygiene, EnvVaxRate, SDThreshold, PolicyTightened\n")

	attachDiseaseToAll(env, disease)
	for i := 0; i < config.initialInfected; i++ {
		infectOneRandom(env, disease)
	}

	// Day 0 statistics + Day 0 frames
	printStats(0, env, false)
	framesSpatial = append(framesSpatial, env.DrawToCanvas(config.canvasWidth, config.pointRadius))
	framesPie = append(framesPie, DrawEnvironmentPie(env, config.canvasWidth))

	for day := 1; day <= config.numDays; day++ {
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

		// Add both spatial and pie frames every frameFrequency steps
		if day%config.frameFrequency == 0 {
			framesSpatial = append(framesSpatial, env.DrawToCanvas(config.canvasWidth, config.pointRadius))
			framesPie = append(framesPie, DrawEnvironmentPie(env, config.canvasWidth))
		}
	}

	// 1) Save spatial distribution GIF
	if err := SaveEnvironmentGIF(config.gifFilename, framesSpatial, config.gifDelay); err != nil {
		fmt.Println("failed to save spatial gif:", err)
	} else {
		fmt.Println("Spatial GIF saved to:", config.gifFilename)
	}

	// 2) Save pie chart GIF (prefix the filename)
	pieName := "pie_" + config.gifFilename
	if err := SaveEnvironmentGIF(pieName, framesPie, config.gifDelay); err != nil {
		fmt.Println("failed to save pie gif:", err)
	} else {
		fmt.Println("Pie GIF saved to:", pieName)
	}
}
