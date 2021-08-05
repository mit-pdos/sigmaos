package fslib

import (
	"math/rand"
	"strconv"
)

type Ttype uint32
type Tcore uint32

const (
	T_DEF Ttype = 0
	T_LC  Ttype = 1
	T_BE  Ttype = 2
)

const (
	C_DEF Tcore = 0
)

type WaitFile struct {
	Started  bool
	StartDep []string // PIDs of lambdas that have a start dependency on this lambda.
	ExitDep  []string // PIDs of lambdas that have a start dependency on this lambda.
}

const (
	LOCALD_ROOT  = "name/localds"
	NO_OP_LAMBDA = "no-op-lambda"
)

func GenPid() string {
	return strconv.Itoa(rand.Intn(100000))
}
