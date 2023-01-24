package proc

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	sp "sigmaos/sigmap"
)

type Tpid string
type Ttype uint32 // If this type changes, make sure to change the typecasts below.
type Tcore uint32 // If this type changes, make sure to change the typecasts below.
type Tmem uint32  // If this type changes, make sure to change the typecasts below.

const (
	T_BE Ttype = 0
	T_LC Ttype = 1
)

func (t Ttype) String() string {
	switch t {
	case T_BE:
		return "T_BE"
	case T_LC:
		return "T_LC"
	default:
		log.Fatalf("Unknown proc type: %v", t)
	}
	return ""
}

func (pid Tpid) String() string {
	return string(pid)
}

type Proc struct {
	*ProcProto
}

func MakeEmptyProc() *Proc {
	p := &Proc{}
	p.ProcProto = &ProcProto{}
	return p
}

func MakeProc(program string, args []string) *Proc {
	pid := GenPid()
	return MakeProcPid(pid, program, args)
}

func MakePrivProcPid(pid Tpid, program string, args []string, priv bool) *Proc {
	p := &Proc{}
	p.ProcProto = &ProcProto{}
	p.PidStr = pid.String()
	p.Program = program
	p.Args = args
	p.TypeInt = uint32(T_BE)
	p.NcoreInt = uint32(0)
	p.Privileged = priv
	p.Env = make(map[string]string)
	p.setProcDir("")
	if !p.Privileged {
		p.TypeInt = uint32(T_LC)
	}
	p.setBaseEnv()
	return p
}

func MakeProcPid(pid Tpid, program string, args []string) *Proc {
	return MakePrivProcPid(pid, program, args, false)
}

func MakeProcFromProto(p *ProcProto) *Proc {
	return &Proc{p}
}

func (p *Proc) GetProto() *ProcProto {
	return p.ProcProto
}

// Called by procclnt to set the parent dir when spawning.
func (p *Proc) SetParentDir(parentdir string) {
	if parentdir == PROCDIR {
		p.ParentDir = path.Join(GetProcDir(), CHILDREN, p.GetPid().String())
	} else {
		p.ParentDir = path.Join(parentdir, CHILDREN, p.GetPid().String())
	}
}

func (p *Proc) setProcDir(procdIp string) {
	if p.IsPrivilegedProc() {
		p.ProcDir = path.Join(KPIDS, p.GetPid().String())
	} else {
		if procdIp != "" {
			p.ProcDir = path.Join(sp.PROCD, procdIp, PIDS, p.GetPid().String())
		}
	}
}

func (p *Proc) AppendEnv(name, val string) {
	p.Env[name] = val
}

func (p *Proc) LookupEnv(name string) (string, bool) {
	s, ok := p.Env[name]
	return s, ok
}

// Set the envvars which can be set at proc creation time.
func (p *Proc) setBaseEnv() {
	p.AppendEnv(SIGMAPRIVILEGEDPROC, fmt.Sprintf("%t", p.IsPrivilegedProc()))
	p.AppendEnv(SIGMAPID, p.GetPid().String())
	p.AppendEnv(SIGMAPROGRAM, p.Program)
	// Pass through debug/performance vars.
	p.AppendEnv(SIGMAPERF, GetSigmaPerf())
	p.AppendEnv(SIGMADEBUG, GetSigmaDebug())
	if p.Privileged {
		p.AppendEnv("PATH", os.Getenv("PATH")) // inherit linux path from boot
	}
}

// Finalize env details which can only be set once a physical machine has been
// chosen.
func (p *Proc) FinalizeEnv(procdIp string) {
	// Set the procdir based on procdIp
	p.setProcDir(procdIp)
	p.AppendEnv(SIGMALOCAL, GetSigmaLocal())
	p.AppendEnv(SIGMAPROCDIP, procdIp)
	p.AppendEnv(SIGMANODEDID, GetNodedId())
	p.AppendEnv(SIGMAPROCDIR, p.ProcDir)
	p.AppendEnv(SIGMAPARENTDIR, p.ParentDir)
}

func (p *Proc) IsPrivilegedProc() bool {
	return p.Privileged
}

func (p *Proc) String() string {
	return fmt.Sprintf("&{ Pid:%v Priv %t Program:%v ProcDir:%v ParentDir:%v UnixDir:%v Args:%v Env:%v Type:%v Ncore:%v Mem:%v }", p.GetPid(), p.Privileged, p.Program, p.ProcDir, p.ParentDir, "Abcd", p.Args, p.GetEnv(), p.GetType(), p.GetNcore(), p.GetMem())
}

// ========== Getters and Setters ==========

func (p *Proc) GetPid() Tpid {
	return Tpid(p.ProcProto.PidStr)
}

func (p *Proc) GetType() Ttype {
	return Ttype(p.ProcProto.TypeInt)
}

func (p *Proc) GetNcore() Tcore {
	return Tcore(p.ProcProto.NcoreInt)
}

func (p *Proc) GetMem() Tmem {
	return Tmem(p.ProcProto.MemInt)
}

func (p *Proc) GetRealm() sp.Trealm {
	return sp.Trealm(p.ProcProto.RealmStr)
}

func (p *Proc) SetRealm(r sp.Trealm) {
	p.ProcProto.RealmStr = r.String()
}

func (p *Proc) SetSpawnTime(t time.Time) {
	p.SpawnTimePB = timestamppb.New(t)
}

func (p *Proc) GetSpawnTime() time.Time {
	return p.SpawnTimePB.AsTime()
}

func (p *Proc) SetShared(target string) {
	p.SharedTarget = target
}

func (p *Proc) GetShared() string {
	return p.SharedTarget
}

// Return Env map as a []string
func (p *Proc) GetEnv() []string {
	env := []string{}
	for key, envvar := range p.Env {
		env = append(env, key+"="+envvar)
	}
	return env
}

// Set the number of cores on this proc. If > 0, then this proc is LC. For now,
// LC procs necessarily must specify LC > 1.
func (p *Proc) SetNcore(ncore Tcore) {
	if ncore > Tcore(0) {
		p.TypeInt = uint32(T_LC)
		p.NcoreInt = uint32(ncore)
	}
}

// Set the amount of memory (in MB) required to run this proc.
func (p *Proc) SetMem(mb Tmem) {
	p.MemInt = uint32(mb)
}

func (p *Proc) Marshal() []byte {
	b, err := proto.Marshal(p.ProcProto)
	if err != nil {
		log.Fatalf("Error marshal: %v", err)
	}
	return b
}

func (p *Proc) Unmarshal(b []byte) {
	if err := proto.Unmarshal(b, p.ProcProto); err != nil {
		log.Fatalf("Error unmarshal: %v", err)
	}
}
