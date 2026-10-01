package dxf

import (
	"fmt"
)

// The Point struct represents a 3D coordinate.
type Point struct {
	X float64
	Y float64
	Z float64
}

// NewOrigin creates a new Point representing the (0, 0, 0) location.
func NewOrigin() *Point {
	return &Point{
		X: 0.0,
		Y: 0.0,
		Z: 0.0,
	}
}

// Add returns p moved by offset.
func (p Point) Add(offset Vector) Point {
	return Point{p.X + offset.X, p.Y + offset.Y, p.Z + offset.Z}
}

// Sub returns the vector from other to p.
func (p Point) Sub(other Point) Vector {
	return Vector{p.X - other.X, p.Y - other.Y, p.Z - other.Z}
}

// ToVector returns the vector from the origin to p.
func (p Point) ToVector() Vector {
	return Vector{p.X, p.Y, p.Z}
}

func (p *Point) String() string {
	return fmt.Sprintf("(%s, %s, %s)", formatFloat64Text(p.X), formatFloat64Text(p.Y), formatFloat64Text(p.Z))
}
