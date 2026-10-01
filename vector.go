package dxf

import (
	"fmt"
	"math"
)

// The Vector struct represents a vector in 3D space.
type Vector struct {
	X float64
	Y float64
	Z float64
}

// NewXAxis creates a unit vector along the X axis.
func NewXAxis() *Vector {
	return &Vector{
		X: 1.0,
		Y: 0.0,
		Z: 0.0,
	}
}

// NewYAxis creates a unit vector along the Y axis.
func NewYAxis() *Vector {
	return &Vector{
		X: 0.0,
		Y: 1.0,
		Z: 0.0,
	}
}

// NewZAxis creates a unit vector along the Z axis.
func NewZAxis() *Vector {
	return &Vector{
		X: 0.0,
		Y: 0.0,
		Z: 1.0,
	}
}

// NewZeroVector creates a vector representing zero distance across any axis.
func NewZeroVector() *Vector {
	return &Vector{
		X: 0.0,
		Y: 0.0,
		Z: 0.0,
	}
}

// Add returns v + other.
func (v Vector) Add(other Vector) Vector {
	return Vector{v.X + other.X, v.Y + other.Y, v.Z + other.Z}
}

// Sub returns v - other.
func (v Vector) Sub(other Vector) Vector {
	return Vector{v.X - other.X, v.Y - other.Y, v.Z - other.Z}
}

// Scale returns v multiplied by factor.
func (v Vector) Scale(factor float64) Vector {
	return Vector{v.X * factor, v.Y * factor, v.Z * factor}
}

// Neg returns -v.
func (v Vector) Neg() Vector {
	return Vector{-v.X, -v.Y, -v.Z}
}

// Dot returns the dot product of v and other.
func (v Vector) Dot(other Vector) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

// Cross returns the cross product v × other.
func (v Vector) Cross(other Vector) Vector {
	return Vector{
		v.Y*other.Z - v.Z*other.Y,
		v.Z*other.X - v.X*other.Z,
		v.X*other.Y - v.Y*other.X,
	}
}

// Length returns the Euclidean length of v.
func (v Vector) Length() float64 {
	return math.Sqrt(v.Dot(v))
}

// Normalize returns v scaled to unit length, or the zero vector if v has no length.
func (v Vector) Normalize() Vector {
	length := v.Length()
	if length == 0 {
		return Vector{}
	}
	return v.Scale(1.0 / length)
}

// IsZero reports whether every component of v is within epsilon of zero.
func (v Vector) IsZero(epsilon float64) bool {
	return math.Abs(v.X) <= epsilon && math.Abs(v.Y) <= epsilon && math.Abs(v.Z) <= epsilon
}

// ToPoint returns the point at v from the origin.
func (v Vector) ToPoint() Point {
	return Point{v.X, v.Y, v.Z}
}

func (v *Vector) String() string {
	return fmt.Sprintf("(%s, %s, %s)", formatFloat64Text(v.X), formatFloat64Text(v.Y), formatFloat64Text(v.Z))
}
