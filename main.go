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

// ValidationError represents a configuration validation error
type ValidationError struct {
	Parameter string
	Value     string
	Message   string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("Invalid parameter '%s' with value '%s': %s", e.Parameter, e.Value, e.Message)
}

// ConfigValidator holds validation rules for each parameter
type ConfigValidator struct {
	errors []ValidationError
}

func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{errors: make([]ValidationError, 0)}
}

func (v *ConfigValidator) AddError(param, value, message string) {
	v.errors = append(v.errors, ValidationError{
		Parameter: param,
		Value:     value,
		Message:   message,
	})
}

func (v *ConfigValidator) HasErrors() bool {
	return len(v.errors) > 0
}

func (v *ConfigValidator) PrintErrors() {
	fmt.Println("\n=== Configuration Validation Errors ===")
	for i, err := range v.errors {
		fmt.Printf("%d. %s\n", i+1, err.Error())
	}
	fmt.Println("========================================\n")
}

// parseAndValidateFloat parses a float and validates it's within the given range
func (v *ConfigValidator) parseAndValidateFloat(key, value string, min, max float64, allowEqual bool) (float64, bool) {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		v.AddError(key, value, fmt.Sprintf("must be a valid decimal number (float64), got '%s'", value))
		return 0, false
	}

	if allowEqual {
		if f < min || f > max {
			v.AddError(key, value, fmt.Sprintf("must be between %.2f and %.2f (inclusive), got %.4f", min, max, f))
			return f, false
		}
	} else {
		if f <= min || f >= max {
			v.AddError(key, value, fmt.Sprintf("must be between %.2f and %.2f (exclusive), got %.4f", min, max, f))
			return f, false
		}
	}
	return f, true
}

// parseAndValidateInt parses an integer and validates it's within the given range
func (v *ConfigValidator) parseAndValidateInt(key, value string, min, max int) (int, bool) {
	i, err := strconv.Atoi(value)
	if err != nil {
		v.AddError(key, value, fmt.Sprintf("must be a valid integer, got '%s'", value))
		return 0, false
	}

	if i < min || i > max {
		v.AddError(key, value, fmt.Sprintf("must be between %d and %d (inclusive), got %d", min, max, i))
		return i, false
	}
	return i, true
}

// parseAndValidatePositiveInt parses an integer and validates it's positive
func (v *ConfigValidator) parseAndValidatePositiveInt(key, value string, max int) (int, bool) {
	return v.parseAndValidateInt(key, value, 1, max)
}

// parseAndValidateNonNegativeInt parses an integer and validates it's non-negative
func (v *ConfigValidator) parseAndValidateNonNegativeInt(key, value string, max int) (int, bool) {
	return v.parseAndValidateInt(key, value, 0, max)
}

// parseAndValidateString validates a non-empty string
func (v *ConfigValidator) parseAndValidateString(key, value string, maxLen int) (string, bool) {
	if value == "" {
		v.AddError(key, value, "cannot be empty")
		return "", false
	}
	if len(value) > maxLen {
		v.AddError(key, value, fmt.Sprintf("must be at most %d characters, got %d", maxLen, len(value)))
		return value, false
	}
	return value, true
}

