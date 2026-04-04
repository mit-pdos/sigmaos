package python

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/pyenv"
	"sigmaos/pyenv/clnt"
	"sigmaos/pyenv/pylock"
	"sigmaos/util/perf"

	lru "github.com/hashicorp/golang-lru/v2"
)

type cacheKey struct {
	path    string
	version int
}

var (
	requiredWheelsCache *lru.Cache[cacheKey, []*pylock.Wheel]
	once                sync.Once
)

func initCache() {
	once.Do(func() {
		cache, err := lru.New[cacheKey, []*pylock.Wheel](32)
		if err != nil {
			panic(fmt.Sprintf("failed to create LRU cache: %v", err))
		}
		requiredWheelsCache = cache
	})
}

func prepareSysTagRanks(compatibilityTags []string) map[string]int {
	ranks := make(map[string]int)
	for i, tag := range compatibilityTags {
		ranks[tag] = i
	}
	return ranks
}

// last3DashFields returns the last three '-' separated fields of s,
// joined by '-' (i.e. the substring from the 3rd dash-from-end onward).
// If fewer than 3 dashes exist, it returns "" and false.
func last3DashFields(s string) (string, bool) {
	end := len(s)
	dashCount := 0

	for i := end - 1; i >= 0; i-- {
		if s[i] == '-' {
			dashCount++
			if dashCount == 3 {
				// substring after this dash to end
				return s[i+1:], true
			}
		}
	}

	return "", false
}

// Returns the wheel that best matches the compatibility tags supported by sigmaos.
// Compatibility tags (e.g. cp311-cp311-manylinux_2_39_x86_64) are ordered from
// most preferred to least preferred.
func getBestWheel(pkg pylock.Package, sysTagRanks map[string]int) (*pylock.Wheel, error) {
	if len(pkg.Wheels) == 0 {
		return nil, fmt.Errorf("package %q has no wheels", pkg.Name)
	}

	var best *pylock.Wheel
	bestRank := len(sysTagRanks)

	for i := range pkg.Wheels {
		w := &pkg.Wheels[i]

		tagTripple, ok := last3DashFields(strings.TrimSuffix(w.Name, ".whl"))
		if !ok {
			continue
		}

		// Fast path: No compressed tags
		if strings.IndexByte(tagTripple, '.') < 0 {
			if rank, ok := sysTagRanks[tagTripple]; ok && rank < bestRank {
				best = w
				bestRank = rank
			}
			continue
		}

		// Slow path: Need to expand compressed tags
		parts := strings.Split(tagTripple, "-")

		// Expand any compressed tag triples
		pytags := strings.Split(parts[0], ".")
		abitags := strings.Split(parts[1], ".")
		platformtags := strings.Split(parts[2], ".")

		for _, py := range pytags {
			for _, abi := range abitags {
				for _, plat := range platformtags {
					tagTriple := py + "-" + abi + "-" + plat
					if rank, ok := sysTagRanks[tagTriple]; ok && rank < bestRank {
						best = w
						bestRank = rank
					}
				}
			}
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no compatible wheel found for %q", pkg.Name)
	}
	return best, nil
}

func getRequiredWheels(lock *pylock.Pylock, pyVersion *pyenv.PythonVersion) ([]*pylock.Wheel, error) {
	var wheels []*pylock.Wheel
	envMarkers := pyVersion.EnvMarkers()
	sysTagRanks := prepareSysTagRanks(pyVersion.SysTags())
	markerCache := make(map[string]bool)

	for _, pkg := range lock.Packages {
		isRequired, ok := markerCache[pkg.Marker]
		if !ok {
			var err error
			isRequired, err = pylock.EvaluateMarker(pkg.Marker, envMarkers)
			if err != nil {
				return nil, err
			}
			markerCache[pkg.Marker] = isRequired
		}

		db.DPrintf(db.CONTAINER, "Python package %v (%v) required: %v (%v)", pkg.Name, pkg.Version, isRequired, pkg.Marker)
		if !isRequired {
			continue
		}

		wheel, err := getBestWheel(pkg, sysTagRanks)
		if err != nil {
			return nil, err
		}

		wheels = append(wheels, wheel)
	}

	return wheels, nil
}

// SetupSitePackages sets up the site-packages directory by installing all required wheels.
// It atomically acquires locks on all wheels.
// Returns the path to the site-packages directory and a lock handle.
// The lock handle must be released when the process is done with the packages.
func SetupSitePackages(
	uproc *proc.Proc,
	pyVersion *pyenv.PythonVersion,
	pylockPath string,
	pyenvClnt *clnt.PyEnvClnt,
) (string, clnt.LockHandle, error) {
	initCache()

	var wheels []*pylock.Wheel
	var ok bool
	var err error

	key := cacheKey{path: pylockPath, version: pyVersion.Index()}
	if wheels, ok = requiredWheelsCache.Get(key); !ok {
		s := time.Now()
		lock, err := pylock.ParsePylock(pylockPath)
		perf.LogSpawnLatency("SetupSitePackages pylock.ParsePylock", uproc.GetPid(), uproc.GetSpawnTime(), s)
		if err != nil {
			return "", 0, err
		}

		s = time.Now()
		wheels, err = getRequiredWheels(lock, pyVersion)
		perf.LogSpawnLatency("SetupSitePackages getRequiredWheels", uproc.GetPid(), uproc.GetSpawnTime(), s)
		if err != nil {
			return "", 0, err
		}

		requiredWheelsCache.Add(key, wheels)
	}

	if len(wheels) == 0 {
		// No wheels needed - return empty site-packages path and empty lock handle
		return "", 0, nil
	}

	if db.WillBePrinted(db.CONTAINER) {
		totalSize := int64(0)
		for _, wheel := range wheels {
			totalSize += wheel.Size
		}
		db.DPrintf(db.CONTAINER, "Total size of required python wheels: %d bytes", totalSize)
	}

	// Install all wheels atomically and acquire locks
	// This ensures all-or-nothing semantics
	s := time.Now()
	installPaths, handle, err := pyenvClnt.InstallWheels(wheels, pyVersion)
	perf.LogSpawnLatency("SetupSitePackages pyenvClnt.InstallWheels", uproc.GetPid(), uproc.GetSpawnTime(), s)
	if err != nil {
		return "", 0, fmt.Errorf("failed to install wheels: %w", err)
	}

	for i, path := range installPaths {
		installPaths[i] = filepath.Join(path, "site-packages")
	}
	return strings.Join(installPaths, ":"), handle, nil
}

func GetPythonFileArg(args []string) (string, int, error) {
	for i, arg := range args {
		if strings.HasSuffix(arg, ".py") && !strings.HasPrefix(arg, "-") {
			return arg, i, nil
		}
	}
	return "", -1, fmt.Errorf("no python file argument found")
}

func GetPylockPath(workingDir string, pythonFile string) (string, error) {
	dir := filepath.Dir(pythonFile)
	pylockFileNames := []string{"pylock.sigmaos.toml", "pylock.toml"}
	for {
		for _, name := range pylockFileNames {
			lockPath := filepath.Join(workingDir, dir, name)
			if _, err := os.Stat(lockPath); err == nil {
				return lockPath, nil
			}
		}

		dir = filepath.Dir(dir)
		if dir == "/" || dir == "." {
			break
		}
	}
	return "", fmt.Errorf("no pylock file found")
}
