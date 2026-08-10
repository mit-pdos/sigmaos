package proc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	rpcproto "sigmaos/rpc/proto"
	sp "sigmaos/sigmap"
)

type Ttype uint32 // If this type changes, make sure to change the typecasts below.

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
		log.Fatalf("FATAL Unknown proc type: %v", int(t))
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
	sync.Mutex
	*ProcProto
}

func NewEmptyProc() *Proc {
	p := &Proc{}
	p.ProcProto = &ProcProto{}
	return p
}

func NewProc(program string, args []string) *Proc {
	pid := sp.GenPid(program)
	return NewProcPid(pid, program, args)
}

func NewPrivProcPid(pid sp.Tpid, program string, args []string, priv bool) *Proc {
	p := &Proc{}
	p.ProcProto = &ProcProto{}
	procdir := sp.NOT_SET
	if priv {
		// If this is a privileged proc, we already know its procdir.
		procdir = KProcDir(pid)
	}
	p.ProcEnvProto = NewProcEnv(
		program,
		pid,
		sp.Trealm(sp.NOT_SET),
		sp.NewPrincipal(
			sp.TprincipalID(pid.String()),
			sp.Trealm(sp.NOT_SET),
		),
		procdir,
		sp.NOT_SET,
		priv,
		false,
		false,
	).GetProto()
	p.Args = args
	p.TypeInt = uint32(T_BE)
	p.ResourceRes = NewResourceReservationProto(0, 0)
	p.CoSandboxResourceRes = NewResourceReservationProto(0, 0)
	if p.ProcEnvProto.Privileged {
		p.TypeInt = uint32(T_LC)
	}
	p.Env = make(map[string]string)
	p.AddedBins = nil
	p.setBaseEnv()
	return p
}

func NewProcPid(pid sp.Tpid, program string, args []string) *Proc {
	return NewPrivProcPid(pid, program, args, false)
}

func NewProcFromProto(p *ProcProto) *Proc {
	return &Proc{ProcProto: p}
}

func (p *Proc) GetProto() *ProcProto {
	p.Lock()
	defer p.Unlock()
	return p.ProcProto
}

func (p *Proc) AppendEnv(name, val string) {
	p.Env[name] = val
}

func (p *Proc) LookupEnv(name string) (string, bool) {
	s, ok := p.Env[name]
	return s, ok
}

func (p *Proc) GetSecrets() map[string]*sp.SecretProto {
	return p.ProcEnvProto.GetSecrets()
}

func (p *Proc) GetVersion() string {
	return p.ProcEnvProto.GetVersion()
}

func (p *Proc) GetAddedBins() []*AddedBinProto {
	return p.AddedBins
}

func (p *Proc) AddBin(versionedProgram string) {
	p.AddedBins = append(p.AddedBins, &AddedBinProto{Program: versionedProgram})
}

func (p *Proc) AddCompressedBin(versionedProgram string) {
	p.AddedBins = append(p.AddedBins, &AddedBinProto{Program: versionedProgram, Compressed: true})
}

func (p *Proc) InheritParentProcEnv(parentPE *ProcEnv) {
	p.ProcEnvProto.SetRealm(parentPE.GetRealm())
	p.ProcEnvProto.ParentDir = filepath.Join(parentPE.ProcDir, CHILDREN, p.GetPid().String())
	p.ProcEnvProto.EtcdEndpoints = parentPE.EtcdEndpoints
	p.ProcEnvProto.Perf = parentPE.Perf
	p.ProcEnvProto.Debug = parentPE.Debug
	p.ProcEnvProto.BuildTag = parentPE.BuildTag
	p.ProcEnvProto.Version = parentPE.Version
	p.ProcEnvProto.UseSPProxy = p.ProcEnvProto.UseSPProxy || parentPE.UseSPProxy
	// Don't override intentionally set net proxy settings
	p.ProcEnvProto.UseDialProxy = parentPE.UseDialProxy || p.ProcEnvProto.UseDialProxy
	p.ProcEnvProto.SigmaPath = append(p.ProcEnvProto.SigmaPath, parentPE.SigmaPath...)
	// If parent didn't specify secrets, inherit the parent's secrets
	if p.ProcEnvProto.SecretsMap == nil {
		p.ProcEnvProto.SecretsMap = parentPE.SecretsMap
	}
}