// parseAndValidateFilename validates a filename
func (v *ConfigValidator) parseAndValidateFilename(key, value string) (string, bool) {
	if value == "" {
		v.AddError(key, value, "filename cannot be empty")
		return "", false
	}
	if !strings.HasSuffix(strings.ToLower(value), ".gif") {
		v.AddError(key, value, "filename must end with .gif extension")
		return value, false
	}
	// Check for invalid characters in filename
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		if strings.Contains(value, char) {
			v.AddError(key, value, fmt.Sprintf("filename contains invalid character '%s'", char))
			return value, false
		}
	}
	return value, true
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
	validator := NewConfigValidator()

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

		// Parse and validate based on key
		switch key {
		// Disease parameters
		case "diseaseName":
			if val, ok := validator.parseAndValidateString(key, value, 100); ok {
				config.diseaseName = val
			}

		case "transmissionRate":
			// Probability: 0.0 to 1.0
			if val, ok := validator.parseAndValidateFloat(key, value, 0.0, 1.0, true); ok {
				config.transmissionRate = val
			}

		case "transmissionDistance":
			// Distance: must be positive, reasonable max of 100 units
			if val, ok := validator.parseAndValidateFloat(key, value, 0.0, 100.0, false); ok {
				config.transmissionDistance = val
			}

		case "recoveryRate":
			// Probability per day: 0.0 to 1.0
			if val, ok := validator.parseAndValidateFloat(key, value, 0.0, 1.0, true); ok {
				config.recoveryRate = val
			}

		case "mortalityRate":
			// Probability: 0.0 to 1.0
			if val, ok := validator.parseAndValidateFloat(key, value, 0.0, 1.0, true); ok {
				config.mortalityRate = val
			}

		case "latentPeriod":
			// Days: 0 to 365
			if val, ok := validator.parseAndValidateNonNegativeInt(key, value, 365); ok {
				config.latentPeriod = val
			}

		case "infectiousPeriod":
			// Days: 1 to 365
			if val, ok := validator.parseAndValidatePositiveInt(key, value, 365); ok {
				config.infectiousPeriod = val
			}

		case "immunityDuration":
			// Days: 0 (no immunity) to 3650 (10 years)
			if val, ok := validator.parseAndValidateNonNegativeInt(key, value, 3650); ok {
				config.immunityDuration = val
			}

		// Population parameters
		case "popSize":
			// Population: 1 to 1,000,000
			if val, ok := validator.parseAndValidatePositiveInt(key, value, 1000000); ok {
				config.popSize = val
			}

		case "initialInfected":
			// Initial infected: 0 to popSize (will validate later)
			if val, ok := validator.parseAndValidateNonNegativeInt(key, value, 1000000); ok {
				config.initialInfected = val
			}

		// Environment parameters
		case "areaSize":
			// Area size: must be positive
			if val, ok := validator.parseAndValidateFloat(key, value, 0.0, 10000.0, false); ok {
				config.areaSize = val
			}

		case "socialDistanceThreshold":
			// Distance: 0 to areaSize (reasonable max)
			if val, ok := validator.parseAndValidateFloat(key, value, 0.0, 100.0, true); ok {
				config.socialDistanceThreshold = val
			}

		case "hygieneLevel":
			// Level: 0.0 to 1.0
			if val, ok := validator.parseAndValidateFloat(key, value, 0.0, 1.0, true); ok {
				config.hygieneLevel = val
			}

		case "mobilityRate":
			// Rate: 0.0 to 10.0 (allows higher mobility)
			if val, ok := validator.parseAndValidateFloat(key, value, 0.0, 10.0, true); ok {
				config.mobilityRate = val
			}

		case "vaccinationRate":
			// Rate: 0.0 to 1.0
			if val, ok := validator.parseAndValidateFloat(key, value, 0.0, 1.0, true); ok {
				config.vaccinationRate = val
			}

		case "medicalCareLevel":
			// Level: 0.0 to 1.0
			if val, ok := validator.parseAndValidateFloat(key, value, 0.0, 1.0, true); ok {
				config.medicalCareLevel = val
			}

		case "medicalCapacity":
			// Capacity: 0 (auto-calculate) to popSize
			if val, ok := validator.parseAndValidateNonNegativeInt(key, value, 1000000); ok {
				config.medicalCapacity = val
			}

		// Simulation parameters
		case "numDays":
			// Days: 1 to 10000
			if val, ok := validator.parseAndValidatePositiveInt(key, value, 10000); ok {
				config.numDays = val
			}

		// Visualization parameters
		case "canvasWidth":
			// Width: 100 to 4096 pixels
			if val, ok := validator.parseAndValidateInt(key, value, 100, 4096); ok {
				config.canvasWidth = val
			}

		case "pointRadius":
			// Radius: 0.5 to 50 pixels
			if val, ok := validator.parseAndValidateFloat(key, value, 0.5, 50.0, true); ok {
				config.pointRadius = val
			}

		case "frameFrequency":
			// Frequency: 1 to numDays
			if val, ok := validator.parseAndValidatePositiveInt(key, value, 10000); ok {
				config.frameFrequency = val
			}

		case "gifDelay":
			// Delay: 1 to 1000 (centiseconds)
			if val, ok := validator.parseAndValidatePositiveInt(key, value, 1000); ok {
				config.gifDelay = val
			}

		case "gifFilename":
			if val, ok := validator.parseAndValidateFilename(key, value); ok {
				config.gifFilename = val
			}

		default:
			fmt.Printf("Warning: unknown parameter '%s' on line %d\n", key, lineNum)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Cross-field validations
	if config.initialInfected > config.popSize {
		validator.AddError("initialInfected", fmt.Sprintf("%d", config.initialInfected),
			fmt.Sprintf("cannot exceed popSize (%d)", config.popSize))
	}

	if config.medicalCapacity > config.popSize {
		validator.AddError("medicalCapacity", fmt.Sprintf("%d", config.medicalCapacity),
			fmt.Sprintf("cannot exceed popSize (%d)", config.popSize))
	}

	if config.socialDistanceThreshold > config.areaSize {
		validator.AddError("socialDistanceThreshold", fmt.Sprintf("%.2f", config.socialDistanceThreshold),
			fmt.Sprintf("cannot exceed areaSize (%.2f)", config.areaSize))
	}

	if config.transmissionDistance > config.areaSize {
		validator.AddError("transmissionDistance", fmt.Sprintf("%.2f", config.transmissionDistance),
			fmt.Sprintf("cannot exceed areaSize (%.2f)", config.areaSize))
	}

	if config.frameFrequency > config.numDays {
		validator.AddError("frameFrequency", fmt.Sprintf("%d", config.frameFrequency),
			fmt.Sprintf("cannot exceed numDays (%d)", config.numDays))
	}

	// Check if there were validation errors
	if validator.HasErrors() {
		validator.PrintErrors()
		return nil, fmt.Errorf("configuration validation failed with %d error(s)", len(validator.errors))
	}

	// Calculate medical capacity if not set
	if config.medicalCapacity == 0 {
		config.medicalCapacity = int(0.1 * float64(config.popSize))
	}

	return config, nil
}

