package protdevclntgrp

import (
	"strconv"

	"google.golang.org/protobuf/proto"

	"sigmaos/fslib"
	"sigmaos/group"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
)

type ClntGroup struct {
	*fslib.FsLib
	clnts []*protdevclnt.ProtDevClnt
}

func MkProtDevClntGrp(fsl *fslib.FsLib, fn string, n int) (*ClntGroup, error) {
	clntgrp := &ClntGroup{}
	clntgrp.clnts = make([]*protdevclnt.ProtDevClnt, 0)
	clntgrp.FsLib = fsl
	for g := 0; g < n; g++ {
		gn := group.GRP + strconv.Itoa(g)
		pdc, err := protdevclnt.MkProtDevClnt(fsl, fn+gn)
		if err != nil {
			return nil, err
		}
		clntgrp.clnts = append(clntgrp.clnts, pdc)
	}
	return clntgrp, nil
}

func (gc *ClntGroup) Nshard() int {
	return len(gc.clnts)
}

func (gc *ClntGroup) RPC(g int, m string, arg proto.Message, res proto.Message) error {
	return gc.clnts[g].RPC(m, arg, res)
}

func (gc *ClntGroup) StatsSrv(g int) (*protdevsrv.Stats, error) {
	return gc.clnts[g].StatsSrv()
}

func (gc *ClntGroup) StatsClnt(g int) *protdevsrv.Stats {
	return gc.clnts[g].StatsClnt()
}