func (p *Proc) GetPrincipal() *sp.Tprincipal {
	return p.ProcEnvProto.GetPrincipal()
}

func (p *Proc) SetKernelID(kernelID string, setProcDir bool) {
	p.Lock()
	defer p.Unlock()

	p.ProcEnvProto.KernelID = kernelID
	if setProcDir {
		p.setProcDir(kernelID)
	}
}

func (p *Proc) SetRealm(realm sp.Trealm) {
	p.ProcEnvProto.SetRealm(realm)
}

func (p *Proc) SetRealmSwitch(realm sp.Trealm) {
	p.ProcEnvProto.SetRealmSwitch(realm)
}

func (p *Proc) SetKernels(kernels []string) {
	p.ProcEnvProto.Kernels = kernels
}

func (p *Proc) HasNoKernelPref() bool {
	return len(p.ProcEnvProto.Kernels) == 0
}

func (p *Proc) HasKernelPref(kernelID string) bool {
	for _, k := range p.ProcEnvProto.Kernels {
		if k == kernelID {
			return true
		}
	}
	return false
}

func (p *Proc) PrependSigmaPath(pn string) {
	p.ProcEnvProto.PrependSigmaPath(pn)
}

// Finalize env details which can only be set once a physical machine and
// procd container have been chosen.
func (p *Proc) FinalizeEnv(innerIP sp.Tip, outerIP sp.Tip, procdPid sp.Tpid) {
	p.Lock()
	defer p.Unlock()

	p.ProcEnvProto.InnerContainerIPStr = innerIP.String()
	p.ProcEnvProto.OuterContainerIPStr = outerIP.String()
	p.ProcEnvProto.SetProcdPID(procdPid)
	oldr := p.GetRealm()
	// If a realm switch was requested, perform the realm switch before
	// marshaling the proc's ProcEnv. A realm switch is only possible if the
	// original realm is the root realm, and assumes that authorization checks
	// have already taken place (a proc cannot fake being part of the root realm,
	// originally)
	if newr, ok := p.ProcEnvProto.GetRealmSwitch(); ok {
		p.SetRealm(newr)
		// Clear the cached named endpoint, since it corresponds to the named
		// endpoint for the realm the proc *used* to belong to
		p.ProcEnvProto.ClearNamedEndpoint()
	}
	p.AppendEnv(SIGMACONFIG, NewProcEnvFromProto(p.ProcEnvProto).Marshal())
	// Marshal the principal ID
	b, err := json.Marshal(p.GetPrincipal())
	if err != nil {
		log.Fatalf("FATAL Error marshal principal: %v", err)
	}
	// Add marshaled principal ID to env
	p.AppendEnv(SIGMAPRINCIPAL, string(b))
	// Restore old realm
	p.SetRealm(oldr)
}

func (p *Proc) IsPrivileged() bool {
	return p.ProcEnvProto.Privileged
}

