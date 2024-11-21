package chunk

import (
	"path/filepath"
	"strings"

	sp "sigmaos/sigmap"
)

const (
	CHUNKSZ = 1 * sp.MBYTE
)

func Index(o int64) int        { return int(o / CHUNKSZ) }
func ChunkOff(i int) int64     { return int64(i * CHUNKSZ) }
func ChunkRound(o int64) int64 { return (o + CHUNKSZ - 1) &^ (CHUNKSZ - 1) }

func ChunkdPath(kernelId string) string {
	return filepath.Join(sp.CHUNKD, kernelId)
}

func IsChunkSrvPath(path string) bool {
	return strings.Contains(path, sp.CHUNKD)
}
