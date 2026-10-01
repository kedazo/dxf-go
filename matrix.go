package dxf

import (
	"math"
)

// Matrix is an affine 4x4 transformation acting on column vectors: p' = M·p. Use IdentityMatrix as the starting
// point; the zero value maps everything to the origin.
type Matrix [4][4]float64

// IdentityMatrix returns the transformation that changes nothing.
func IdentityMatrix() Matrix {
	return Matrix{
		{1, 0, 0, 0},
		{0, 1, 0, 0},
		{0, 0, 1, 0},
		{0, 0, 0, 1},
	}
}

// TranslationMatrix returns a transformation that moves by offset.
func TranslationMatrix(offset Vector) Matrix {
	m := IdentityMatrix()
	m[0][3] = offset.X
	m[1][3] = offset.Y
	m[2][3] = offset.Z
	return m
}

// ScaleMatrix returns a transformation that scales along the axes; negative factors mirror.
func ScaleMatrix(x, y, z float64) Matrix {
	m := IdentityMatrix()
	m[0][0] = x
	m[1][1] = y
	m[2][2] = z
	return m
}

// RotationZMatrix returns a counter-clockwise rotation around the Z axis by angle radians.
func RotationZMatrix(angle float64) Matrix {
	sin, cos := math.Sincos(angle)
	m := IdentityMatrix()
	m[0][0] = cos
	m[0][1] = -sin
	m[1][0] = sin
	m[1][1] = cos
	return m
}

// AxesMatrix returns the transformation that maps the X, Y and Z axes to the given vectors and the origin to origin.
func AxesMatrix(xAxis, yAxis, zAxis Vector, origin Point) Matrix {
	return Matrix{
		{xAxis.X, yAxis.X, zAxis.X, origin.X},
		{xAxis.Y, yAxis.Y, zAxis.Y, origin.Y},
		{xAxis.Z, yAxis.Z, zAxis.Z, origin.Z},
		{0, 0, 0, 1},
	}
}

// Mul returns m·other: the transformation that applies other first and then m.
func (m Matrix) Mul(other Matrix) Matrix {
	var result Matrix
	for row := 0; row < 4; row++ {
		for column := 0; column < 4; column++ {
			sum := 0.0
			for k := 0; k < 4; k++ {
				sum += m[row][k] * other[k][column]
			}
			result[row][column] = sum
		}
	}
	return result
}

// TransformPoint applies m to a point.
func (m Matrix) TransformPoint(p Point) Point {
	return Point{
		m[0][0]*p.X + m[0][1]*p.Y + m[0][2]*p.Z + m[0][3],
		m[1][0]*p.X + m[1][1]*p.Y + m[1][2]*p.Z + m[1][3],
		m[2][0]*p.X + m[2][1]*p.Y + m[2][2]*p.Z + m[2][3],
	}
}

// TransformVector applies m to a direction or offset, i.e. without the translation.
func (m Matrix) TransformVector(v Vector) Vector {
	return Vector{
		m[0][0]*v.X + m[0][1]*v.Y + m[0][2]*v.Z,
		m[1][0]*v.X + m[1][1]*v.Y + m[1][2]*v.Z,
		m[2][0]*v.X + m[2][1]*v.Y + m[2][2]*v.Z,
	}
}

// TransformNormal returns the unit normal of a plane with the given normal after applying m (using the inverse
// transpose, so it stays perpendicular under non-uniform scaling). It returns false if m is degenerate.
func (m Matrix) TransformNormal(normal Vector) (Vector, bool) {
	inverse, ok := m.Inverse()
	if !ok {
		return Vector{}, false
	}
	transformed := Vector{
		inverse[0][0]*normal.X + inverse[1][0]*normal.Y + inverse[2][0]*normal.Z,
		inverse[0][1]*normal.X + inverse[1][1]*normal.Y + inverse[2][1]*normal.Z,
		inverse[0][2]*normal.X + inverse[1][2]*normal.Y + inverse[2][2]*normal.Z,
	}
	return transformed.Normalize(), true
}

// Determinant3 returns the determinant of the linear (3x3) part: the volume scale, negative when m mirrors.
func (m Matrix) Determinant3() float64 {
	return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) -
		m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
		m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
}

// IsMirrored reports whether m flips orientation.
func (m Matrix) IsMirrored() bool {
	return m.Determinant3() < 0
}

// Inverse returns the inverse of the affine transformation m, or false if m is degenerate.
func (m Matrix) Inverse() (Matrix, bool) {
	determinant := m.Determinant3()
	if determinant == 0 || math.IsNaN(determinant) || math.IsInf(determinant, 0) {
		return Matrix{}, false
	}

	// inverse of the linear part via the adjugate
	inverse := IdentityMatrix()
	inverse[0][0] = (m[1][1]*m[2][2] - m[1][2]*m[2][1]) / determinant
	inverse[0][1] = (m[0][2]*m[2][1] - m[0][1]*m[2][2]) / determinant
	inverse[0][2] = (m[0][1]*m[1][2] - m[0][2]*m[1][1]) / determinant
	inverse[1][0] = (m[1][2]*m[2][0] - m[1][0]*m[2][2]) / determinant
	inverse[1][1] = (m[0][0]*m[2][2] - m[0][2]*m[2][0]) / determinant
	inverse[1][2] = (m[0][2]*m[1][0] - m[0][0]*m[1][2]) / determinant
	inverse[2][0] = (m[1][0]*m[2][1] - m[1][1]*m[2][0]) / determinant
	inverse[2][1] = (m[0][1]*m[2][0] - m[0][0]*m[2][1]) / determinant
	inverse[2][2] = (m[0][0]*m[1][1] - m[0][1]*m[1][0]) / determinant

	// the inverse translation is -L⁻¹·t
	translation := inverse.TransformVector(Vector{m[0][3], m[1][3], m[2][3]})
	inverse[0][3] = -translation.X
	inverse[1][3] = -translation.Y
	inverse[2][3] = -translation.Z
	return inverse, true
}

// ArbitraryAxis returns the X and Y axes of the object coordinate system (OCS) with the given normal (extrusion
// direction), using AutoCAD's arbitrary axis algorithm.
func ArbitraryAxis(normal Vector) (xAxis, yAxis Vector) {
	n := normal.Normalize()
	if n.IsZero(0) {
		return *NewXAxis(), *NewYAxis()
	}

	const limit = 1.0 / 64.0
	if math.Abs(n.X) < limit && math.Abs(n.Y) < limit {
		xAxis = NewYAxis().Cross(n).Normalize()
	} else {
		xAxis = NewZAxis().Cross(n).Normalize()
	}
	yAxis = n.Cross(xAxis).Normalize()
	return
}

// OCSToWCSMatrix returns the transformation from the object coordinate system with the given normal to world
// coordinates.
func OCSToWCSMatrix(normal Vector) Matrix {
	xAxis, yAxis := ArbitraryAxis(normal)
	zAxis := normal.Normalize()
	if zAxis.IsZero(0) {
		zAxis = *NewZAxis()
	}
	return AxesMatrix(xAxis, yAxis, zAxis, Point{})
}

// WCSToOCSMatrix returns the transformation from world coordinates to the object coordinate system with the given
// normal.
func WCSToOCSMatrix(normal Vector) Matrix {
	m := OCSToWCSMatrix(normal)
	// the axes are orthonormal, so the inverse is the transpose
	for row := 0; row < 3; row++ {
		for column := row + 1; column < 3; column++ {
			m[row][column], m[column][row] = m[column][row], m[row][column]
		}
	}
	return m
}