func (p *Proc) String() string {
	return fmt.Sprintf("&{ "+
		"Program:%v "+
		"Version:%v "+
		"Pid:%v "+
		"Tag:%v "+
		"Priv:%t "+
		"SigmaPath:%v "+
		"KernelId:%v "+
		"UseSPProxy:%v "+
		"UseDialProxy:%v "+
		"Realm:%v "+
		"Perf:%v "+
		"InnerIP:%v "+
		"OuterIP:%v "+
		"Args:%v "+
		"Type:%v "+
		"ResourceRerervation:%v "+
		"CoSandboxResourceRerervation:%v "+
		"Queueable:%v "+
		"Kernels:%v "+
		"}",
		p.ProcEnvProto.Program,
		p.ProcEnvProto.Version,
		p.ProcEnvProto.GetPID(),
		p.ProcEnvProto.GetBuildTag(),
		p.ProcEnvProto.Privileged,
		p.ProcEnvProto.GetSigmaPath(),
		p.ProcEnvProto.KernelID,
		p.ProcEnvProto.UseSPProxy,
		p.ProcEnvProto.UseDialProxy,
		p.ProcEnvProto.GetRealm(),
		p.ProcEnvProto.GetPerf(),
		p.ProcEnvProto.GetInnerContainerIP(),
		p.ProcEnvProto.GetOuterContainerIP(),
		p.Args,
		p.GetType(),
		p.GetResourceReservation(),
		p.GetCoSandboxResourceReservation(),
		p.GetIsQueueable(),
		p.ProcEnvProto.Kernels,
	)
}

// ========== Special getters and setters (for internal use) ==========
func (p *Proc) setProcDir(kernelId string) {
	// Privileged procs have their ProcDir (sp.KPIDS) set at the time of creation
	// of the proc struct.
	if !p.IsPrivileged() {
		p.ProcEnvProto.ProcDir = filepath.Join(sp.MSCHED, kernelId, sp.PIDS, p.GetPid().String())
	}
}

// Set the envvars which can be set at proc creation time.
func (p *Proc) setBaseEnv() {
	// Pass through debug/performance vars.
	p.AppendEnv(SIGMAPERF, GetSigmaPerf())
	p.AppendEnv(SIGMADEBUG, GetSigmaDebug())
	p.AppendEnv(SIGMADEBUGPROCS, GetSigmaDebugProcs())
	p.AppendEnv(SIGMAFAIL, GetSigmaFail())
	p.AppendEnv(SIGMADEBUGPID, p.GetPid().String())
	if p.IsPrivileged() {
		p.AppendEnv("PATH", os.Getenv("PATH")) // inherit linux path from boot
	}
}

// ========== Getters and Setters ==========

func (p *Proc) GetProcEnv() *ProcEnv {
	return NewProcEnvFromProto(p.ProcEnvProto)
}

func (p *Proc) GetProgram() string {
	return p.ProcEnvProto.Program
}

func (p *Proc) GetVersionedProgram() string {
	// Kernel procs, including named, are not versioned
	if p.IsPrivileged() || p.GetProgram() == "named" {
		return p.GetProgram()
	}
	return p.GetProgram() + "-v" + p.GetVersion()
}

func (p *Proc) GetSigmaPath() []string {
	return p.ProcEnvProto.SigmaPath
}

func (p *Proc) GetProcDir() string {
	if p.ProcEnvProto.ProcDir == sp.NOT_SET {
		b := debug.Stack()
		log.Fatalf("Error, getting unset proc dir: %v", string(b))
	}
	return p.ProcEnvProto.ProcDir
}

func (p *Proc) GetParentDir() string {
	return p.ProcEnvProto.ParentDir
}

func (p *Proc) GetPid() sp.Tpid {
	return p.ProcEnvProto.GetPID()
}

func (p *Proc) GetType() Ttype {
	return Ttype(p.ProcProto.TypeInt)
}

func (p *Proc) GetMcpu() Tmcpu {
	mcpu := p.ProcProto.ResourceRes.GetMcpu()
	// Sanity check
	if mcpu > 0 && mcpu%10 != 0 {
		log.Fatalf("%v FATAL: Error! Suspected missed MCPU conversion in GetMcpu: %v", GetSigmaDebugPid(), mcpu)
	}
	return mcpu
}

func (p *Proc) GetCoSandboxMcpu() Tmcpu {
	mcpu := p.ProcProto.CoSandboxResourceRes.GetMcpu()
	// Sanity check
	if mcpu > 0 && mcpu%10 != 0 {
		log.Fatalf("%v FATAL: Error! Suspected missed MCPU conversion in GetCoSandboxMcpu: %v", GetSigmaDebugPid(), mcpu)
	}
	return mcpu
}

