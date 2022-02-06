package proc

import (
	"fmt"
	"path"
	"strings"

	np "ulambda/ninep"
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
	Pid          string   // SigmaOS PID
	ProcDir      string   // SigmaOS directory to store this proc's state
	ParentDir    string   // SigmaOS parent proc directory
	Program      string   // Program to run
	Dir          string   // Unix working directory for the process
	Args         []string // Args
	Env          []string // Environment variables
	Type         Ttype    // Type
	Ncore        Tcore    // Number of cores requested
	sharedTarget string   // Target of shared state
}

func MakeEmptyProc() *Proc {
	p := &Proc{}
	return p
}

func MakeProc(program string, args []string) *Proc {
	p := &Proc{}
	p.Pid = GenPid()
	p.setProcDir("")
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
	p.setProcDir("")
	p.ParentDir = path.Join(GetProcDir(), CHILDREN, p.Pid)
	return p
}

func (p *Proc) setProcDir(procdIp string) {
	if p.IsPrivilegedProc() {
		p.ProcDir = path.Join(KPIDS, p.Pid)
	} else {
		if procdIp != "" {
			p.ProcDir = path.Join(np.PROCD, procdIp, PIDS, p.Pid) // TODO: make relative to ~procd
		}
	}
}

func (p *Proc) AppendEnv(name, val string) {
	p.Env = append(p.Env, name+"="+val)
}

func (p *Proc) GetEnv(procdIp, newRoot string) []string {
	// Set the procdir based on procdIp
	p.setProcDir(procdIp)

	env := []string{}
	for _, envvar := range p.Env {
		env = append(env, envvar)
	}
	env = append(env, SIGMAPRIVILEGEDPROC+"="+fmt.Sprintf("%v", p.IsPrivilegedProc()))
	env = append(env, SIGMANEWROOT+"="+newRoot)
	env = append(env, SIGMAPROCDIP+"="+procdIp)
	env = append(env, SIGMAPID+"="+p.Pid)
	env = append(env, SIGMAPROGRAM+"="+p.Program)
	env = append(env, SIGMAPROCDIR+"="+p.ProcDir)
	env = append(env, SIGMAPARENTDIR+"="+p.ParentDir)
	return env
}

func (p *Proc) SetShared(target string) {
	p.sharedTarget = target
}

func (p *Proc) GetShared() string {
	return p.sharedTarget
}

func (p *Proc) IsPrivilegedProc() bool {
	return strings.Contains(p.Program, "kernel") || strings.Contains(p.Program, "realm")
}

func (p *Proc) String() string {
	return fmt.Sprintf("&{ Pid:%v Program:%v ProcDir:%v ParentDir:%v UnixDir:%v Args:%v Env:%v Type:%v Ncore:%v }", p.Pid, p.Program, p.ProcDir, p.ParentDir, p.Dir, p.Args, p.GetEnv("", ""), p.Type, p.Ncore)
}
