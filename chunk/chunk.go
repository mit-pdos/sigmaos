package chunk

import (
	"path/filepath"

	sp "sigmaos/sigmap"
)

const (
	CHUNKSZ = 1 * sp.MBYTE
)

func ChunkdPath(kernelId string) string {
	return filepath.Join(sp.CHUNKD, kernelId)
}
