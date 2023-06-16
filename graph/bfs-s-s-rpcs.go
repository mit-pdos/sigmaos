package graph

import (
	"encoding/json"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/graph/proto"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
)

type BfsSingle struct {
	Thread
	g       *Graph
	parents []int
	CS      chan int
	pdc     *protdevclnt.ProtDevClnt
}

func StartThread(public bool, jobname string, graph string) error {
	var err error
	b := BfsSingle{}
	if b.Thread, err = initThread(jobname); err != nil {
		return err
	}
	b.g = &Graph{}
	if err = json.Unmarshal([]byte(graph), b.g); err != nil {
		return err
	}
	pds, err := protdevsrv.MakeProtDevSrvPublic(b.serverPath, b, public)
	if err != nil {
		db.DPrintf(DEBUG_GRAPH, "|%v| Failed to make ProtDevSrv: %v", b.job, err)
		return err
	}
	if b.pdc, err = protdevclnt.MkProtDevClnt([]*fslib.FsLib{b.FsLib}, b.serverPath); err != nil {
		return err
	}
	// XXX Replace with variable length queue
	b.CS = make(chan int, b.g.NumEdges)
	return pds.RunServer()
}

func (b *BfsSingle) BfsSingle(ctx fs.CtxI, req proto.BfsIn, res *proto.BfsPath) error {
	n1 := int(req.N1)
	n2 := int(req.N2)
	b.parents = make([]int, b.g.NumNodes)
	for i := range b.parents {
		b.parents[i] = NOT_VISITED
	}
	putReq := proto.Index{Val: req.N1}
	putRes := proto.None{}
	b.parents[n1] = n1
	if err := b.pdc.RPC("BfsSingle.Put", &putReq, &putRes); err != nil {
		return err
	}

	// XXX End Condition for No Path
	for {
		index := <-b.CS
		adj := b.g.GetNeighbors(index)
		for _, a := range *adj {
			if b.parents[a] == NOT_VISITED {
				putReq := proto.Index{Val: int64(a)}
				if err := b.pdc.RPC("BfsSingle.Put", &putReq, &putRes); err != nil {
					return err
				}

				b.parents[a] = index
				if a == n2 {
					// Return the shortest path from n1 to n2
					res.Val = *findPath64(&b.parents, n1, n2)
					return nil
				}
			}
		}
	}
	return ERR_NOPATH
}

func (b *BfsSingle) Put(ctx fs.CtxI, req proto.Index, res *proto.None) error {
	b.CS <- int(req.Val)
	return nil
}
