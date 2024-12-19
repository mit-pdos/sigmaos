package clnt

import (
	"sync"

	"golang.org/x/exp/slices" // todo: upgrade to > 1.21

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/rand"
	"sigmaos/util/syncmap"
)

type kernelIDs struct {
	idMap   map[string]bool
	idSlice []string
}

func newKernelIDs(kernelID string) *kernelIDs {
	return &kernelIDs{
		idMap:   map[string]bool{kernelID: true},
		idSlice: []string{kernelID},
	}
}

// BinPaths keeps track of kernels that have ran a binary, and are
// likely to have to the binary cached.
type BinPaths struct {
	sync.RWMutex
	bins map[string]*kernelIDs
}

func NewBinPaths() *BinPaths {
	return &BinPaths{
		bins: make(map[string]*kernelIDs),
	}
}

func (bp *BinPaths) GetBinKernelID(bin string) (string, bool) {
	bp.RLock()
	defer bp.RUnlock()

	if kids, ok := bp.bins[bin]; ok {
		i := rand.Int64(int64(len(kids.idSlice)))
		k := kids.idSlice[int(i)]
		db.DPrintf(db.CHUNKCLNT, "GetBinKernelID %v %v\n", bin, k)
		return k, true
	}
	return "", false
}

func (bp *BinPaths) SetBinKernelID(bin, kernelID string) {
	bp.RLock()
	defer bp.RUnlock()

	if _, ok := bp.bins[bin]; !ok {
		bp.RUnlock()
		bp.Lock()
		if _, ok := bp.bins[bin]; !ok {
			bp.bins[bin] = newKernelIDs(kernelID)
		}
		bp.Unlock()
		bp.RLock()
	}
	if _, ok := bp.bins[bin].idMap[kernelID]; !ok {
		bp.RUnlock()
		bp.Lock()
		if _, ok := bp.bins[bin].idMap[kernelID]; !ok {
			bp.bins[bin].idSlice = append(bp.bins[bin].idSlice, kernelID)
		}
		bp.Unlock()
		bp.RLock()
	}
}

func (bp *BinPaths) DelBinKernelID(bin, kernelID string) {
	bp.Lock()
	defer bp.Unlock()

	if _, ok := bp.bins[bin]; ok {
		if _, ok := bp.bins[bin].idMap[kernelID]; ok {
			i := slices.IndexFunc(bp.bins[bin].idSlice, func(s string) bool { return s == kernelID })
			copy(bp.bins[bin].idSlice[:i], bp.bins[bin].idSlice[i+1:])
			bp.bins[bin].idSlice = bp.bins[bin].idSlice[:len(bp.bins[bin].idSlice)-1]
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