func (p *Proc) GetMem() Tmem {
	return p.ProcProto.ResourceRes.GetMem()
}

func (p *Proc) GetCoSandboxMem() Tmem {
	return p.ProcProto.CoSandboxResourceRes.GetMem()
}

func (p *Proc) GetRealm() sp.Trealm {
	return p.ProcEnvProto.GetRealm()
}

func (p *Proc) GetBuildTag() string {
	return p.ProcEnvProto.BuildTag
}

func (p *Proc) GetKernelID() string {
	return p.ProcEnvProto.KernelID
}

func (p *Proc) SetType(t Ttype) {
	p.ProcProto.TypeInt = uint32(t)
}

func (p *Proc) SetSpawnTime(t time.Time) {
	p.ProcEnvProto.SetSpawnTime(t)
}

func (p *Proc) GetSpawnTime() time.Time {
	return p.ProcEnvProto.GetSpawnTime()
}

func (p *Proc) SetHow(n Thow) {
	p.ProcEnvProto.SetHow(n)
}

func (p *Proc) GetHow() Thow {
	return p.ProcEnvProto.GetHow()
}

func (p *Proc) GetIsQueueable() bool {
	return p.GetRunAfterCoSandbox()
}

func (p *Proc) GetQueueableResourcePoolID() uint64 {
	return p.QueueableResourcePoolID
}

func (p *Proc) SetQueueableResourcePoolID(id uint64) {
	p.QueueableResourcePoolID = id
}

func (p *Proc) GetRunAfterCoSandbox() bool {
	return p.RunAfterCoSandbox
}

func (p *Proc) SetRunAfterCoSandbox(runAfter bool) {
	p.RunAfterCoSandbox = runAfter
}

func (p *Proc) SetMSchedEndpoint(ep *sp.Tendpoint) {
	p.Lock()
	defer p.Unlock()

	p.SetCachedEndpoint(sp.MSCHEDREL, ep)
}

func (p *Proc) SetNamedEndpoint(ep *sp.Tendpoint) {
	p.SetCachedEndpoint(sp.NAMEDREL, ep)
}

func (p *Proc) SetCachedEndpoint(pn string, ep *sp.Tendpoint) {
	p.ProcEnvProto.SetCachedEndpoint(pn, ep)
}

func (p *Proc) ClearCachedEndpoint(pn string) {
	p.ProcEnvProto.ClearCachedEndpoint(pn)
}

func (p *Proc) GetNamedEndpoint() *sp.TendpointProto {
	ep, _ := p.ProcEnvProto.GetNamedEndpoint()
	return ep.GetProto()
}

// GetCoSandbox is the co-sandbox carried inline by SetCoSandbox, and nil for a
// proc that named one instead (GetCoSandboxPath).
func (p *Proc) GetCoSandbox() []byte {
	if p.Blob == nil || len(p.Blob.Iov) == 0 {
		return nil
	}
	return p.Blob.Iov[0]
}

func (p *Proc) GetCoSandboxInput() []byte {
	return p.CoSandboxInput
}

// SetCoSandbox attaches a co-sandbox to this proc by value: the bytes travel
// with it on every hop of the spawn path. Fine for a proc spawned once, and the
// only option for a co-sandbox that was never uploaded (a test building one on
// the fly); prefer SetCoSandboxPath when spawning many procs with the same
// co-sandbox.
func (p *Proc) SetCoSandbox(b []byte, input []byte) {
	p.Blob = &rpcproto.Blob{
		Iov: [][]byte{b},
	}
	p.CoSandboxInput = input
}

// SetCoSandboxPath attaches a co-sandbox to this proc by name: procd fetches it
// through chunksrv and caches it, so N procs sharing a co-sandbox transfer it
// once per node instead of once per proc. pn is a sigma pathname; build it with
// wasmer.CoSandboxPath so the spawner and procd agree.
func (p *Proc) SetCoSandboxPath(pn string, input []byte) {
	p.CoSandboxPath = pn
	p.CoSandboxInput = input
}

