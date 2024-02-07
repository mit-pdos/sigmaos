package proc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"runtime/debug"
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
		log.Fatalf("FATAL %v: No ProcEnv\n%s", GetSigmaDebugPid(), stack)
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

func NewProcEnv(program string, pid sp.Tpid, realm sp.Trealm, principal *sp.Tprincipal, procDir string, parentDir string, priv, overlays, useSigmaclntd bool) *ProcEnv {
	// Load Perf & Debug from the environment for convenience.
	return &ProcEnv{
		ProcEnvProto: &ProcEnvProto{
			PidStr:              string(pid),
			RealmStr:            string(realm),
			Principal:           principal,
			ProcDir:             procDir,
			ParentDir:           parentDir,
			Program:             program,
			InnerContainerIPStr: NOT_SET,
			OuterContainerIPStr: NOT_SET,
			KernelID:            NOT_SET,
			BuildTag:            NOT_SET,
			Net:                 NOT_SET,
			Perf:                os.Getenv(SIGMAPERF),
			Strace:              os.Getenv(SIGMASTRACE),
			Debug:               os.Getenv(SIGMADEBUG),
			UprocdPIDStr:        NOT_SET,
			Privileged:          priv,
			Overlays:            overlays,
			UseSigmaclntd:       useSigmaclntd,
			Claims: &ProcClaimsProto{
				PrincipalIDStr: principal.ID,
				AllowedPaths:   nil, // By default, will be set to the parent's AllowedPaths unless otherwise specified
				Secrets:        nil, // By default, will be set to the parent's Secrets unless otherwise specified
			},
		},
	}
}

func NewProcEnvUnset(priv, overlays bool) *ProcEnv {
	// Load Perf & Debug from the environment for convenience.
	return NewProcEnv(NOT_SET, sp.Tpid(NOT_SET), sp.Trealm(NOT_SET), &sp.Tprincipal{ID: NOT_SET}, NOT_SET, NOT_SET, priv, overlays, false)
}

func NewProcEnvFromProto(p *ProcEnvProto) *ProcEnv {
	return &ProcEnv{p}
}

func NewBootProcEnv(principal *sp.Tprincipal, secrets map[string]*ProcSecretProto, etcdIP sp.Tip, innerIP sp.Tip, outerIP sp.Tip, buildTag string, overlays bool) *ProcEnv {
	pe := NewProcEnvUnset(true, overlays)
	pe.SetPrincipal(principal)
	// Allow all paths for boot env
	pe.SetAllowedPaths([]string{"*"})
	pe.Program = "kernel"
	pe.SetPID(sp.GenPid(principal.ID))
	pe.EtcdIP = string(etcdIP)
	pe.InnerContainerIPStr = innerIP.String()
	pe.OuterContainerIPStr = outerIP.String()
	pe.BuildTag = buildTag
	pe.SetRealm(sp.ROOTREALM, overlays)
	pe.ProcDir = path.Join(sp.KPIDS, pe.GetPID().String())
	pe.Privileged = true
	pe.HowInt = int32(BOOT)
	return pe
}

func NewTestProcEnv(realm sp.Trealm, secrets map[string]*ProcSecretProto, etcdIP sp.Tip, innerIP sp.Tip, outerIP sp.Tip, buildTag string, overlays, useSigmaclntd bool) *ProcEnv {
	pe := NewProcEnvUnset(true, overlays)
	pe.SetPrincipal(&sp.Tprincipal{
		ID:       "test",
		TokenStr: NOT_SET,
	})
	pe.SetSecrets(secrets)
	// Allow all paths for boot env
	pe.SetAllowedPaths([]string{"*"})
	pe.SetPID(sp.GenPid("test"))
	pe.SetRealm(realm, overlays)
	pe.EtcdIP = string(etcdIP)
	pe.InnerContainerIPStr = innerIP.String()
	pe.OuterContainerIPStr = outerIP.String()
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
	pe2.SetPrincipal(&sp.Tprincipal{
		ID:       pe.GetPrincipal().ID,
		TokenStr: pe.GetPrincipal().TokenStr,
	})
	return pe2
}

func NewDifferentRealmProcEnv(pe *ProcEnv, realm sp.Trealm) *ProcEnv {
	pe2 := NewProcEnvUnset(pe.Privileged, pe.Overlays)
	*(pe2.ProcEnvProto) = *(pe.ProcEnvProto)
	pe2.SetRealm(realm, pe.Overlays)
	pe2.SetPrincipal(&sp.Tprincipal{
		ID:       pe.GetPrincipal().ID + "-realm-" + realm.String(),
		TokenStr: NOT_SET,
	})
	return pe2
}

func (pe *ProcEnvProto) GetPID() sp.Tpid {
	return sp.Tpid(pe.PidStr)
}

func (pe *ProcEnvProto) SetSecrets(secrets map[string]*ProcSecretProto) {
	pe.Claims.Secrets = secrets
}

func (pe *ProcEnvProto) SetToken(token string) {
	pe.Principal.TokenStr = token
}

func (pe *ProcEnvProto) SetAllowedPaths(paths []string) {
	pe.Claims.AllowedPaths = paths
}

func (pe *ProcEnvProto) SetPID(pid sp.Tpid) {
	pe.PidStr = string(pid)
}

func (pe *ProcEnvProto) SetInnerContainerIP(ip sp.Tip) {
	pe.InnerContainerIPStr = ip.String()
}

func (pe *ProcEnvProto) GetInnerContainerIP() sp.Tip {
	return sp.Tip(pe.InnerContainerIPStr)
}

func (pe *ProcEnvProto) SetOuterContainerIP(ip sp.Tip) {
	pe.OuterContainerIPStr = ip.String()
}

func (pe *ProcEnvProto) GetOuterContainerIP() sp.Tip {
	return sp.Tip(pe.OuterContainerIPStr)
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

func (pe *ProcEnvProto) SetPrincipal(principal *sp.Tprincipal) {
	pe.Principal = principal
	pe.Claims.PrincipalIDStr = principal.ID
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
		log.Fatalf("FATAL %v: Error marshal ProcEnv: %v", GetSigmaDebugPid(), err)
	}
	return string(b)
}

func Unmarshal(pestr string) *ProcEnv {
	pe := &ProcEnv{}
	err := json.Unmarshal([]byte(pestr), pe)
	if err != nil {
		log.Fatalf("FATAL %v: Error unmarshal ProcEnv: %v", GetSigmaDebugPid(), err)
	}
	return pe
}

// TODO: cleanup
func (pe *ProcEnv) String() string {
	return fmt.Sprintf("&{ Program: %v Pid:%v Realm:%v Principal:%v KernelID:%v UprocdPID:%v Net:%v ProcDir:%v ParentDir:%v How:%v Perf:%v Debug:%v EtcdIP:%v InnerIP:%v OuterIP:%v BuildTag:%v Privileged:%v Overlays:%v Crash:%v Partition:%v NetFail:%v UseSigmacltnd:%v }", pe.Program, pe.GetPID(), pe.GetRealm(), pe.GetPrincipal().String(), pe.KernelID, pe.UprocdPIDStr, pe.Net, pe.ProcDir, pe.ParentDir, Thow(pe.HowInt), pe.Perf, pe.Debug, pe.EtcdIP, pe.InnerContainerIPStr, pe.OuterContainerIPStr, pe.BuildTag, pe.Privileged, pe.Overlays, pe.Crash, pe.Partition, pe.NetFail, pe.UseSigmaclntd)
}
