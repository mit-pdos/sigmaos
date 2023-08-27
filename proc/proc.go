package proc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"sigmaos/config"
	sp "sigmaos/sigmap"
)

type Ttype uint32 // If this type changes, make sure to change the typecasts below.
type Tmcpu uint32 // If this type changes, make sure to change the typecasts below.
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

func ParseTtype(tstr string) Ttype {
	switch tstr {
	case "T_BE":
		return T_BE
	case "T_LC":
		return T_LC
	default:
		log.Fatalf("Unknown proc type: %v", tstr)
	}
	return 0
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
	pid := sp.GenPid()
	return MakeProcPid(pid, program, args)
}

func MakePrivProcPid(pid sp.Tpid, program string, args []string, priv bool) *Proc {
	p := &Proc{}
	p.ProcProto = &ProcProto{}
	p.PidStr = pid.String()
	p.Program = program
	p.Args = args
	p.TypeInt = uint32(T_BE)
	p.McpuInt = uint32(0)
	p.Privileged = priv
	if p.Privileged {
		p.TypeInt = uint32(T_LC)
	}
	p.setProcDir("NO_SCHEDD_IP")
	p.Env = make(map[string]string)
	p.setBaseEnv()
	return p
}

func MakeProcPid(pid sp.Tpid, program string, args []string) *Proc {
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

func (p *Proc) setProcDir(kernelId string) {
	if p.IsPrivilegedProc() {
		p.ProcDir = path.Join(sp.KPIDSREL, p.GetPid().String())
	} else {
		p.ProcDir = path.Join(sp.SCHEDD, kernelId, sp.PIDS, p.GetPid().String())
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
func (p *Proc) Finalize(kernelId string) {
	p.setProcDir(kernelId)
	p.AppendEnv(SIGMAKERNEL, kernelId)
	p.AppendEnv(SIGMALOCAL, GetSigmaLocal())
	//	p.AppendEnv(SIGMAPROCDIR, p.ProcDir)
	//	p.AppendEnv(SIGMAPARENTDIR, p.ParentDir)
	p.AppendEnv(SIGMAJAEGERIP, GetSigmaJaegerIP())
}

func (p *Proc) IsPrivilegedProc() bool {
	return p.Privileged
}

func (p *Proc) String() string {
	return fmt.Sprintf("&{ Program:%v Pid:%v Priv:%t KernelId:%v Realm:%v ProcDir:%v ParentDir:%v Args:%v Env:%v Type:%v Mcpu:%v Mem:%v }",
		p.Program,
		p.GetPid(),
		p.Privileged,
		p.KernelId,
		p.GetRealm(),
		p.ProcDir,
		p.ParentDir,
		p.Args,
		p.GetEnv(),
		p.GetType(),
		p.GetMcpu(),
		p.GetMem(),
	)
}

// ========== Getters and Setters ==========

func (p *Proc) SetSigmaConfig(scfg *config.SigmaConfig) {
	p.SigmaConfig = scfg.Marshal()
	// TODO: don't append every time.
	p.AppendEnv(config.SIGMACONFIG, scfg.Marshal())
}

func (p *Proc) GetSigmaConfig() *config.SigmaConfig {
	return config.Unmarshal(p.SigmaConfig)
}

func (p *Proc) GetPid() sp.Tpid {
	return sp.Tpid(p.ProcProto.PidStr)
}

func (p *Proc) GetType() Ttype {
	return Ttype(p.ProcProto.TypeInt)
}

func (p *Proc) GetMcpu() Tmcpu {
	mcpu := p.ProcProto.McpuInt
	if mcpu > 0 && mcpu%10 != 0 {
		log.Fatalf("Error! Suspected missed MCPU conversion in GetMcpu: %v", mcpu)
	}
	return Tmcpu(p.ProcProto.McpuInt)
}

func (p *Proc) GetMem() Tmem {
	return Tmem(p.ProcProto.MemInt)
}

func (p *Proc) GetRealm() sp.Trealm {
	return sp.Trealm(p.ProcProto.RealmStr)
}

func (p *Proc) SetType(t Ttype) {
	p.ProcProto.TypeInt = uint32(t)
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
func (p *Proc) SetMcpu(mcpu Tmcpu) {
	if mcpu > Tmcpu(0) {
		if mcpu%10 != 0 {
			log.Fatalf("Error! Suspected missed MCPU conversion: %v", mcpu)
		}
		p.TypeInt = uint32(T_LC)
		p.McpuInt = uint32(mcpu)
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

func (p *Proc) MarshalJson() []byte {
	b, err := json.Marshal(p.ProcProto)
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

func (p *Proc) UnmarshalJson(b []byte) {
	if err := json.Unmarshal(b, p.ProcProto); err != nil {
		log.Fatalf("Error unmarshal: %v", err)
	}
}