func (p *Proc) GetCoSandboxPath() string {
	return p.CoSandboxPath
}

// SetCoSandboxLocalPath records where procd put the binary it fetched, in the
// filesystem procd and spproxyd share. Set by procd only.
func (p *Proc) SetCoSandboxLocalPath(pn string) {
	p.CoSandboxLocalPath = pn
}

func (p *Proc) GetCoSandboxLocalPath() string {
	return p.CoSandboxLocalPath
}

func (p *Proc) SetRunCoSandbox(run bool) {
	p.ProcEnvProto.SetRunCoSandbox(run)
}

func (p *Proc) GetRunCoSandbox() bool {
	return p.ProcEnvProto.GetRunCoSandbox()
}

func (p *Proc) SetProcContainerType(t ProcContainerType) {
	p.ProcEnvProto.ContainerType = t
}

func (p *Proc) GetProcContainerType() ProcContainerType {
	return p.ProcEnvProto.GetContainerType()
}

func (p *Proc) GetShmemEnabled() bool {
	return p.ProcEnvProto.GetShmemEnabled()
}

func (p *Proc) SetShmemMB(mb Tmem) {
	p.ProcEnvProto.SetShmemMB(mb)
}

func (p *Proc) GetShmemMB() Tmem {
	return p.ProcEnvProto.GetShmemMB()
}

func (p *Proc) SetCoSandboxBufMB(mb Tmem) {
	p.CosandboxBufMBInt = uint64(mb)
}

func (p *Proc) GetCoSandboxBufMB() Tmem {
	return Tmem(p.CosandboxBufMBInt)
}

func (p *Proc) SetWasmBufMB(mb Tmem) {
	p.WasmBufMBInt = uint64(mb)
}

func (p *Proc) GetWasmBufMB() Tmem {
	return Tmem(p.WasmBufMBInt)
}

func (p *Proc) SetMeasurePSS(measure bool, delayMS int) {
	p.MeasurePSS = measure
	p.MeasurePSSDelayMS = uint64(delayMS)
}

func (p *Proc) GetResourceReservation() *ResourceReservation {
	return &ResourceReservation{p.ResourceRes}
}

func (p *Proc) GetCoSandboxResourceReservation() *ResourceReservation {
	return &ResourceReservation{p.CoSandboxResourceRes}
}

// Return Env map as a []string
func (p *Proc) GetEnv() []string {
	env := []string{}
	for key, envvar := range p.Env {
		env = append(env, key+"="+envvar)
	}
	return env
}

func (p *Proc) UpdateEnv(env []string) {
	for _, e := range env {
		kv := strings.Split(e, "=")
		if len(kv) == 2 {
			p.Env[kv[0]] = kv[1]
		}
	}
}

// Set the number of cores on this proc. If > 0, then this proc is LC. For now,
// LC procs necessarily must specify LC > 1.
func (p *Proc) SetMcpu(mcpu Tmcpu) {
	if mcpu > Tmcpu(0) {
		if mcpu%10 != 0 {
			log.Fatalf("%v FATAL: Error! Suspected missed MCPU conversion in GetMcpu: %v", GetSigmaDebugPid(), mcpu)
		}
		p.TypeInt = uint32(T_LC)
		p.ResourceRes.SetMcpu(mcpu)
	}
}

func (p *Proc) SetCoSandboxMcpu(mcpu Tmcpu) {
	p.CoSandboxResourceRes.SetMcpu(mcpu)
}

// Set the aendpoint of memory (in MB) required to run this proc.
func (p *Proc) SetMem(mb Tmem) {
	p.ResourceRes.SetMem(mb)
}

func (p *Proc) SetCoSandboxMem(mb Tmem) {
	p.CoSandboxResourceRes.SetMem(mb)
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
