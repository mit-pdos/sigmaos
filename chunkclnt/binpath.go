package chunkclnt

import (
	"sync"

	"golang.org/x/exp/slices" // todo: upgrade to > 1.21

	"sigmaos/rand"
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

func (bl *BinPaths) GetBinKernelID(bin string) (string, bool) {
	bl.Lock()
	defer bl.Unlock()

	if kids, ok := bl.bins[bin]; ok {
		i := rand.Int64(int64(len(kids)))
		return kids[int(i)], true
	}
	return "", false
}

func (bl *BinPaths) SetBinKernelID(bin, kernelId string) {
	bl.Lock()
	defer bl.Unlock()

	if _, ok := bl.bins[bin]; ok {
		i := slices.IndexFunc(bl.bins[bin], func(s string) bool { return s == kernelId })
		if i == -1 {
			bl.bins[bin] = append(bl.bins[bin], kernelId)
		}
	} else {
		bl.bins[bin] = []string{kernelId}
	}
}

func (bl *BinPaths) DelBinKernelID(bin, kernelId string) {
	bl.Lock()
	defer bl.Unlock()

	if _, ok := bl.bins[bin]; ok {
		i := slices.IndexFunc(bl.bins[bin], func(s string) bool { return s == kernelId })
		if i != -1 {
			bl.bins[bin] = append(bl.bins[bin][:i], bl.bins[bin][i+1:]...)
		}
	}
}
