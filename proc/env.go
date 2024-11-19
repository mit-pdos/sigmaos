package proc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	sp "sigmaos/sigmap"
)

// Environment variables for procs (SHOULD NOT BE ADDED TO)
const (
	SIGMASTRACE    = "SIGMASTRACE"
	SIGMADEBUGPID  = "SIGMADEBUGPID"
	SIGMAPERF      = "SIGMAPERF"
	SIGMADEBUG     = "SIGMADEBUG"
	SIGMAFAIL      = "SIGMAFAIL"
	SIGMACONFIG    = "SIGMACONFIG"
	SIGMAPRINCIPAL = "SIGMAPRINCIPAL"
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

func GetSigmaFail() string {
	return os.Getenv(SIGMAFAIL)
}

func SetSigmaFail(s string) {
	os.Setenv(SIGMAFAIL, s)
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
		if l != "" {
			m[l] = true
		}
	}
	return m
}

func NewProcEnv(program string, pid sp.Tpid, realm sp.Trealm, principal *sp.Tprincipal, procDir string, parentDir string, priv, overlays, useSPProxy bool, useNetProxy bool) *ProcEnv {
	// Load Debug, Perf, and Fail from the environment for convenience.
	return &ProcEnv{
		ProcEnvProto: &ProcEnvProto{
			PidStr:              string(pid),
			RealmStr:            string(realm),
			Principal:           principal,
			ProcDir:             procDir,
			ParentDir:           parentDir,
			Program:             program,
			InnerContainerIPStr: sp.NOT_SET,
			OuterContainerIPStr: sp.NOT_SET,
			KernelID:            sp.NOT_SET,
			BuildTag:            sp.NOT_SET,
			Version:             sp.Version,
			Perf:                os.Getenv(SIGMAPERF),
			Strace:              os.Getenv(SIGMASTRACE),
			Debug:               os.Getenv(SIGMADEBUG),
			Fail:                os.Getenv(SIGMAFAIL),
			UprocdPIDStr:        sp.NOT_SET,
			Privileged:          priv,
			Overlays:            overlays,
			UseSPProxy:          useSPProxy,
			UseNetProxy:         useNetProxy,
			SecretsMap:          nil,
			SigmaPath:           []string{},
			RealmSwitchStr:      sp.NOT_SET,
		},
	}
}

func NewProcEnvUnset(priv, overlays bool) *ProcEnv {
	// Load Perf & Debug from the environment for convenience.
	return NewProcEnv(sp.NOT_SET, sp.Tpid(sp.NOT_SET), sp.Trealm(sp.NOT_SET), sp.NoPrincipal(), sp.NOT_SET, sp.NOT_SET, priv, overlays, false, false)
}

func NewProcEnvFromProto(p *ProcEnvProto) *ProcEnv {
	return &ProcEnv{p}
}

func NewBootProcEnv(principal *sp.Tprincipal, secrets map[string]*sp.SecretProto, etcdMnts map[string]*sp.TendpointProto, innerIP sp.Tip, outerIP sp.Tip, buildTag string, overlays bool) *ProcEnv {
	pe := NewProcEnvUnset(true, overlays)
	pe.SetPrincipal(principal)
	pe.SetSecrets(secrets)
	pe.Program = "kernel"
	pe.SetPID(sp.Tpid(principal.GetID().String()))
	pe.EtcdEndpoints = etcdMnts
	pe.InnerContainerIPStr = innerIP.String()
	pe.OuterContainerIPStr = outerIP.String()
	pe.BuildTag = buildTag
	pe.SetRealm(sp.ROOTREALM)
	pe.ProcDir = filepath.Join(sp.KPIDS, pe.GetPID().String())
	pe.Privileged = true
	pe.SetSigmaPath(buildTag)
	pe.HowInt = int32(BOOT)
	return pe
}

