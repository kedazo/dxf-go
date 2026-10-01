package dxf

import (
	"fmt"
	"math"
	"testing"
)

func assertNearMatrix(t *testing.T, expected, actual Matrix) {
	t.Helper()
	for row := 0; row < 4; row++ {
		for column := 0; column < 4; column++ {
			if math.Abs(expected[row][column]-actual[row][column]) > nearEpsilon {
				t.Errorf("Expected: %v\nActual: %v", expected, actual)
				return
			}
		}
	}
}

func TestMatrixBasicTransforms(t *testing.T) {
	p := Point{1.0, 2.0, 3.0}
	assertEqPoint(t, p, IdentityMatrix().TransformPoint(p))
	assertEqPoint(t, Point{2.0, 2.0, 4.0}, TranslationMatrix(Vector{1.0, 0.0, 1.0}).TransformPoint(p))
	assertEqPoint(t, Point{2.0, -2.0, 9.0}, ScaleMatrix(2.0, -1.0, 3.0).TransformPoint(p))
	assertNearPoint(t, Point{-2.0, 1.0, 3.0}, RotationZMatrix(math.Pi/2.0).TransformPoint(p))

	// vectors ignore the translation
	assertEqVector(t, Vector{1.0, 2.0, 3.0}, TranslationMatrix(Vector{5.0, 5.0, 5.0}).TransformVector(Vector{1.0, 2.0, 3.0}))
}

func TestMatrixMulAppliesRightOperandFirst(t *testing.T) {
	translate := TranslationMatrix(Vector{1.0, 0.0, 0.0})
	rotate := RotationZMatrix(math.Pi / 2.0)
	// rotate first, then translate
	assertNearPoint(t, Point{1.0, 1.0, 0.0}, translate.Mul(rotate).TransformPoint(Point{1.0, 0.0, 0.0}))
	// translate first, then rotate
	assertNearPoint(t, Point{0.0, 2.0, 0.0}, rotate.Mul(translate).TransformPoint(Point{1.0, 0.0, 0.0}))
}

func TestMatrixInverse(t *testing.T) {
	m := TranslationMatrix(Vector{3.0, -2.0, 1.0}).
		Mul(RotationZMatrix(0.7)).
		Mul(ScaleMatrix(2.0, -0.5, 4.0)).
		Mul(OCSToWCSMatrix(Vector{1.0, 1.0, 1.0}))
	inverse, ok := m.Inverse()
	assertEqBool(t, true, ok)
	assertNearMatrix(t, IdentityMatrix(), m.Mul(inverse))
	assertNearMatrix(t, IdentityMatrix(), inverse.Mul(m))

	p := Point{1.5, -2.5, 7.0}
	assertNearPoint(t, p, inverse.TransformPoint(m.TransformPoint(p)))

	_, ok = ScaleMatrix(1.0, 0.0, 1.0).Inverse()
	assertEqBool(t, false, ok)
}

func TestMatrixDeterminantAndMirroring(t *testing.T) {
	assertNearFloat64(t, 1.0, RotationZMatrix(1.2).Determinant3())
	assertNearFloat64(t, -6.0, ScaleMatrix(1.0, -2.0, 3.0).Determinant3())
	assertEqBool(t, true, ScaleMatrix(-1.0, 1.0, 1.0).IsMirrored())
	assertEqBool(t, false, ScaleMatrix(-1.0, -1.0, 1.0).IsMirrored())
}

func TestMatrixTransformNormal(t *testing.T) {
	// the plane x + y = 0 scaled by 2 along X has the normal (1/2, 1, 0)
	normal, ok := ScaleMatrix(2.0, 1.0, 1.0).TransformNormal(Vector{1.0, 1.0, 0.0}.Normalize())
	assertEqBool(t, true, ok)
	assertNearVector(t, Vector{0.5, 1.0, 0.0}.Normalize(), normal)

	normal, _ = RotationZMatrix(math.Pi / 2.0).TransformNormal(*NewXAxis())
	assertNearVector(t, *NewYAxis(), normal)

	_, ok = ScaleMatrix(0.0, 1.0, 1.0).TransformNormal(*NewZAxis())
	assertEqBool(t, false, ok)
}

func TestArbitraryAxis(t *testing.T) {
	x, y := ArbitraryAxis(*NewZAxis())
	assertNearVector(t, *NewXAxis(), x)
	assertNearVector(t, *NewYAxis(), y)

	// mirrored drawings use (0, 0, -1): X flips, Y stays
	x, y = ArbitraryAxis(Vector{0.0, 0.0, -1.0})
	assertNearVector(t, Vector{-1.0, 0.0, 0.0}, x)
	assertNearVector(t, *NewYAxis(), y)

	// the normal does not need to be normalized
	x, y = ArbitraryAxis(Vector{2.0, 2.0, 2.0})
	assertNearVector(t, Vector{-1.0, 1.0, 0.0}.Normalize(), x)
	assertNearVector(t, Vector{-1.0, -1.0, 2.0}.Normalize(), y)

	// close to the Z axis the world Y axis is used
	x, _ = ArbitraryAxis(Vector{0.01, 0.0, 1.0})
	assertNearVector(t, NewYAxis().Cross(Vector{0.01, 0.0, 1.0}.Normalize()).Normalize(), x)

	for _, normal := range []Vector{{0, 0, 1}, {0, 0, -1}, {1, 0, 0}, {0.3, -0.4, 0.5}, {0.001, 0.002, -1}} {
		x, y := ArbitraryAxis(normal)
		n := normal.Normalize()
		assertNearFloat64(t, 0.0, x.Dot(y))
		assertNearFloat64(t, 0.0, x.Dot(n))
		assertNearVector(t, n, x.Cross(y))
		assert(t, math.Abs(x.Length()-1.0) < nearEpsilon, fmt.Sprintf("x axis for %v is not a unit vector", normal))
	}
}

func TestOCSConversion(t *testing.T) {
	// an OCS point on a mirrored (0, 0, -1) arc
	assertNearPoint(t, Point{-1.0, 2.0, -3.0}, OCSToWCSMatrix(Vector{0.0, 0.0, -1.0}).TransformPoint(Point{1.0, 2.0, 3.0}))

	normal := Vector{0.3, -0.4, 0.5}
	p := Point{1.0, 2.0, 3.0}
	assertNearPoint(t, p, WCSToOCSMatrix(normal).TransformPoint(OCSToWCSMatrix(normal).TransformPoint(p)))
	// the OCS Z axis is the normal, so the OCS Z coordinate is the elevation along it
	wcs := OCSToWCSMatrix(normal).TransformPoint(Point{0.0, 0.0, 2.0})
	assertNearVector(t, normal.Normalize().Scale(2.0), wcs.ToVector())
}