// printConfigValidationHelp prints help information about valid parameter ranges
func printConfigValidationHelp() {
	fmt.Println(`
=== Configuration Parameter Validation Rules ===

DISEASE PARAMETERS:
  diseaseName          string    Non-empty, max 100 characters
  transmissionRate     float64   0.0 - 1.0 (probability)
  transmissionDistance float64   > 0.0, <= 100.0 (units)
  recoveryRate         float64   0.0 - 1.0 (probability per day)
  mortalityRate        float64   0.0 - 1.0 (probability)
  latentPeriod         int       0 - 365 (days)
  infectiousPeriod     int       1 - 365 (days)
  immunityDuration     int       0 - 3650 (days, 0 = no immunity)

POPULATION PARAMETERS:
  popSize              int       1 - 1,000,000
  initialInfected      int       0 - popSize

ENVIRONMENT PARAMETERS:
  areaSize             float64   > 0.0, <= 10,000.0
  socialDistanceThreshold float64 0.0 - 100.0, <= areaSize
  hygieneLevel         float64   0.0 - 1.0
  mobilityRate         float64   0.0 - 10.0
  vaccinationRate      float64   0.0 - 1.0
  medicalCareLevel     float64   0.0 - 1.0
  medicalCapacity      int       0 - popSize (0 = auto 10%)

SIMULATION PARAMETERS:
  numDays              int       1 - 10,000

VISUALIZATION PARAMETERS:
  canvasWidth          int       100 - 4096 (pixels)
  pointRadius          float64   0.5 - 50.0 (pixels)
  frameFrequency       int       1 - numDays
  gifDelay             int       1 - 1000 (centiseconds)
  gifFilename          string    Must end with .gif, no special chars

================================================
`)
}

func main() {
	configFile := flag.String("config", "", "Path to configuration file")
	showHelp := flag.Bool("help-config", false, "Show configuration parameter validation rules")
	flag.Parse()

	if *showHelp {
		printConfigValidationHelp()
		return
	}

	var config *Config

	if *configFile != "" {
		var err error
		config, err = loadConfigFromFile(*configFile)
		if err != nil {
			fmt.Printf("Error loading config file: %v\n", err)
			fmt.Println("\nRun with -help-config to see valid parameter ranges.")
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

	// Create output_gif folder if it doesn't exist
	outputDir := "output_gif"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("failed to create output directory '%s': %v\n", outputDir, err)
		return
	}

	// 1) Save spatial distribution GIF
	spatialPath := outputDir + "/" + config.gifFilename
	if err := SaveEnvironmentGIF(spatialPath, framesSpatial, config.gifDelay); err != nil {
		fmt.Println("failed to save spatial gif:", err)
	} else {
		fmt.Println("Spatial GIF saved to:", spatialPath)
	}

	// 2) Save pie chart GIF (prefix the filename)
	piePath := outputDir + "/pie_" + config.gifFilename
	if err := SaveEnvironmentGIF(piePath, framesPie, config.gifDelay); err != nil {
		fmt.Println("failed to save pie gif:", err)
	} else {
		fmt.Println("Pie GIF saved to:", piePath)
	}
}
