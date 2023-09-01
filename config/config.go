package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"runtime/debug"
	"strconv"

	sp "sigmaos/sigmap"
)

const (
	SIGMACONFIG = "SIGMACONFIG"
	SIGMADEBUG  = "SIGMADEBUG"
	SIGMAPERF   = "SIGMAPERF"
	NOT_SET     = "NOT_SET" // Catch cases where we fail to set a variable.
)

// TODO: make into proto

type SigmaConfig struct {
	PID        sp.Tpid   `json:pid,omitempty`
	Realm      sp.Trealm `json:realm,omitempty`
	Uname      sp.Tuname `json:uname,omitempty`
	KernelID   string    `json:kernelid,omitempty`
	UprocdPID  sp.Tpid   `json:uprocdpid,omitempty`
	Net        string    `json:net,omitempty`
	Privileged bool      `json:privileged,omitempty` // XXX phase out?
	Program    string    `json:program,omitempty`
	ProcDir    string    `json:procdir,omitempty`   // XXX phase out?
	ParentDir  string    `json:parentdir,omitempty` // XXX phase out?
	Perf       string    `json:perf,omitempty`
	Debug      string    `json:debug,omitempty`
	EtcdIP     string    `json:etcdip,omitempty`
	LocalIP    string    `json:localip,omitempty`
	BuildTag   string    `json:buildtag,omitempty`
	// For testing purposes
	Crash     bool `json:crash,omitempty`
	Partition bool `json:partition,omitempty`
}

// XXX will serve as a guide for necessary constructors
//func MakeFsLibAddrNet(uname sp.Tuname, realm sp.Trealm, lip string, addrs sp.Taddrs, clntnet string) (*FsLib, error) {
//func MakeFsLibAddr(uname sp.Tuname, realm sp.Trealm, lip string, addrs sp.Taddrs) (*FsLib, error) {
//func MakeFsLib(uname sp.Tuname) (*FsLib, error) {
//func MkSigmaClntRealmFsLib(rootrealm *fslib.FsLib, uname sp.Tuname, rid sp.Trealm) (*SigmaClnt, error) {
//func MkSigmaClntRootInit(uname sp.Tuname, ip string, namedAddr sp.Taddrs) (*SigmaClnt, error) {

func NewSigmaConfig() *SigmaConfig {
	// Load Perf & Debug from the environment for convenience.
	return &SigmaConfig{
		PID:       NOT_SET,
		ProcDir:   NOT_SET,
		ParentDir: NOT_SET,
		Program:   NOT_SET,
		Perf:      os.Getenv(SIGMAPERF),
		Debug:     os.Getenv(SIGMADEBUG),
	}
}

func NewBootSigmaConfig(uname sp.Tuname, etcdIP, localIP string) *SigmaConfig {
	sc := NewSigmaConfig()
	sc.Uname = uname
	sc.Program = "kernel"
	sc.PID = sp.GenPid(string(uname))
	sc.EtcdIP = etcdIP
	sc.LocalIP = localIP
	sc.Realm = sp.ROOTREALM
	sc.ProcDir = path.Join(sp.KPIDS, sc.PID.String())
	return sc
}

func NewTestSigmaConfig(realm sp.Trealm, etcdIP, localIP, buildTag string) *SigmaConfig {
	sc := NewSigmaConfig()
	sc.Uname = "test"
	sc.PID = sp.GenPid("test")
	sc.Realm = realm
	sc.EtcdIP = etcdIP
	sc.LocalIP = localIP
	sc.BuildTag = buildTag
	sc.Program = "test"
	sc.ProcDir = path.Join(sp.KPIDS, sc.PID.String())
	return sc
}

// Create a new sigma config which is a derivative of an existing sigma config.
func NewAddedSigmaConfig(sc *SigmaConfig, idx int) *SigmaConfig {
	sc2 := NewSigmaConfig()
	*sc2 = *sc
	sc2.Uname = sp.Tuname(string(sc2.Uname) + "-clnt-" + strconv.Itoa(idx))
	return sc2
}

func NewDifferentRealmSigmaConfig(sc *SigmaConfig, realm sp.Trealm) *SigmaConfig {
	sc2 := NewSigmaConfig()
	*sc2 = *sc
	sc2.Realm = realm
	sc2.Uname = sp.Tuname(string(sc2.Uname) + "-realm-" + realm.String())
	return sc2
}

func (sc *SigmaConfig) Marshal() string {
	b, err := json.Marshal(sc)
	if err != nil {
		log.Fatalf("Error marshal sigmaconfig: %v")
	}
	return string(b)
}

func Unmarshal(scstr string) *SigmaConfig {
	sc := &SigmaConfig{}
	err := json.Unmarshal([]byte(scstr), sc)
	if err != nil {
		log.Fatalf("Error unmarshal SigmaConfig %v", err)
	}
	return sc
}

// XXX When should I not get the config?
func GetSigmaConfig() *SigmaConfig {
	scstr := os.Getenv(SIGMACONFIG)
	if scstr == "" {
		stack := debug.Stack()
		log.Fatalf("%s\nError: No Sigma Config", stack)
	}
	return Unmarshal(scstr)
}

func (sc *SigmaConfig) String() string {
	return fmt.Sprintf("&{ Pid:%v Realm:%v Uname:%v KernelID:%v UprocdPID:%v Net:%v Privileged:%v Program:%v ProcDir:%v ParentDir:%v Perf:%v Debug:%v EtcdIP:%v LocalIP:%v BuildTag:%v Crash:%v Partition:%v }", sc.PID, sc.Realm, sc.Uname, sc.KernelID, sc.UprocdPID, sc.Net, sc.Privileged, sc.Program, sc.ProcDir, sc.ParentDir, sc.Perf, sc.Debug, sc.EtcdIP, sc.LocalIP, sc.BuildTag, sc.Crash, sc.Partition)
}
