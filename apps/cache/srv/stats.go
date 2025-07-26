package srv

import (
	"fmt"

	"sigmaos/apps/cache"
)

type shardStats struct {
	shardID cache.Tshard
	hitCnt  uint64
}

func (s shardStats) String() string {
	return fmt.Sprintf("&{ sid:%v hitCnt:%v }", s.shardID, s.hitCnt)
}
