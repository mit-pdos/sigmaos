package npproxy

import (
	"sync"
	"sync/atomic"

	db "sigmaos/debug"
	"sigmaos/path"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
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

func newProxyPath(k string) *proxyPath {
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
	pm *syncmap.SyncMap[string, *proxyPath]
}

func newPathMap(p sp.Tpath) *pathMap {
	return &pathMap{pm: syncmap.NewSyncMap[string, *proxyPath]()}
}

type qidMap struct {
	qm *syncmap.SyncMap[sp.Tpath, *pathMap]
}

func newQidMap() *qidMap {
	return &qidMap{qm: syncmap.NewSyncMap[sp.Tpath, *pathMap]()}
}

func (qm *qidMap) Insert(pn path.Tpathname, qids []*sp.Tqid) []*sp.TqidProto {
	db.DPrintf(db.NPPROXY, "Insert: pn %v qids %v\n", pn, qids)
	pqids := make([]*sp.Tqid, len(qids))
	for i, q := range qids {
		pm, _ := qm.qm.AllocNew(sp.Tpath(q.Path), newPathMap)
		pp, ok := pm.pm.AllocNew(pn.String(), newProxyPath)
		db.DPrintf(db.NPPROXY, "Insert: ok %t pmap %v %v %v\n", ok, pn, sp.Tpath(q.Path), pp)
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

func (qm *qidMap) Clunk(pn path.Tpathname, qid *sp.Tqid) {
	pm, ok := qm.qm.Lookup(sp.Tpath(qid.Path))
	if ok {
		pp, ok := pm.pm.Lookup(pn.String())
		if ok {
			if n := pp.Dec(); n <= 0 {
				db.DPrintf(db.NPPROXY, "Clunk: pn %v qid %v del %v\n", pn, qid, pp)
				pm.pm.Delete(pn.String())
			}
		}
	}
	if !ok {
		db.DPrintf(db.ERROR, "Clunk: pn %v qid %v not present\n", pn, qid)
	}
}
