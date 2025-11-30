package main

import (
	"canvas"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"math"
	"os"
)

// AnimateEnvironment generates a sequence of images over time.
// Each Environment in timePoints becomes one frame (subsampled by frequency).
func AnimateEnvironment(timePoints []*Environment, canvasWidth, frequency int, scalingFactor float64) []image.Image {
	images := make([]image.Image, 0)

	if len(timePoints) == 0 {
		panic("Error: no Environment objects present in AnimateEnvironment.")
	}

	for i := range timePoints {
		if i%frequency == 0 {
			fmt.Println("frame", i)
			images = append(images, timePoints[i].DrawToCanvas(canvasWidth, scalingFactor))
		}
	}

	return images
}

// DrawToCanvas renders a single Environment (one time step) onto a square canvas of canvasWidth x canvasWidth pixels with a black background.
// Each individual is drawn as a colored circle; scalingFactor controls the radius of each point in pixels.
func (env *Environment) DrawToCanvas(canvasWidth int, scalingFactor float64) image.Image {
	if env == nil {
		panic("Can't Draw a nil Environment.")
	}

	// Create a new canvas
	c := canvas.CreateNewCanvas(canvasWidth, canvasWidth)

	// Background: Black
	c.SetFillColor(canvas.MakeColor(0, 0, 0))
	c.ClearRect(0, 0, canvasWidth, canvasWidth)
	c.Fill()

	if env.areaSize <= 0 {
		// If areaSize is invalid
		env.areaSize = 1.0
	}

	// Draw each individual as a circle
	for _, ind := range env.population {
		if ind == nil {
			continue
		}

		r, g, b := colorForHealthStatus(ind)

		c.SetFillColor(canvas.MakeColor(r, g, b))

		// Map position from [0, areaSize] to [0, canvasWidth]
		cx := (ind.position.x / env.areaSize) * float64(canvasWidth)
		cy := (ind.position.y / env.areaSize) * float64(canvasWidth)

		// Radius in pixels; if scalingFactor is non-positive, use a default
		radius := scalingFactor
		if radius <= 0 {
			radius = 2.0
		}

		c.Circle(cx, cy, radius)
		c.Fill()
	}

	return c.GetImage()
}

// colorForHealthStatus returns an RGB color for an individual based on their health status and vaccination status
func colorForHealthStatus(ind *Individual) (uint8, uint8, uint8) {
	if ind == nil {
		return 255, 255, 255 // fallback: white
	}
	if ind.vaccinated && ind.healthStatus != Dead {
		return 64, 128, 64 // Dark Green
	}

	switch ind.healthStatus {
	case Healthy:
		return 128, 255, 0 // Green
	case Susceptible:
		return 255, 255, 0 // Yellow
	case Infected:
		return 255, 0, 0 // Red
	case Recovered:
		return 0, 128, 255 // Blue
	case Dead:
		return 160, 160, 160 // Grey
	default:
		return 255, 255, 255 // unknown: white
	}
}

// SaveEnvironmentGIF encodes a sequence of frames into a single GIF file.
// delay is specified in units of 1/100 seconds between frames.
func SaveEnvironmentGIF(filename string, frames []image.Image, delay int) error {
	if len(frames) == 0 {
		return fmt.Errorf("no frames to save")
	}

	outGIF := &gif.GIF{}

	for _, img := range frames {
		bounds := img.Bounds()
		paletted := image.NewPaletted(bounds, palette.Plan9)
		draw.Draw(paletted, bounds, img, bounds.Min, draw.Src)

		outGIF.Image = append(outGIF.Image, paletted)
		outGIF.Delay = append(outGIF.Delay, delay)
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := gif.EncodeAll(f, outGIF); err != nil {
		return err
	}
	return nil
}

// DrawEnvironmentPie renders a pie chart of population status for a single Environment at one time step.
// The pie shows counts of Healthy, Vaccinated(alive), Susceptible, Infected, Recovered, and Dead.
func DrawEnvironmentPie(env *Environment, size int) image.Image {
	if env == nil {
		panic("Can't draw pie for nil Environment")
	}
	if size <= 0 {
		size = 400
	}

	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Background: Black
	draw.Draw(img, img.Bounds(), &image.Uniform{color.Black}, image.Point{}, draw.Src)

	h, v, s, inf, r, d := statusCountsFromEnv(env)
	total := h + s + inf + r + d
	if total == 0 {
		return img
	}

	colHealthy := color.RGBA{128, 255, 0, 255}    // Green
	colVaccniated := color.RGBA{64, 128, 64, 255} // Dark Green
	colSuscept := color.RGBA{255, 255, 0, 255}    // Yellow
	colInfect := color.RGBA{255, 0, 0, 255}       // Red
	colRecov := color.RGBA{0, 128, 255, 255}      // Blue
	colDead := color.RGBA{160, 160, 160, 255}     // Grey

	type slice struct {
		start float64
		end   float64
		col   color.RGBA
	}

	const tau = 2 * math.Pi
	slices := make([]slice, 0)
	acc := 0.0

	// Helper to append a slice if count > 0
	addSlice := func(count int, c color.RGBA) {
		if count <= 0 {
			return
		}
		frac := float64(count) / float64(total)
		slices = append(slices, slice{
			start: acc,
			end:   acc + frac*tau,
			col:   c,
		})
		acc += frac * tau
	}

	// Order: D, R, I, S, V, H
	addSlice(d, colDead)
	addSlice(r, colRecov)
	addSlice(inf, colInfect)
	addSlice(s, colSuscept)
	addSlice(v, colVaccniated)
	addSlice(h, colHealthy)

	// Draw pie chart centered in the image
	cx := float64(size) / 2
	cy := float64(size) / 2
	radius := float64(size) * 0.4

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist2 := dx*dx + dy*dy
			if dist2 > radius*radius {
				continue
			}

			angle := math.Atan2(-dy, dx)
			if angle < 0 {
				angle += tau
			}
			angle = math.Mod(math.Pi/2-angle+tau, tau)

			for _, sl := range slices {
				if angle >= sl.start && angle < sl.end {
					img.Set(x, y, sl.col)
					break
				}
			}
		}
	}

	return img
}

// statusCountsFromEnv returns the counts of individuals by status:
// healthy (non-infected), vaccinated (alive), susceptible, infected,
func statusCountsFromEnv(env *Environment) (healthy, vaccinated, susceptible, infected, recovered, dead int) {
	if env == nil {
		return
	}

	_, totalInfected, _, _, _, _ := ComputePopulationStats(env)
	infected = totalInfected

	for _, ind := range env.population {
		if ind == nil {
			continue
		}
		if ind.vaccinated && ind.healthStatus != Dead {
			vaccinated++
		}
		switch ind.healthStatus {
		case Healthy:
			healthy++
		case Susceptible:
			susceptible++
		case Infected: // Already implemented in totalInfected
		case Recovered:
			recovered++
		case Dead:
			dead++
		}
	}
	return
}

// AnimateEnvironmentPie generates a sequence of pie-chart images for a sequence of Environments.
// Each Environment in timePoints becomes one frame (subsampled by frequency).
func AnimateEnvironmentPie(timePoints []*Environment, size, frequency int) []image.Image {
	images := make([]image.Image, 0)

	if len(timePoints) == 0 {
		panic("Error: no Environment objects present in AnimateEnvironmentPie.")
	}

	for i := range timePoints {
		if i%frequency == 0 {
			fmt.Println("pie frame", i)
			images = append(images, DrawEnvironmentPie(timePoints[i], size))
		}
	}

	return images
}
