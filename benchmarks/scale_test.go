package benchmarks_test

import (
	"fmt"
	"time"
)

type ManualScalingConfig struct {
	svc        string
	scale      bool
	scaleDelay time.Duration
	nToAdd     int
}

func NewManualScalingConfig(svc string, scale bool, scaleDelay time.Duration, nToAdd int) *ManualScalingConfig {
	return &ManualScalingConfig{
		svc:        svc,
		scale:      scale,
		scaleDelay: scaleDelay,
		nToAdd:     nToAdd,
	}
}

func (cfg *ManualScalingConfig) GetShouldScale() bool {
	return cfg.scale
}

func (cfg *ManualScalingConfig) GetScalingDelay() time.Duration {
	return cfg.scaleDelay
}

func (cfg *ManualScalingConfig) GetNToAdd() int {
	return cfg.nToAdd
}

func (cfg *ManualScalingConfig) String() string {
	return fmt.Sprintf("&{ svc:%v scale:%v delay:%v nToAdd:%v }", cfg.svc, cfg.scale, cfg.scaleDelay, cfg.nToAdd)
}
