package proc

import (
	"fmt"
	"path"
	"strings"
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

func PidDir(pid string) string {
	piddir := path.Dir(pid)
	if piddir == "." {
		piddir = "pids/" + pid
	} else {
		piddir = pid
	}
	return piddir
}

type Proc struct {
	Pid     string   // SigmaOS PID
	PidDir  string   // SigmaOS PID pathname
	Program string   // Program to run
	Dir     string   // Unix working directory for the process
	Args    []string // Args
	Env     []string // Environment variables
	Type    Ttype    // Type
	Ncore   Tcore    // Number of cores requested
}

func MakeEmptyProc() *Proc {
	p := &Proc{}
	return p
}

func MakeProc(program string, args []string) *Proc {
	p := &Proc{}
	p.Pid = GenPid()
	p.PidDir = "pids"
	p.Program = program
	p.Args = args
	p.Type = T_DEF
	p.Ncore = C_DEF
	return p
}

func MakeProcPid(pid string, program string, args []string) *Proc {
	p := MakeProc(program, args)
	p.Pid = pid
	return p
}

func (p *Proc) AppendEnv(name, val string) {
	p.Env = append(p.Env, name+"="+val)
}

func (p *Proc) IsKernelProc() bool {
	return strings.Contains(p.Program, "kernel")
}

func (p *Proc) IsRealmProc() bool {
	return strings.Contains(p.Program, "realm")
}

func (p *Proc) String() string {
	return fmt.Sprintf("&{ Pid:%v ParentDir:%v Program:%v Dir:%v Args:%v Env:%v Type:%v Ncore:%v }", p.Pid, p.PidDir, p.Program, p.Dir, p.Args, p.Env, p.Type, p.Ncore)
}
