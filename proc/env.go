package proc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	sp "sigmaos/sigmap"
)

// Environment variables for procs (SHOULD NOT BE ADDED TO)
const (
	SIGMASTRACE   = "SIGMASTRACE"
	SIGMADEBUGPID = "SIGMADEBUGPID"
	SIGMAPERF     = "SIGMAPERF"
	SIGMADEBUG    = "SIGMADEBUG"
	SIGMACONFIG   = "SIGMACONFIG"
)

const (
	NOT_SET = "NOT_SET" // Catch cases where we fail to set a variable.
)

type ProcEnv struct {
	*ProcEnvProto
}

func GetProcEnv() *ProcEnv {
	pestr := os.Getenv(SIGMACONFIG)
	if pestr == "" {
		stack := debug.Stack()
		log.Fatalf("%s\nError: No Sigma Config", stack)
	}
	return Unmarshal(pestr)
}

func SetSigmaDebugPid(pid string) {
	os.Setenv(SIGMADEBUGPID, pid)
}

func GetSigmaDebugPid() string {
	return os.Getenv(SIGMADEBUGPID)
}

func GetSigmaPerf() string {
	return os.Getenv(SIGMAPERF)
}

func GetSigmaDebug() string {
	return os.Getenv(SIGMADEBUG)
}

func GetLabelsEnv(envvar string) map[string]bool {
	s := os.Getenv(envvar)
	return GetLabels(s)
}

func GetLabels(s string) map[string]bool {
	m := make(map[string]bool)
	if s == "" {
		return m
	}
	labels := strings.Split(s, ";")
	for _, l := range labels {
		m[l] = true
	}
	return m
}

func NewProcEnv(program string, pid sp.Tpid, realm sp.Trealm, uname sp.Tuname, procDir string, parentDir string, priv, overlays, useSigmaclntd bool) *ProcEnv {
	// Load Perf & Debug from the environment for convenience.
	return &ProcEnv{
		ProcEnvProto: &ProcEnvProto{
			PidStr:        string(pid),
			RealmStr:      string(realm),
			UnameStr:      string(uname),
			ProcDir:       procDir,
			ParentDir:     parentDir,
			Program:       program,
			LocalIPStr:    NOT_SET,
			KernelID:      NOT_SET,
			BuildTag:      NOT_SET,
			Net:           NOT_SET,
			Perf:          os.Getenv(SIGMAPERF),
			Strace:        os.Getenv(SIGMASTRACE),
			Debug:         os.Getenv(SIGMADEBUG),
			UprocdPIDStr:  NOT_SET,
			Privileged:    priv,
			Overlays:      overlays,
			UseSigmaclntd: useSigmaclntd,
			IDStr:         NOT_SET,
		},
	}
}

func NewProcEnvUnset(priv, overlays bool) *ProcEnv {
	// Load Perf & Debug from the environment for convenience.
	return NewProcEnv(NOT_SET, sp.Tpid(NOT_SET), sp.Trealm(NOT_SET), sp.Tuname(NOT_SET), NOT_SET, NOT_SET, priv, overlays, false)
}

func NewProcEnvFromProto(p *ProcEnvProto) *ProcEnv {
	return &ProcEnv{p}
}

func NewBootProcEnv(uname sp.Tuname, etcdIP string, localIP sp.Thost, buildTag string, overlays bool) *ProcEnv {
	pe := NewProcEnvUnset(true, overlays)
	pe.SetUname(uname)
	pe.Program = "kernel"
	pe.SetPID(sp.GenPid(string(uname)))
	pe.EtcdIP = etcdIP
	pe.LocalIPStr = localIP.String()
	pe.BuildTag = buildTag
	pe.SetRealm(sp.ROOTREALM, overlays)
	pe.ProcDir = path.Join(sp.KPIDS, pe.GetPID().String())
	pe.Privileged = true
	pe.HowInt = int32(BOOT)
	return pe
}

func NewTestProcEnv(realm sp.Trealm, etcdIP string, localIP sp.Thost, buildTag string, overlays, useSigmaclntd bool) *ProcEnv {
	pe := NewProcEnvUnset(true, overlays)
	pe.SetUname("test")
	pe.SetPID(sp.GenPid("test"))
	pe.SetRealm(realm, overlays)
	pe.EtcdIP = etcdIP
	pe.LocalIPStr = localIP.String()
	pe.BuildTag = buildTag
	pe.Program = "test"
	pe.ProcDir = path.Join(sp.KPIDS, pe.GetPID().String())
	pe.HowInt = int32(TEST)
	pe.UseSigmaclntd = useSigmaclntd
	return pe
}

// Create a new sigma config which is a derivative of an existing sigma config.
func NewAddedProcEnv(pe *ProcEnv, idx int) *ProcEnv {
	pe2 := NewProcEnvUnset(pe.Privileged, false)
	*(pe2.ProcEnvProto) = *(pe.ProcEnvProto)
	pe2.SetUname(sp.Tuname(string(pe.GetUname()) + "-clnt-" + strconv.Itoa(idx)))
	return pe2
}

