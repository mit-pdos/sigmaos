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

type Proc struct {
	Pid       string   // SigmaOS PID
	ProcDir   string   // SigmaOS directory to store this proc's state
	ParentDir string   // SigmaOS parent proc directory
	Program   string   // Program to run
	Dir       string   // Unix working directory for the process
	Args      []string // Args
	env       []string // Environment variables
	Type      Ttype    // Type
	Ncore     Tcore    // Number of cores requested
}

func MakeEmptyProc() *Proc {
	p := &Proc{}
	return p
}

func MakeProc(program string, args []string) *Proc {
	p := &Proc{}
	p.Pid = GenPid()
	p.ProcDir = path.Join("pids", p.Pid) // TODO: make relative to ~procd
	p.ParentDir = path.Join(GetProcDir(), CHILDREN, p.Pid)
	p.Program = program
	p.Args = args
	p.Type = T_DEF
	p.Ncore = C_DEF
	return p
}

func MakeProcPid(pid string, program string, args []string) *Proc {
	p := MakeProc(program, args)
	p.Pid = pid
	p.ProcDir = path.Join("pids", p.Pid) // TODO: make relative to ~procd
	p.ParentDir = path.Join(GetProcDir(), CHILDREN, p.Pid)
	return p
}

func (p *Proc) AppendEnv(name, val string) {
	p.env = append(p.env, name+"="+val)
}

func (p *Proc) GetEnv(procdIp, newRoot string) []string {
	env := []string{}
	for _, envvar := range p.env {
		env = append(env, envvar)
	}
	env = append(env, SIGMANEWROOT+"="+newRoot)
	env = append(env, SIGMAPROCDIP+"="+procdIp)
	env = append(env, SIGMAPID+"="+p.Pid)
	env = append(env, SIGMAPROCDIR+"="+p.ProcDir)
	env = append(env, SIGMAPARENTDIR+"="+p.ParentDir)
	return env
}

func (p *Proc) IsKernelProc() bool {
	return strings.Contains(p.Program, "kernel")
}

func (p *Proc) IsRealmProc() bool {
	return strings.Contains(p.Program, "realm")
}

func (p *Proc) String() string {
	return fmt.Sprintf("&{ Pid:%v ParentDir:%v Program:%v ProcDir:%v ParentDir:%v Args:%v Env:%v Type:%v Ncore:%v }", p.Pid, p.ProcDir, p.ParentDir, p.Program, p.Dir, p.Args, p.GetEnv("NOPROCDIP", "NONEWROOT"), p.Type, p.Ncore)
}
