package srv

import (
	"sigmaos/apps/cache"
)

type shardStats struct {
	shardID cache.Tshard
	hitCnt  uint64
}
