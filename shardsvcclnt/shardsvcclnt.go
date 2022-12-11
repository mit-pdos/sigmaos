package shardsvcclnt

import (
	"log"
	"sync"

	"google.golang.org/protobuf/proto"

	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	"sigmaos/reader"
	"sigmaos/shardsvcmgr"
)

type ShardWatch func(string, int, error)

type ShardSvcClnt struct {
	sync.Mutex
	*fslib.FsLib
	clnts []*protdevclnt.ProtDevClnt
	pn    string
	sw    ShardWatch
	rdr   *reader.Reader
}

func MkShardSvcClnt(fsl *fslib.FsLib, pn string, n int, sw ShardWatch) (*ShardSvcClnt, error) {
	ssc := &ShardSvcClnt{FsLib: fsl, pn: pn, sw: sw}
	ssc.clnts = make([]*protdevclnt.ProtDevClnt, 0)
	for i := 0; i < n; i++ {
		if err := ssc.addClnt(i); err != nil {
			return nil, err
		}
	}
	if err := ssc.setWatch(); err != nil {
		return nil, err
	}
	return ssc, nil
}

func (ssc *ShardSvcClnt) setWatch() error {
	dir := ssc.pn + shardsvcmgr.SHRDDIR
	_, rdr, err := ssc.ReadDir(dir)
	if err != nil {
		return err
	}
	ssc.rdr = rdr
	if err := ssc.SetDirWatch(ssc.rdr.Fid(), dir, ssc.Watch); err != nil {
		return err
	}
	return nil
}

func (ssc *ShardSvcClnt) addClnt(i int) error {
	ssc.Lock()
	defer ssc.Unlock()

	sn := ssc.pn + shardsvcmgr.Shard(i)
	pdc, err := protdevclnt.MkProtDevClnt(ssc.FsLib, sn)
	if err != nil {
		return err
	}
	ssc.clnts = append(ssc.clnts, pdc)
	return nil
}

func (ssc *ShardSvcClnt) Watch(path string, err error) {
	log.Printf("%v: shardsvcclnt watch %v err %v\n", proc.GetName(), path, err)
	if err != nil {
		log.Printf("Watch err %v\n", err)
		return
	}
	sts, err := ssc.GetDir(path)
	if len(sts) > len(ssc.clnts) {
		if err := ssc.addClnt(len(sts) - 1); err != nil {
			log.Printf("%v: addClnt err %v\n", proc.GetName(), err)
		}
		ssc.sw(path, len(sts), err)
	}
	ssc.rdr.Close()
	if err := ssc.setWatch(); err != nil {
		log.Printf("setWatch err %v\n", err)
	}
}

func (ssc *ShardSvcClnt) Server(i int) string {
	return ssc.pn + shardsvcmgr.Shard(i)
}

func (ssc *ShardSvcClnt) Nshard() int {
	return len(ssc.clnts)
}

func (ssc *ShardSvcClnt) RPC(i int, m string, arg proto.Message, res proto.Message) error {
	return ssc.clnts[i].RPC(m, arg, res)
}

func (ssc *ShardSvcClnt) StatsSrv(i int) (*protdevsrv.Stats, error) {
	return ssc.clnts[i].StatsSrv()
}

func (ssc *ShardSvcClnt) StatsClnt(i int) *protdevsrv.Stats {
	return ssc.clnts[i].StatsClnt()
}
