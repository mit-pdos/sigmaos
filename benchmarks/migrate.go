package benchmarks

import (
	"fmt"
	"time"
)

type MigrationConfig struct {
	Svc              string          `json:"svc"`
	Migrate          bool            `json:"migrate"`
	MigrationDelays  []time.Duration `json:"migration_delays"`
	MigrationTargets []int           `json:"migration_targets"`
}

func NewMigrationConfig(svc string, migrate bool, migrationDelays []time.Duration, migrationTargets []int) *MigrationConfig {
	return &MigrationConfig{
		Svc:              svc,
		Migrate:          migrate,
		MigrationDelays:  migrationDelays,
		MigrationTargets: migrationTargets,
	}
}

func (cfg *MigrationConfig) String() string {
	return fmt.Sprintf("&{ svc:%v migrate:%v delays:%v targets:%v }", cfg.Svc, cfg.Migrate, cfg.MigrationDelays, cfg.MigrationTargets)
}
