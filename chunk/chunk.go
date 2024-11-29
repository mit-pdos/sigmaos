package chunk

import (
	"path/filepath"
	"strings"

	sp "sigmaos/sigmap"
)

func Index(o int64) int    { return int(o / sp.Conf.Chunk.CHUNK_SZ) }
func ChunkOff(i int) int64 { return int64(int64(i) * sp.Conf.Chunk.CHUNK_SZ) }
func ChunkRound(o int64) int64 {
	return (o + sp.Conf.Chunk.CHUNK_SZ - 1) &^ (sp.Conf.Chunk.CHUNK_SZ - 1)
}

func ChunkdPath(kernelId string) string {
	return filepath.Join(sp.CHUNKD, kernelId)
}

func IsChunkSrvPath(path string) bool {
	return strings.Contains(path, sp.CHUNKD)
}
