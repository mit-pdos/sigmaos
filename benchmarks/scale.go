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

type AutoscalingConfig struct {
	Scale            bool          `json:"scale"`
	InitialNReplicas int           `json:"initial_n_replicas"`
	TargetRIF        float64       `json:"target_rif"`
	Frequency        time.Duration `json:"frequency"`
	Tolerance        float64       `json:"tolerance"`
}

func NewAutoscalingConfig(scale bool, initialNReplicas int, targetRIF float64, frequency time.Duration, tolerance float64) *AutoscalingConfig {
	return &AutoscalingConfig{
		Scale:            scale,
		InitialNReplicas: initialNReplicas,
		TargetRIF:        targetRIF,
		Frequency:        frequency,
		Tolerance:        tolerance,
	}
}

func (cfg *AutoscalingConfig) String() string {
	return fmt.Sprintf("&{ scale:%v targetRIF:%v frequency:%v initialNReplicas:%v tolerance:%v }", cfg.Scale, cfg.TargetRIF, cfg.Frequency, cfg.InitialNReplicas, cfg.Tolerance)
}
