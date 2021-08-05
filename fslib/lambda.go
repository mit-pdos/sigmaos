package fslib

import (
	"math/rand"
	"strconv"
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
