package chunkclnt

import (
	"sync"

	"golang.org/x/exp/slices" // todo: upgrade to > 1.21

	db "sigmaos/debug"
	"sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

// BinPaths keeps track of kernels that have ran a binary, and are
// likely to have to the binary cached.
type BinPaths struct {
	sync.Mutex
	bins map[string][]string // map from bin name to kernelId
}

func NewBinPaths() *BinPaths {
	return &BinPaths{bins: make(map[string][]string)}
}

func (bp *BinPaths) GetBinKernelID(bin string) (string, bool) {
	bp.Lock()
	defer bp.Unlock()

	if kids, ok := bp.bins[bin]; ok {
		i := rand.Int64(int64(len(kids)))
		k := kids[int(i)]
		db.DPrintf(db.CHUNKCLNT, "GetBinKernelID %v %v\n", bin, k)
		return k, true
	}
	return "", false
}

func (bp *BinPaths) SetBinKernelID(bin, kernelId string) {
	bp.Lock()
	defer bp.Unlock()

	if _, ok := bp.bins[bin]; ok {
		i := slices.IndexFunc(bp.bins[bin], func(s string) bool { return s == kernelId })
		if i == -1 {
			bp.bins[bin] = append(bp.bins[bin], kernelId)
		}
	} else {
		bp.bins[bin] = []string{kernelId}
	}
}

func (bp *BinPaths) DelBinKernelID(bin, kernelId string) {
	bp.Lock()
	defer bp.Unlock()

	if _, ok := bp.bins[bin]; ok {
		i := slices.IndexFunc(bp.bins[bin], func(s string) bool { return s == kernelId })
		if i != -1 {
			bp.bins[bin] = append(bp.bins[bin][:i], bp.bins[bin][i+1:]...)
		}
	}
}

type RealmBinPaths struct {
	realmbins *syncmap.SyncMap[sp.Trealm, *BinPaths]
}

func NewRealmBinPaths() *RealmBinPaths {
	return &RealmBinPaths{realmbins: syncmap.NewSyncMap[sp.Trealm, *BinPaths]()}
}

func (rbp *RealmBinPaths) GetBinKernelID(r sp.Trealm, bin string) (string, bool) {
	bp, ok := rbp.realmbins.Lookup(r)
	if !ok {
		bp, _ = rbp.realmbins.Alloc(r, NewBinPaths())
	}
	return bp.GetBinKernelID(bin)
}

func (rbp *RealmBinPaths) SetBinKernelID(r sp.Trealm, bin, kernelId string) {
	bp, ok := rbp.realmbins.Lookup(r)
	if !ok {
		bp, _ = rbp.realmbins.Alloc(r, NewBinPaths())
	}
	bp.SetBinKernelID(bin, kernelId)
}

func (rbp *RealmBinPaths) DelBinKernelID(r sp.Trealm, bin, kernelId string) {
	bp, ok := rbp.realmbins.Lookup(r)
	if !ok {
		return
	}
	bp.DelBinKernelID(bin, kernelId)
}
