package python

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/pyenv"
	"sigmaos/pyenv/clnt"
	"sigmaos/pyenv/pylock"
	"sigmaos/util/perf"

	lru "github.com/hashicorp/golang-lru/v2"
)

type TPySitePackagesType string

const (
	OverlaySPType    TPySitePackagesType = "overlayfs"
	SymlinkSPType    TPySitePackagesType = "symlink"
	PythonPathSPType TPySitePackagesType = "pythonpath"
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

// SetupSitePackages sets up the site-packages directory by installing all required wheels
// and mounting an overlayFS. It atomically acquires locks on all wheels.
// Returns the path to the site-packages directory and a lock handle.
// The lock handle must be released when the process is done with the packages.
func SetupSitePackages(
	uproc *proc.Proc,
	workingDir string,
	pyVersion *pyenv.PythonVersion,
	pylockPath string,
	stType TPySitePackagesType,
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

	s = time.Now()
	if stType == OverlaySPType {
		overlayDir, err := mountOverlayFS(workingDir, installPaths)
		perf.LogSpawnLatency("SetupSitePackages mountOverlayFS", uproc.GetPid(), uproc.GetSpawnTime(), s)
		if err != nil {
			// Release locks on failure
			pyenvClnt.ReleaseLocks(handle)
			return "", 0, err
		}
		return filepath.Join(overlayDir, "site-packages"), handle, nil
	} else if stType == SymlinkSPType {
		symlinkDir, err := symlinkSitePackages(workingDir, installPaths)
		perf.LogSpawnLatency("SetupSitePackages symlinkSitePackages", uproc.GetPid(), uproc.GetSpawnTime(), s)
		if err != nil {
			// Release locks on failure
			pyenvClnt.ReleaseLocks(handle)
			return "", 0, fmt.Errorf("failed to set up site-packages symlinks: %w", err)
		}
		return filepath.Join(symlinkDir, "site-packages"), handle, nil
	} else if stType == PythonPathSPType {
		for i, path := range installPaths {
			installPaths[i] = filepath.Join(path, "site-packages")
		}
		return strings.Join(installPaths, ":"), handle, nil
	}

	return "", 0, fmt.Errorf("unknown site-packages type: %v", stType)
}

func mountOverlayFS(workingDir string, lowerdirs []string) (string, error) {
	upperdir := filepath.Join(workingDir, "upper")
	workdir := filepath.Join(workingDir, "work")
	target := filepath.Join(workingDir, "overlay")

	for _, d := range append(lowerdirs, upperdir, workdir, target) {
		if err := os.MkdirAll(d, 0755); err != nil {
			return "", err
		}
	}

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s",
		strings.Join(lowerdirs, ":"), upperdir, workdir)

	// Use fuse-overlayfs to allow creating an overlayFS inside the docker overlayFS
	cmd := exec.Command("fuse-overlayfs", "-o", opts, target)
	if err := cmd.Run(); err != nil {
		// fuse.overlayfs tends to return non-zero exit code even on success
		// with error: "unknown argument ignored: lazytime"
		// So we double-check with findmnt if the mount was successful.
		findmntCmd := exec.Command("findmnt", "-n", "-t", "fuse.fuse-overlayfs", "-T", target)
		if findmntCmd.Run() != nil {
			return "", fmt.Errorf("setting up python site-packages overlayfs failed (%v): %w", cmd, err)
		}
	}

	return target, nil
}

func symlinkSitePackages(workingDir string, lowerdirs []string) (string, error) {
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return "", err
	}

	err := symlinkMerge(workingDir, lowerdirs)
	if err != nil {
		return "", err
	}

	return workingDir, nil
}

// Merges multiple input directories into a single output directory using symlinks.
// In case of filename collisions, creates subdirectories to resolve them.
func symlinkMerge(outDir string, inputDirs []string) error {
	// Map to group paths by their filename
	// Key: Filename (e.g., "a", "b")
	// Value: List of full paths where this file exists (e.g., ["/abs/foo/a", "/abs/bar/a"])
	entries := make(map[string][]string)

	for _, src := range inputDirs {
		items, err := os.ReadDir(src)
		if err != nil {
			return fmt.Errorf("failed to read dir %s: %w", src, err)
		}

		for _, item := range items {
			name := item.Name()
			fullPath := filepath.Join(src, name)
			entries[name] = append(entries[name], fullPath)
		}
	}

	for name, paths := range entries {
		targetOut := filepath.Join(outDir, name)
		// CASE 1: Unique (Only exists in one source)
		// We can just symlink the whole thing, whether it's a file or a folder.
		// This handles the "out/b -> bar/b" requirement.
		if len(paths) == 1 {
			if err := os.Symlink(paths[0], targetOut); err != nil {
				return err
			}
			continue
		}

		// CASE 2: Collision File
		info, err := os.Stat(paths[0])
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// If it's a file, we create a symlink to the first path in the list.
			if err := os.Symlink(paths[0], targetOut); err != nil {
				return err
			}
			continue
		}

		// CASE 3: Collision Directory
		if err := os.MkdirAll(targetOut, 0755); err != nil {
			return fmt.Errorf("failed to create merge dir %s: %w", targetOut, err)
		}
		if err := symlinkMerge(targetOut, paths); err != nil {
			return err
		}
	}

	return nil
}

// CleanSitePackages unmounts the site-packages overlayFS.
// Note: Lock release is handled separately by the caller using the lock handle
// returned from SetupSitePackages.
func CleanSitePackages(workingDir string) error {
	target := filepath.Join(workingDir, "overlay")
	if err := syscall.Unmount(target, 0); err != nil {
		return fmt.Errorf("failed to unmount overlayFS: %w", err)
	}
	return nil
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
