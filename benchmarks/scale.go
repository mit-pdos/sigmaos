package benchmarks

import (
	"fmt"
	"time"
)

type ManualScalingConfig struct {
	Svc        string        `json:"svc"`
	Scale      bool          `json:"scale"`
	ScaleDelay time.Duration `json:"scale_delay"`
	NToAdd     int           `json:"n_to_add"`
}

func NewManualScalingConfig(svc string, scale bool, scaleDelay time.Duration, nToAdd int) *ManualScalingConfig {
	return &ManualScalingConfig{
		Svc:        svc,
		Scale:      scale,
		ScaleDelay: scaleDelay,
		NToAdd:     nToAdd,
	}
}

func (cfg *ManualScalingConfig) GetShouldScale() bool {
	return cfg.Scale
}

func (cfg *ManualScalingConfig) GetScalingDelay() time.Duration {
	return cfg.ScaleDelay
}

func (cfg *ManualScalingConfig) GetNToAdd() int {
	return cfg.NToAdd
}

func (cfg *ManualScalingConfig) String() string {
	return fmt.Sprintf("&{ svc:%v scale:%v delay:%v nToAdd:%v }", cfg.Svc, cfg.Scale, cfg.ScaleDelay, cfg.NToAdd)
}
