package proc

import (
	"fmt"
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

type Proc struct {
	Pid     string   // SigmaOS PID
	Program string   // Program to run
	Dir     string   // Working directory for the process
	Args    []string // Args
	Env     []string // Environment variables
	Type    Ttype    // Type
	Ncore   Tcore    // Number of cores requested
}

func MakeEmptyProc() *Proc {
	p := &Proc{}
	return p
}

func MakeProc(pid string, program string, args []string) *Proc {
	p := &Proc{}
	p.Pid = pid
	p.Program = program
	p.Args = args
	return p
}

func (p *Proc) GetProc() *Proc {
	return p
}

func (p *Proc) String() string {
	return fmt.Sprintf("&{ Pid:%v Program:%v Dir:%v Args:%v Env:%v Type:%v Ncore:%v }", p.Pid, p.Program, p.Dir, p.Args, p.Env, p.Type, p.Ncore)
}
