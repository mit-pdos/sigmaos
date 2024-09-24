package remote

import (
	"encoding/json"
	"fmt"
	"os"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type BenchConfig struct {
	Platform   sp.Tplatform `json:"platform"`
	VPC        string       `json:"vpc"`
	Tag        string       `json:"tag"`
	Branch     string       `json:"branch"`
	Version    string       `json:"version"`
	Debug      string       `json:"debug"`
	Perf       string       `json:"perf"`
	NoNetproxy bool         `json:"no_netproxy"`
	Overlays   bool         `json:"overlays"`
	Parallel   bool         `json:"parallel"`
	NoShutdown bool         `json:"no_shutdown"`
	K8s        bool         `json:"k8s"`
}

// Return a new benchmark config, given the flag arguments used to run the
// remote benchmarks
func NewBenchConfig() (*BenchConfig, error) {
	cfg := &BenchConfig{
		Platform:   sp.Tplatform(platformArg),
		VPC:        vpcArg,
		Tag:        tagArg,
		Branch:     branchArg,
		Version:    versionArg,
		Debug:      os.Getenv(proc.SIGMADEBUG),
		Perf:       os.Getenv(proc.SIGMAPERF),
		NoNetproxy: noNetproxyArg,
		Overlays:   overlaysArg,
		Parallel:   parallelArg,
		NoShutdown: noShutdownArg,
		K8s:        k8sArg,
	}
	// Check that required arguments have been set
	if cfg.Platform == sp.NOT_SET {
		return nil, fmt.Errorf("platform not set")
	}
	if cfg.Platform != sp.PLATFORM_AWS && cfg.Platform != sp.PLATFORM_CLOUDLAB {
		return nil, fmt.Errorf("unrecognized platform: %v", cfg.Platform)
	}
	if cfg.Platform == sp.PLATFORM_AWS && cfg.VPC == sp.NOT_SET {
		return nil, fmt.Errorf("vpc not set for platform AWS")
	}
	if cfg.Platform == sp.PLATFORM_CLOUDLAB {
		cfg.VPC = "no-vpc"
	}
	if cfg.Tag == sp.NOT_SET {
		return nil, fmt.Errorf("tag not set")
	}
	if cfg.Branch == sp.NOT_SET {
		return nil, fmt.Errorf("branch not set")
	}
	if cfg.Version == sp.NOT_SET {
		return nil, fmt.Errorf("version not set")
	}
	if cfg.K8s && cfg.Platform != sp.PLATFORM_CLOUDLAB {
		return nil, fmt.Errorf("k8s is only supported on cloudlab")
	}
	if cfg.Overlays && !cfg.NoNetproxy {
		return nil, fmt.Errorf("Should not run with overlays AND netproxy!")
	}
	if !cfg.Overlays && cfg.NoNetproxy {
		return nil, fmt.Errorf("Should not run without overlays AND without netproxy!")
	}
	return cfg, nil
}

func (cfg *BenchConfig) String() string {
	b, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		db.DFatalf("Marshal cfg: %v", err)
	}
	return string(b)
}
