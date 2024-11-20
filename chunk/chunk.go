package chunk

import (
	"path/filepath"
	"strings"

	sp "sigmaos/sigmap"
)

const (
	CHUNKSZ = 1 * sp.MBYTE
)

func Index(o int64) int { return int(o / CHUNKSZ) }

func ChunkdPath(kernelId string) string {
	return filepath.Join(sp.CHUNKD, kernelId)
}

func IsChunkSrvPath(path string) bool {
	return strings.Contains(path, sp.CHUNKD)
}
