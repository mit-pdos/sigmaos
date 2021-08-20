package fssrv

import (
	"ulambda/stats"
)

type FsServer struct {
	stats *stats.Stats
}

func MkFsServer() *FsServer {
	fs := &FsServer{}
	fs.stats = stats.MkStats()
	return fs

}
func (fs *FsServer) GetStats() *stats.Stats {
	return fs.stats
}