func NewTestProcEnv(realm sp.Trealm, secrets map[string]*sp.SecretProto, etcdMnts map[string]*sp.TendpointProto, innerIP sp.Tip, outerIP sp.Tip, buildTag string, overlays, useSPProxy bool, useNetProxy bool) *ProcEnv {
	pe := NewProcEnvUnset(true, overlays)
	pe.SetPrincipal(sp.NewPrincipal(sp.TprincipalID("test"), realm))
	pe.SetSecrets(secrets)
	pe.SetPID(sp.GenPid("test"))
	pe.SetRealm(realm)
	pe.EtcdEndpoints = etcdMnts
	pe.InnerContainerIPStr = innerIP.String()
	pe.OuterContainerIPStr = outerIP.String()
	pe.BuildTag = buildTag
	pe.Program = "test"
	pe.ProcDir = filepath.Join(sp.KPIDS, pe.GetPID().String())
	pe.HowInt = int32(TEST)
	pe.UseSPProxy = useSPProxy
	pe.SetSigmaPath(buildTag)
	pe.UseNetProxy = useNetProxy
	return pe
}

// Create a new sigma config which is a derivative of an existing sigma config.
func NewAddedProcEnv(pe *ProcEnv) *ProcEnv {
	pe2 := NewProcEnvUnset(pe.Privileged, false)
	*(pe2.ProcEnvProto) = *(pe.ProcEnvProto)
	pe2.SetPrincipal(sp.NewPrincipal(pe.GetPrincipal().GetID(), pe.GetRealm()))
	pe2.SecretsMap = make(map[string]*sp.SecretProto)
	// Deep copy secrets
	for k, v := range pe.GetSecrets() {
		pe2.SecretsMap[k] = &sp.SecretProto{
			ID:       v.ID,
			Key:      v.Key,
			Metadata: v.Metadata,
		}
	}
	return pe2
}

func NewDifferentRealmProcEnv(pe *ProcEnv, realm sp.Trealm) *ProcEnv {
	pe2 := NewProcEnvUnset(pe.Privileged, pe.Overlays)
	*(pe2.ProcEnvProto) = *(pe.ProcEnvProto)
	pe2.SetPrincipal(sp.NewPrincipal(
		sp.TprincipalID(pe.GetPrincipal().GetID().String()+"-realm-"+realm.String()),
		realm,
	))
	pe2.SecretsMap = make(map[string]*sp.SecretProto)
	pe2.SetRealm(realm)
	// Deep copy secrets
	for k, v := range pe.GetSecrets() {
		pe2.SecretsMap[k] = &sp.SecretProto{
			ID:       v.ID,
			Key:      v.Key,
			Metadata: v.Metadata,
		}
	}
	// Clear the named endpoint, so the new realm doesn't try to access the old
	// one's named
	pe2.ClearNamedEndpoint()
	return pe2
}

func (pe *ProcEnvProto) GetPID() sp.Tpid {
	return sp.Tpid(pe.PidStr)
}

func (pe *ProcEnvProto) SetSecrets(secrets map[string]*sp.SecretProto) {
	pe.SecretsMap = secrets
}

func (pe *ProcEnvProto) GetSecrets() map[string]*sp.SecretProto {
	return pe.SecretsMap
}

func (pe *ProcEnvProto) SetPID(pid sp.Tpid) {
	pe.PidStr = string(pid)
}

func (pe *ProcEnvProto) SetInnerContainerIP(ip sp.Tip) {
	pe.InnerContainerIPStr = ip.String()
}

func (pe *ProcEnvProto) SetSigmaPath(buildTag string) {
	if buildTag == sp.LOCAL_BUILD {
		pe.SigmaPath = append(pe.SigmaPath, filepath.Join(sp.UX, sp.LOCAL, "bin/user/common"))
	} else {
		pe.SigmaPath = append(pe.SigmaPath, filepath.Join(sp.S3, sp.LOCAL, buildTag, "bin"))
	}
}

