package benchmarks

import (
	"fmt"
	"time"
)

type ManualScalingConfig struct {
	Svc         string          `json:"svc"`
	Scale       bool            `json:"scale"`
	ScaleDelays []time.Duration `json:"scale_delays"`
	ScaleDeltas []int           `json:"scale_deltas"`
}

func NewManualScalingConfig(svc string, scale bool, scaleDelays []time.Duration, scaleDeltas []int) *ManualScalingConfig {
	return &ManualScalingConfig{
		Svc:         svc,
		Scale:       scale,
		ScaleDelays: scaleDelays,
		ScaleDeltas: scaleDeltas,
	}
}

func (cfg *ManualScalingConfig) GetShouldScale() bool {
	return cfg.Scale
}

func (cfg *ManualScalingConfig) GetScalingDelays() []time.Duration {
	return cfg.ScaleDelays
}

func (cfg *ManualScalingConfig) GetScalingDeltas() []int {
	return cfg.ScaleDeltas
}

func (cfg *ManualScalingConfig) String() string {
	return fmt.Sprintf("&{ svc:%v scale:%v delays:%v deltas:%v }", cfg.Svc, cfg.Scale, cfg.ScaleDelays, cfg.ScaleDeltas)
}

type AutoscalingConfig struct {
	Svc              string        `json:"svc"`
	Scale            bool          `json:"scale"`
	InitialNReplicas int           `json:"initial_n_replicas"`
	MaxReplicas      int           `json:"max_replicas"`
	TargetRIF        float64       `json:"target_rif"`
	Frequency        time.Duration `json:"frequency"`
	Tolerance        float64       `json:"tolerance"`
}

func NewAutoscalingConfig(svc string, scale bool, initialNReplicas int, maxReplicas int, targetRIF float64, frequency time.Duration, tolerance float64) *AutoscalingConfig {
	return &AutoscalingConfig{
		Svc:              svc,
		Scale:            scale,
		InitialNReplicas: initialNReplicas,
		MaxReplicas:      maxReplicas,
		TargetRIF:        targetRIF,
		Frequency:        frequency,
		Tolerance:        tolerance,
	}
}

func (cfg *AutoscalingConfig) GetShouldScale() bool {
	return cfg.Scale
}

func (cfg *AutoscalingConfig) String() string {
	return fmt.Sprintf("&{ svc:%v scale:%v targetRIF:%v frequency:%v initialNReplicas:%v maxReplicas:%v tolerance:%v }", cfg.Svc, cfg.Scale, cfg.TargetRIF, cfg.Frequency, cfg.InitialNReplicas, cfg.MaxReplicas, cfg.Tolerance)
}
