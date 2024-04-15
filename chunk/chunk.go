package chunk

import (
	"path/filepath"

	sp "sigmaos/sigmap"
)

func ChunkdPath(kernelId string) string {
	return filepath.Join(sp.CHUNKD, kernelId)
}