func (pe *ProcEnvProto) PrependSigmaPath(pn string) {
	for _, p := range pe.SigmaPath {
		if p == pn {
			return
		}
	}
	pe.SigmaPath = append([]string{pn}, pe.SigmaPath...)
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

func (pe *ProcEnvProto) SetRealm(realm sp.Trealm) {
	pe.RealmStr = realm.String()
	pe.Principal.RealmStr = realm.String()
}

func (pe *ProcEnvProto) SetPrincipal(principal *sp.Tprincipal) {
	pe.Principal = principal
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

// Returns true if a realm switch was specified
func (pe *ProcEnvProto) GetRealmSwitch() (sp.Trealm, bool) {
	// Realm switch only takes place if a realm switch was specified, and if the
	// proc was originally part of the root realm.
	if pe.RealmSwitchStr == sp.NOT_SET || pe.GetRealm() != sp.ROOTREALM {
		return sp.NOT_SET, false
	}
	return sp.Trealm(pe.RealmSwitchStr), true
}

func (pe *ProcEnvProto) SetRealmSwitch(realm sp.Trealm) {
	pe.RealmSwitchStr = realm.String()
}

func (pe *ProcEnvProto) ClearNamedEndpoint() {
	pe.NamedEndpointProto = nil
}

func (pe *ProcEnvProto) SetNetFail(nf int64) {
	pe.NetFail = nf
}

func (pe *ProcEnvProto) SetVersion(v string) {
	pe.Version = v
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

func (pe *ProcEnv) GetScheddEndpoint() (*sp.Tendpoint, bool) {
	mp := pe.ProcEnvProto.GetScheddEndpointProto()
	if mp == nil {
		return &sp.Tendpoint{}, false
	}
	return sp.NewEndpointFromProto(mp), true
}

func (pe *ProcEnv) GetNamedEndpoint() (*sp.Tendpoint, bool) {
	mp := pe.ProcEnvProto.GetNamedEndpointProto()
	if mp == nil {
		return &sp.Tendpoint{}, false
	}
	return sp.NewEndpointFromProto(mp), true
}

func (pe *ProcEnv) SetNamedEndpoint(ep *sp.Tendpoint) {
	pe.ProcEnvProto.NamedEndpointProto = ep.TendpointProto
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
	return fmt.Sprintf("&{ "+
		"Program:%v "+
		"Version:%v "+
		"Pid:%v "+
		"Realm:%v "+
		"Principal:{%v} "+
		"KernelID:%v "+
		"UprocdPID:%v "+
		"ProcDir:%v "+
		"ParentDir:%v "+
		"How:%v "+
		"Perf:%v "+
		"Debug:%v "+
		"EtcdMnt:%v "+
		"InnerIP:%v "+
		"OuterIP:%v "+
		"Named:%v "+
		"BuildTag:%v "+
		"Privileged:%v "+
		"Overlays:%v "+
		"Crash:%v "+
		"Partition:%v "+
		"NetFail:%v "+
		"UseSPProxy:%v "+
		"UseNetProxy:%v "+
		"SigmaPath:%v "+
		"RealmSwitch:%v "+
		"Fail:%v"+
		"}",
		pe.Program,
		pe.Version,
		pe.GetPID(),
		pe.GetRealm(),
		pe.GetPrincipal().String(),
		pe.KernelID,
		pe.UprocdPIDStr,
		pe.ProcDir,
		pe.ParentDir,
		Thow(pe.HowInt),
		pe.Perf,
		pe.Debug,
		pe.GetEtcdEndpoints(),
		pe.InnerContainerIPStr,
		pe.OuterContainerIPStr,
		pe.NamedEndpointProto,
		pe.BuildTag,
		pe.Privileged,
		pe.Overlays,
		pe.Crash,
		pe.Partition,
		pe.NetFail,
		pe.UseSPProxy,
		pe.UseNetProxy,
		pe.SigmaPath,
		pe.RealmSwitchStr,
		pe.Fail,
	)
}
