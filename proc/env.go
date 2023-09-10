package proc

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
	NOT_SET     = "NOT_SET" // Catch cases where we fail to set a variable.
)

type ProcEnv struct {
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

func NewProcEnv() *ProcEnv {
	// Load Perf & Debug from the environment for convenience.
	return &ProcEnv{
		PID:       NOT_SET,
		ProcDir:   NOT_SET,
		ParentDir: NOT_SET,
		Program:   NOT_SET,
		Perf:      os.Getenv(SIGMAPERF),
		Debug:     os.Getenv(SIGMADEBUG),
	}
}

func NewChildProcEnv(pcfg *ProcEnv, p *Proc) *ProcEnv {
	sc2 := NewProcEnv()
	*sc2 = *pcfg
	sc2.PID = p.GetPid()
	sc2.Uname = sp.Tuname(p.GetPid())
	sc2.Program = p.Program
	// XXX Mount parentDir?
	sc2.ParentDir = path.Join(pcfg.ProcDir, CHILDREN, p.GetPid().String())
	// TODO: anything else?
	return sc2
}

func NewBootProcEnv(uname sp.Tuname, etcdIP, localIP string) *ProcEnv {
	sc := NewProcEnv()
	sc.Uname = uname
	sc.Program = "kernel"
	sc.PID = sp.GenPid(string(uname))
	sc.EtcdIP = etcdIP
	sc.LocalIP = localIP
	sc.Realm = sp.ROOTREALM
	sc.ProcDir = path.Join(sp.KPIDS, sc.PID.String())
	return sc
}

func NewTestProcEnv(realm sp.Trealm, etcdIP, localIP, buildTag string) *ProcEnv {
	sc := NewProcEnv()
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
func NewAddedProcEnv(sc *ProcEnv, idx int) *ProcEnv {
	sc2 := NewProcEnv()
	*sc2 = *sc
	sc2.Uname = sp.Tuname(string(sc2.Uname) + "-clnt-" + strconv.Itoa(idx))
	return sc2
}

func NewDifferentRealmProcEnv(sc *ProcEnv, realm sp.Trealm) *ProcEnv {
	sc2 := NewProcEnv()
	*sc2 = *sc
	sc2.Realm = realm
	sc2.Uname = sp.Tuname(string(sc2.Uname) + "-realm-" + realm.String())
	return sc2
}

func (sc *ProcEnv) Marshal() string {
	b, err := json.Marshal(sc)
	if err != nil {
		log.Fatalf("Error marshal sigmaconfig: %v")
	}
	return string(b)
}

func Unmarshal(scstr string) *ProcEnv {
	sc := &ProcEnv{}
	err := json.Unmarshal([]byte(scstr), sc)
	if err != nil {
		log.Fatalf("Error unmarshal ProcEnv %v", err)
	}
	return sc
}

// XXX When should I not get the config?
func GetProcEnv() *ProcEnv {
	scstr := os.Getenv(SIGMACONFIG)
	if scstr == "" {
		stack := debug.Stack()
		log.Fatalf("%s\nError: No Sigma Config", stack)
	}
	return Unmarshal(scstr)
}

func (sc *ProcEnv) String() string {
	return fmt.Sprintf("&{ Pid:%v Realm:%v Uname:%v KernelID:%v UprocdPID:%v Net:%v Privileged:%v Program:%v ProcDir:%v ParentDir:%v Perf:%v Debug:%v EtcdIP:%v LocalIP:%v BuildTag:%v Crash:%v Partition:%v }", sc.PID, sc.Realm, sc.Uname, sc.KernelID, sc.UprocdPID, sc.Net, sc.Privileged, sc.Program, sc.ProcDir, sc.ParentDir, sc.Perf, sc.Debug, sc.EtcdIP, sc.LocalIP, sc.BuildTag, sc.Crash, sc.Partition)
}
