package dxf

import (
	"math"
	"testing"
)

func TestVectorArithmetic(t *testing.T) {
	a := Vector{1.0, 2.0, 3.0}
	b := Vector{4.0, 5.0, 6.0}
	assertEqVector(t, Vector{5.0, 7.0, 9.0}, a.Add(b))
	assertEqVector(t, Vector{-3.0, -3.0, -3.0}, a.Sub(b))
	assertEqVector(t, Vector{2.0, 4.0, 6.0}, a.Scale(2.0))
	assertEqVector(t, Vector{-1.0, -2.0, -3.0}, a.Neg())
	assertEqFloat64(t, 32.0, a.Dot(b))
	assertEqVector(t, Vector{-3.0, 6.0, -3.0}, a.Cross(b))
	assertEqVector(t, *NewZAxis(), NewXAxis().Cross(*NewYAxis()))
	assertEqFloat64(t, 5.0, Vector{3.0, 4.0, 0.0}.Length())
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, a.ToPoint())
}

func TestVectorNormalize(t *testing.T) {
	assertNearVector(t, Vector{0.6, 0.8, 0.0}, Vector{3.0, 4.0, 0.0}.Normalize())
	assertEqVector(t, Vector{}, Vector{}.Normalize())
	assertNearFloat64(t, 1.0, Vector{1.0, 1.0, 1.0}.Normalize().Length())
}

func TestVectorIsZero(t *testing.T) {
	assertEqBool(t, true, Vector{1e-12, -1e-12, 0.0}.IsZero(1e-9))
	assertEqBool(t, false, Vector{0.0, 0.0, 1e-6}.IsZero(1e-9))
	assertEqBool(t, true, Vector{}.IsZero(0.0))
}

func TestPointArithmetic(t *testing.T) {
	p := Point{1.0, 2.0, 3.0}
	assertEqPoint(t, Point{2.0, 2.0, 4.0}, p.Add(Vector{1.0, 0.0, 1.0}))
	assertEqVector(t, Vector{1.0, 2.0, 2.0}, p.Sub(Point{0.0, 0.0, 1.0}))
	assertEqVector(t, Vector{1.0, 2.0, 3.0}, p.ToVector())
	assertEqFloat64(t, math.Sqrt(14.0), p.ToVector().Length())
}
