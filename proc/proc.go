package proc

import (
	"fmt"
	"log"
	"path"
	"strings"

	"ulambda/namespace"
	np "ulambda/ninep"
)

type Tpid string
type Ttype uint32
type Tcore uint32

const (
	T_BE Ttype = 0
	T_LC Ttype = 1
)

const (
	C_DEF Tcore = 0
)

func (pid Tpid) String() string {
	return string(pid)
}

type Proc struct {
	Pid          Tpid     // SigmaOS PID
	ProcDir      string   // SigmaOS directory to store this proc's state
	ParentDir    string   // SigmaOS parent proc directory
	Program      string   // Program to run
	LinuxRoot    string   // Path to which this proc will be chroot-ed
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
	pid := GenPid()
	return MakeProcPid(pid, program, args)
}

func MakeProcPid(pid Tpid, program string, args []string) *Proc {
	p := &Proc{}
	p.Pid = pid
	p.Program = program
	p.LinuxRoot = path.Join(namespace.NAMESPACE_DIR, p.Pid.String())
	p.Args = args
	p.Type = T_BE
	p.Ncore = C_DEF
	p.setProcDir("")
	// If this isn't a user proc, version it.
	if !p.IsPrivilegedProc() {
		// Check the version has been set.
		if Version == "none" {
			log.Fatalf("FATAL %v %v Version not set. Please set by running with --version", GetName(), GetPid())
		}
		// Set the Program to user/VERSION/prog.bin
		p.Program = path.Join(path.Dir(p.Program), Version, path.Base(p.Program))
	}
	p.setBaseEnv()
	return p
}

// Called by procclnt to set the parent dir when spawning.
func (p *Proc) SetParentDir(parentdir string) {
	if parentdir == PROCDIR {
		p.ParentDir = path.Join(GetProcDir(), CHILDREN, p.Pid.String())
	} else {
		p.ParentDir = path.Join(parentdir, CHILDREN, p.Pid.String())
	}
}

func (p *Proc) setProcDir(procdIp string) {
	if p.IsPrivilegedProc() {
		p.ProcDir = path.Join(KPIDS, p.Pid.String())
	} else {
		if procdIp != "" {
			p.ProcDir = path.Join(np.PROCD, procdIp, PIDS, p.Pid.String())
		}
	}
}

func (p *Proc) AppendEnv(name, val string) {
	p.Env = append(p.Env, name+"="+val)
}

// Set the envvars which can be set at proc creation time.
func (p *Proc) setBaseEnv() {
	p.AppendEnv(SIGMAPRIVILEGEDPROC, fmt.Sprintf("%v", p.IsPrivilegedProc()))
	p.AppendEnv(SIGMAPID, p.Pid.String())
	p.AppendEnv(SIGMAPROGRAM, p.Program)
	p.AppendEnv(SIGMANEWROOT, p.LinuxRoot)
}

// Finalize env details which can only be set once a physical machine has been
// chosen.
func (p *Proc) FinalizeEnv(procdIp string) {
	// Set the procdir based on procdIp
	p.setProcDir(procdIp)
	p.AppendEnv(SIGMAPROCDIP, procdIp)
	p.AppendEnv(SIGMANODEDID, GetNodedId())
	p.AppendEnv(SIGMAPROCDIR, p.ProcDir)
	p.AppendEnv(SIGMAPARENTDIR, p.ParentDir)
	// Pass through debug/performance vars.
	p.AppendEnv(SIGMAPERF, GetSigmaPerf())
	p.AppendEnv(SIGMADEBUG, GetSigmaDebug())
}

func (p *Proc) GetEnv() []string {
	env := []string{}
	for _, envvar := range p.Env {
		env = append(env, envvar)
	}
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
	return fmt.Sprintf("&{ Pid:%v Program:%v ProcDir:%v ParentDir:%v UnixDir:%v Args:%v Env:%v Type:%v Ncore:%v }", p.Pid, p.Program, p.ProcDir, p.ParentDir, "Abcd", p.Args, p.GetEnv(), p.Type, p.Ncore)
}
