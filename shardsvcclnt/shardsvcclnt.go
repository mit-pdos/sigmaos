package shardsvcclnt

import (
	"google.golang.org/protobuf/proto"

	"sigmaos/fslib"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	"sigmaos/shardsvcmgr"
)

type ShardSvcClnt struct {
	*fslib.FsLib
	clnts []*protdevclnt.ProtDevClnt
	pn    string
}

func MkShardSvcClnt(fsl *fslib.FsLib, pn string, n int) (*ShardSvcClnt, error) {
	ssc := &ShardSvcClnt{FsLib: fsl, pn: pn}
	ssc.clnts = make([]*protdevclnt.ProtDevClnt, 0)
	for s := 0; s < n; s++ {
		sn := pn + shardsvcmgr.Shard(s)
		pdc, err := protdevclnt.MkProtDevClnt(fsl, sn)
		if err != nil {
			return nil, err
		}
		ssc.clnts = append(ssc.clnts, pdc)
	}
	return ssc, nil
}

func (ssc *ShardSvcClnt) Server(i int) string {
	return ssc.pn + shardsvcmgr.Shard(i)
}

func (ssc *ShardSvcClnt) Nshard() int {
	return len(ssc.clnts)
}

func (ssc *ShardSvcClnt) RPC(g int, m string, arg proto.Message, res proto.Message) error {
	return ssc.clnts[g].RPC(m, arg, res)
}

func (ssc *ShardSvcClnt) StatsSrv(g int) (*protdevsrv.Stats, error) {
	return ssc.clnts[g].StatsSrv()
}

func (ssc *ShardSvcClnt) StatsClnt(g int) *protdevsrv.Stats {
	return ssc.clnts[g].StatsClnt()
}
