package main

import (
	"os"
	"strconv"
	"time"

	"gonum.org/v1/gonum/mat"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v N\n", os.Args[0])
	}
	m, err := MakeMatrixMult(os.Args[1:])
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	m.Work()
}

type MatrixMult struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	n  int
	m1 *mat.Dense
	m2 *mat.Dense
	m3 *mat.Dense
}

func MakeMatrixMult(args []string) (*MatrixMult, error) {
	var err error
	db.DPrintf("MATMUL", "MakeMatrixMul: %v %v", proc.GetPid(), args)
	m := &MatrixMult{}
	m.FsLib = fslib.MakeFsLib("spinner")
	m.ProcClnt = procclnt.MakeProcClnt(m.FsLib)
	m.n, err = strconv.Atoi(args[0])
	if err != nil {
		db.DFatalf("Error parsing N: %v", err)
	}
	m.m1 = matrix(m.n)
	m.m2 = matrix(m.n)
	m.m3 = matrix(m.n)
	return m, nil
}

// Create an n x n matrix.
func matrix(n int) *mat.Dense {
	s := make([]float64, n*n)
	for i := 0; i < n*n; i++ {
		s[i] = float64(i)
	}
	return mat.NewDense(n, n, s)
}

func (m *MatrixMult) doMM() {
	// Multiply m.m1 and m.m2, and place the result in m.m3
	m.m3.Mul(m.m1, m.m2)
}

func (m *MatrixMult) Work() {
	err := m.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	start := time.Now()
	db.DPrintf("MATMUL", "doMM %v", proc.GetPid())
	m.doMM()
	db.DPrintf("MATMUL", "doMM done %v: %v", proc.GetPid(), time.Since(start))
	m.Exited(proc.MakeStatusInfo(proc.StatusOK, "Latency (us)", time.Since(start).Microseconds()))
	db.DPrintf("MATMUL", "Exited %v", proc.GetPid())
}
