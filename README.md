# 02-601 Project: SIR-Based Epidemic Simulation

A computational epidemiological simulation written in Go that models infectious disease spread using an extended SIR (Susceptible-Infected-Recovered) framework with spatial movement, individual behaviors, and dynamic environmental responses.

## Authors

[Ailisi Bao](https://github.com/ailisilob), [Peiyang Liu](https://github.com/mast13f/), [Shuaiqi Huang](https://github.com/idkwhatgitis), [Sihyun Park](https://github.com/SihyunPark99)

## Final Report and Video Demonstration
See our final report [here.](https://docs.google.com/document/d/1Zp94CutJ-sAeT_oSuK9t_Yrk9xigCBARBCo86K1RhbA/edit?usp=sharing) 

See our video demonstration [here.]


## Background

Pandemics have caused countless deaths and tremendous economic losses throughout history. From a scientific perspective, it is essential to predict the outbreak dynamics of infectious diseases before they spread to larger populations. The modeling of pandemics can contribute to the establishment of public health policies and the promotion of hygienic lifestyles, providing a scientific basis for effectively responding to large-scale outbreaks such as COVID-19.

This project addresses the need for accurate and efficient simulation of infectious disease spread within a population. While the classic SIR model divides populations into Susceptible (S), Infectious (I), and Recovered (R) groups using differential equations, many real-world applications require a more thorough examination of how factors like transmission rates, recovery rates, population size, and intervention strategies influence disease spread over time.

Our simulation extends the traditional SIR framework by modeling each individual with their own health state, location, movement patterns, and behavioral attributes. This captures realistic dynamics including vaccination rollout, hospital capacity constraints, behavioral adaptation, and policy responses to infection rates.

## Health States

Individuals transition through five possible health states:

| State | Color | Description |
|-------|-------|-------------|
| Healthy | Green | Not yet exposed to the disease |
| Vaccinated | Dark Green | Healthy individuals who have been vaccinated |
| Susceptible | Yellow | Exposed and at risk of infection |
| Infected | Red | Currently infectious |
| Recovered | Blue | Recovered with temporary immunity |
| Dead | Gray | Deceased from the disease |

## Project Structure

```
PFSFinalProject/
├── main.go              # Entry point, config loading, simulation loop
├── datatypes.go         # Core data structures (Disease, Individual, Environment)
├── initialization.go    # Population setup and initial infection seeding
├── updateInd.go         # Health transitions, vaccination, behavioral updates
├── updateMov.go         # Movement logic for Walk/Train/Flight patterns
├── updateEnv.go         # Policy adjustments and environmental responses
├── helper_functions.go  # Distance calculations and statistics output
├── canvas.go            # Custom graphics library for drawing
├── drawings.go          # Spatial map and pie chart visualization
├── rshiny.R             # R Shiny interactive visualization app
├── config/              # Example configuration files
└── go.mod               # Go module definition
```
## Libraries

### Go
- `image`, `image/color`, `image/gif`, `image/png`, `image/draw` — Image creation and manipulation
- `math`, `math/rand` — Mathematical operations and random number generation
- `golang.org/x/image/font`, `golang.org/x/image/font/basicfont` — Font rendering for visualization labels
- `golang.org/x/image/math/fixed` — Fixed-point math for text positioning
- `github.com/llgcode/draw2d/draw2dimg` — 2D graphics rendering

### R
- `shiny` — Interactive web application framework
- `processx` — Process management
- `magick` — Image processing and GIF handling
## Usage

### Running the Simulation

Build the program:

```bash
go build
```

Run with a configuration file:

```bash
./PFSFinalProject -config your_config.txt
```

Or run with default values:

```bash
./PFSFinalProject
```

The simulation prints daily statistics to the console as it runs.

### Configuration

Create a configuration file with parameters in `key = value` format. Lines starting with `#` are comments.

```
# Disease Configuration
diseaseName = Deadly2
transmissionRate = 0.8          # How easily the disease spreads (0-1)
transmissionDistance = 5        # Distance within which transmission can occur
recoveryRate = 0.0001           # Daily probability of recovery
mortalityRate = 0.3             # Probability of death for infected individuals
latentPeriod = 1                # Days before becoming infectious
infectiousPeriod = 20           # Days an individual remains infectious
immunityDuration = 60           # Days immunity lasts after recovery

# Population Configuration
popSize = 2500                  # Total number of individuals
initialInfected = 50            # Number of infected at simulation start

# Environment Configuration
areaSize = 150.0                # Size of the 2D simulation space
socialDistanceThreshold = 0.1   # Initial social distancing policy strictness
hygieneLevel = 0.01             # Baseline environmental hygiene
mobilityRate = 0.5              # How much individuals move
vaccinationRate = 0.01          # Daily vaccination capacity
medicalCareLevel = 0.10         # Quality of available medical care
medicalCapacity = 80            # Hospital bed capacity

# Simulation Configuration
numDays = 365                   # Number of days to simulate

# Visualization Configuration
canvasWidth = 1000              # Output image width in pixels
pointRadius = 4.0               # Size of individual dots in visualization
frameFrequency = 3              # Capture frame every N days
gifDelay = 8                    # Animation speed (delay between frames)
gifFilename = deadly2.gif       # Output filename
```

### Visualization

The simulation generates two animated GIFs:
1. **Spatial map**: Shows the geographic distribution of individuals colored by health state
2. **Pie chart**: Shows the proportional breakdown of the population across health states over time

For interactive visualization, launch the R Shiny app:

```r
shiny::runApp("rshiny.R")
```

## Output

The simulation automatically creates an `output_gif/` folder (if it doesn't exist) and generates:

- **Animated GIFs**: Spatial distribution map and pie chart showing epidemic progression
- **Console output**: Daily statistics printed during simulation

## Example Results

By adjusting parameters, you can simulate vastly different epidemic scenarios:

- **High mortality**: Large portions of the population may die, visible as expanding gray regions
- **High transmission, low mortality**: Disease spreads widely but most individuals survive and recover
- **Clustered spread**: Local outbreaks form based on movement patterns and population density




