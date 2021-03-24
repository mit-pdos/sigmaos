package perf

import (
	"fmt"
	"math/rand"
)

type Matrix struct {
	m int
	n int
	v [][]float64
}

func MakeMatrix(n int, m int) *Matrix {
	A := &Matrix{}
	A.m = m
	A.n = n
	A.v = make([][]float64, m)
	for i := range A.v {
		A.v[i] = make([]float64, n)
	}
	return A
}

func Mult(A *Matrix, B *Matrix, C *Matrix) error {
	if A.n != B.m || C.m != A.m || C.n != B.n {
		return fmt.Errorf("Matrix dimension mismatch: A:%vx%v, B:%vx%v, C:%vx%v", A.m, A.n, B.m, B.n, C.m, C.n)
	}
	for i := 0; i < A.m; i++ {
		for j := 0; j < B.n; j++ {
			C.v[i][j] = 0.0
			for k := 0; k < A.n; k++ {
				C.v[i][j] += A.v[i][k] * B.v[k][j]
			}
		}
	}
	return nil
}

// Fill with a set value
func (A *Matrix) Fill(x float64) {
	for i := range A.v {
		for j := range A.v[i] {
			A.v[i][j] = x
		}
	}
}

// Fill with random values
func (A *Matrix) FillRandom() {
	for i := range A.v {
		for j := range A.v[i] {
			A.v[i][j] = rand.Float64()
		}
	}
}

// Fill with random non-zero values
func (A *Matrix) FillRandomNonZero() {
	for i := range A.v {
		for j := range A.v[i] {
			A.v[i][j] = rand.Float64() + 1.0
		}
	}
}

func (A *Matrix) String() string {
	return fmt.Sprintf("%v\n", A.v)
}
