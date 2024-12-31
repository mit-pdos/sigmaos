package npproxysrv

import (
	"sync"
	"sync/atomic"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/syncmap"
)

//
// Map qid's path to a new unique path for the kernel
//

var nextPath atomic.Uint64

type proxyPath struct {
	mu   sync.Mutex
	path sp.Tpath
	nfid int
}

func newProxyPath(k sp.Tfid) *proxyPath {
	return &proxyPath{
		nfid: 1,
		path: sp.Tpath(nextPath.Add(1)),
	}
}

func (pp *proxyPath) Inc() {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.nfid += 1
}

func (pp *proxyPath) Dec() int {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.nfid -= 1
	return pp.nfid
}

type pathMap struct {
	pm *syncmap.SyncMap[sp.Tfid, *proxyPath]
}

func newPathMap(p sp.Tpath) *pathMap {
	return &pathMap{pm: syncmap.NewSyncMap[sp.Tfid, *proxyPath]()}
}

type qidMap struct {
	qm *syncmap.SyncMap[sp.Tpath, *pathMap]
}

func newQidMap() *qidMap {
	return &qidMap{qm: syncmap.NewSyncMap[sp.Tpath, *pathMap]()}
}

func (qm *qidMap) Insert(fid sp.Tfid, qids []sp.Tqid) []*sp.TqidProto {
	db.DPrintf(db.NPPROXY, "Insert: %v qids %v\n", fid, qids)
	pqids := make([]sp.Tqid, len(qids))
	for i, q := range qids {
		pm, _ := qm.qm.AllocNew(sp.Tpath(q.Path), newPathMap)
		pp, ok := pm.pm.AllocNew(fid, newProxyPath)
		db.DPrintf(db.NPPROXY, "Insert: ok %t pmap %v %v %v\n", ok, fid, sp.Tpath(q.Path), pp)
		if pm.pm.Len() > 1 {
			db.DPrintf(db.NPPROXY, "Insert: collision %v", pm.pm)
		}
		if !ok {
			pp.Inc()
		}
		pqids[i] = sp.NewQid(q.Ttype(), q.Tversion(), pp.path)
	}
	return sp.NewSliceProto(pqids)
}

func (qm *qidMap) Clunk(fid sp.Tfid, qid *sp.Tqid) {
	pm, ok := qm.qm.Lookup(sp.Tpath(qid.Path))
	if ok {
		pp, ok := pm.pm.Lookup(fid)
		if ok {
			if n := pp.Dec(); n <= 0 {
				db.DPrintf(db.NPPROXY, "Clunk: %v qid %v del %v\n", fid, qid, pp)
				pm.pm.Delete(fid)
			}
		}
	}
	if !ok {
		db.DPrintf(db.ERROR, "Clunk: %v qid %v not present\n", fid, qid)
	}
}