func NewDifferentRealmProcEnv(pe *ProcEnv, realm sp.Trealm) *ProcEnv {
	pe2 := NewProcEnvUnset(pe.Privileged, pe.Overlays)
	*(pe2.ProcEnvProto) = *(pe.ProcEnvProto)
	pe2.SetRealm(realm, pe.Overlays)
	pe2.SetUname(sp.Tuname(string(pe.GetUname()) + "-realm-" + realm.String()))
	return pe2
}

func (pe *ProcEnvProto) GetPID() sp.Tpid {
	return sp.Tpid(pe.PidStr)
}

func (pe *ProcEnvProto) SetPID(pid sp.Tpid) {
	pe.PidStr = string(pid)
}

func (pe *ProcEnvProto) SetLocalIP(host sp.Thost) {
	pe.LocalIPStr = host.String()
}

func (pe *ProcEnvProto) GetLocalIP() sp.Thost {
	return sp.Thost(pe.LocalIPStr)
}

func (pe *ProcEnvProto) GetRealm() sp.Trealm {
	return sp.Trealm(pe.RealmStr)
}

func (pe *ProcEnvProto) SetRealm(realm sp.Trealm, overlays bool) {
	pe.RealmStr = string(realm)
	// Changing the realm changes the overlay network name. Therefore, set the
	// overlay network for the new realm.
	pe.Net = sp.ROOTREALM.String()
	if overlays {
		pe.Net = "sigmanet-" + realm.String()
		if realm == sp.ROOTREALM {
			pe.Net = "sigmanet-testuser"
		}
	}
}

func (pe *ProcEnvProto) GetUname() sp.Tuname {
	return sp.Tuname(pe.UnameStr)
}

func (pe *ProcEnvProto) SetUname(uname sp.Tuname) {
	pe.UnameStr = string(uname)
}

func (pe *ProcEnvProto) SetUprocdPID(pid sp.Tpid) {
	pe.UprocdPIDStr = string(pid)
}

func (pe *ProcEnvProto) GetUprocdPID() sp.Tpid {
	return sp.Tpid(pe.UprocdPIDStr)
}

func (pe *ProcEnv) GetProto() *ProcEnvProto {
	return pe.ProcEnvProto
}

func (pe *ProcEnvProto) SetNetFail(nf int64) {
	pe.NetFail = nf
}

func (pe *ProcEnvProto) SetCrash(nf int64) {
	pe.Crash = nf
}

func (pe *ProcEnvProto) SetPartition(nf int64) {
	pe.Partition = nf
}

func (pe *ProcEnvProto) SetHow(how Thow) {
	pe.HowInt = int32(how)
}

func (pe *ProcEnvProto) GetHow() Thow {
	return Thow(pe.HowInt)
}

func (pe *ProcEnvProto) SetSpawnTime(t time.Time) {
	pe.SpawnTimePB = timestamppb.New(t)
}

func (pe *ProcEnvProto) GetSpawnTime() time.Time {
	return pe.SpawnTimePB.AsTime()
}

func (pe *ProcEnv) GetNamedMount() (sp.Tmount, bool) {
	mnt := pe.ProcEnvProto.GetNamedMountProto()
	if mnt == nil {
		return sp.Tmount{}, false
	}
	return sp.Tmount{mnt}, true
}

func (pe *ProcEnv) Marshal() string {
	b, err := json.Marshal(pe)
	if err != nil {
		log.Fatalf("FATAL Error marshal sigmaconfig: %v")
	}
	return string(b)
}

func Unmarshal(pestr string) *ProcEnv {
	pe := &ProcEnv{}
	err := json.Unmarshal([]byte(pestr), pe)
	if err != nil {
		log.Fatalf("FATAL Error unmarshal ProcEnv %v", err)
	}
	return pe
}

// TODO: cleanup
func (pe *ProcEnv) String() string {
	return fmt.Sprintf("&{ Program: %v Pid:%v Realm:%v Uname:%v KernelID:%v UprocdPID:%v Net:%v ProcDir:%v ParentDir:%v How:%v Perf:%v Debug:%v EtcdIP:%v LocalIP:%v BuildTag:%v Privileged:%v Overlays:%v Crash:%v Partition:%v NetFail:%v }", pe.Program, pe.GetPID(), pe.GetRealm(), pe.GetUname(), pe.KernelID, pe.UprocdPIDStr, pe.Net, pe.ProcDir, pe.ParentDir, Thow(pe.HowInt), pe.Perf, pe.Debug, pe.EtcdIP, pe.LocalIPStr, pe.BuildTag, pe.Privileged, pe.Overlays, pe.Crash, pe.Partition, pe.NetFail)
}
