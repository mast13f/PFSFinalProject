package main

import (
	//"errors"
	"math"
	"math/rand"
	//"time"
)

// ---------------- Movement update ----------------

// updateMove updates the individual's position based on their movement pattern.
// It randomly selects the direction to go, and randomly selects the length of movement
// Then we perform update on individual's position
func (ind *Individual) updateMove(env *Environment) {
	if ind.movementPattern == nil || ind.healthStatus == Dead {
		return
	}

	if ind.healthStatus == Infected {
		ind.movementPattern = &MovementPattern{
			moveType:   Walk,
			moveRadius: env.areaSize * 0.001,
		}
	}
	// Movement radius depends on environment area size
	moveRadius := ind.movementPattern.moveRadius
	if moveRadius <= 0 {
		// In case movementPattern hasn’t been initialized properly
		ind.movementPattern = NewMovementPattern(ind.movementPattern.moveType, env)
		moveRadius = ind.movementPattern.moveRadius
	}

	// Random direction (0 to 2π)
	//random movement length
	dist := math.Sqrt(rand.Float64()) * moveRadius
	angle := rand.Float64() * 2 * math.Pi

	dx := dist * math.Cos(angle)
	dy := dist * math.Sin(angle)

	newX := ind.position.x + dx
	newY := ind.position.y + dy

	// Keep within environment boundaries (wrap around)
	if newX < 0 {
		newX = env.areaSize + newX
	} else if newX > env.areaSize {
		newX = newX - env.areaSize
	}

	if newY < 0 {
		newY = env.areaSize + newY
	} else if newY > env.areaSize {
		newY = newY - env.areaSize
	}

	// Update position
	ind.position = OrderedPair{x: newX, y: newY}

	ind.UpdateMovementPattern(env)
}

// NewMovementPattern creates a MovementPattern based on areaSize
// How far a person can go depends on the travel type.
// If a person is walking, then it will move the slowest. 0.1% of the map in each generation
// If a person is on the train, it can move 1/10th of the map
// If a person is taking a flight, then it can move anywhere
// We may update this in the future for complexity(example, person on a flight can only go to airport)
func NewMovementPattern(mt moveType, env *Environment) *MovementPattern {
	var radius float64

	switch mt {
	case Walk:
		radius = env.areaSize * 0.001
	case Train:
		radius = env.areaSize * 0.1
	case Flight:
		radius = env.areaSize
	default:
		radius = env.areaSize * 0.05
	}

	return &MovementPattern{
		moveType:   mt,
		moveRadius: radius,
	}
}

// updateMovementPattern will assign a movement pattern to an individual
// After the individual moves, decide how it moves for next move
// 1% chance on flight, 4% chance on train, 95% walk
func (ind *Individual) UpdateMovementPattern(env *Environment) {
	val := rand.Float64()

	if val <= 0.01 {
		ind.movementPattern = &MovementPattern{
			moveType:   Flight,
			moveRadius: env.areaSize,
		}
	} else if val <= 0.05 {
		ind.movementPattern = &MovementPattern{
			moveType:   Train,
			moveRadius: env.areaSize * 0.1,
		}

	} else {
		ind.movementPattern = &MovementPattern{
			moveType:   Walk,
			moveRadius: env.areaSize * 0.001,
		}
	}
}
