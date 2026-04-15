// PyMgr manages Python wheel installation and caching across all SigmaOS instances.
// This is a MACHINE-SCOPED manager - all instances on the same machine share the same
// wheel cache at /tmp/python/package-cache. PyMgr should only exist as a service
// (pysrvd), not as a library imported by multiple services.
package pyenv

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"sync"
	"sync/atomic"

	db "sigmaos/debug"
	proto "sigmaos/pyenv/proto"
	sessp "sigmaos/session/proto"

	"golang.org/x/sync/errgroup"
)

type installResult struct {
	path     string
	err      error
	refCount zeroListItem
	sha256   string
	pyIdx    int
}

// LockHandle uniquely identifies a set of acquired locks
type LockHandle struct {
	SessionID   sessp.Tsession
	HandleID    uint64
	refs        []*installResult // References to release on unlock
	releaseOnce sync.Once        // Ensures refs are released exactly once
}

// PyMgr manages Python wheel installation and caching.
// This is a MACHINE-SCOPED SINGLETON, not instance-scoped.
// All SigmaOS instances on the same machine share the same wheel cache
// at /tmp/python/package-cache. This ensures:
// - Wheels are downloaded/installed only once per machine
// - Concurrent operations across instances are coordinated
// - Cache persists across instance restarts
type PyMgr struct {
	mu               sync.RWMutex
	installedWheels  []map[string]*installResult // keyed by sha256, indexed by pyVersion.index
	downloadedWheels map[string]string           // keyed by URL, stores path
	pendingDownloads map[string]*sync.Cond       // keyed by URL
	pendingInstalls  []map[string]*sync.Cond     // keyed by sha256, indexed by pyVersion.index

	installSem  chan struct{}
	downloadSem chan struct{}
	compileSem  chan struct{}

	// Session tracking for automatic lock release
	sessionLocks   map[sessp.Tsession]map[uint64]*LockHandle // session -> handleID -> handle
	sessionLocksMu sync.Mutex

	// Handle counter for generating unique lock handles (starts at 1)
	handleCounter atomic.Uint64

	evictionList zeroList  // List of wheels with refcount=0, ordered by recency of becoming unused
	mode         PyMgrMode // Used for benchmarking
}

// This is only used for benchmarking
// Can be set using the SIGMAPYMGRMODE environment variable when booting the PySrvd

type PyMgrMode string

const (
	DefaultMode = PyMgrMode("default") // Normal caching behaviour
	PrivateMode = PyMgrMode("private") // Evict packages immediately to simulate unfilled cache on every install
	NoPYCMode   = PyMgrMode("no_pyc")  // Don't precompile .pyc files
)

// generateHandleID returns a unique uint64 handle ID
func (pm *PyMgr) generateHandleID() uint64 {
	return pm.handleCounter.Add(1)
}

func NewPyMgr(mode PyMgrMode) *PyMgr {
	numCPU := runtime.NumCPU()

	// Initialize Python versions first to get the count
	initPythonVersions()
	numVersions := len(pyVersions)

	// Create two-tiered lookup structures - one map per Python version
	installedWheels := make([]map[string]*installResult, numVersions)
	pendingInstalls := make([]map[string]*sync.Cond, numVersions)
	for i := 0; i < numVersions; i++ {
		installedWheels[i] = make(map[string]*installResult)
		pendingInstalls[i] = make(map[string]*sync.Cond)
	}

	db.DPrintf(db.PYENV, "Initialized PyMgr with numCPU=%d, mode=%s", numCPU, mode)

	return &PyMgr{
		installedWheels:  installedWheels,
		downloadedWheels: make(map[string]string),
		pendingDownloads: make(map[string]*sync.Cond),
		pendingInstalls:  pendingInstalls,

		installSem:  make(chan struct{}, numCPU),
		downloadSem: make(chan struct{}, min(32, numCPU+4)),
		compileSem:  make(chan struct{}, numCPU),

		sessionLocks: make(map[sessp.Tsession]map[uint64]*LockHandle),

		evictionList: newZeroList(),
		mode:         mode,
	}
}

// InstallWheels installs all wheels and acquires read locks on them atomically.
// All-or-nothing semantics: either all locks are acquired and all wheels are installed,
// or the operation fails and no locks are held.
func (pm *PyMgr) InstallWheels(wheels []*proto.Wheel, pyVersion *PythonVersion, sessionID sessp.Tsession) (*LockHandle, []string, error) {
	pyIdx := pyVersion.Index()

	// First pass: check if all wheels can be acquired (not evicted) and install missing ones
	// We need to do this carefully to avoid deadlocks and ensure atomicity

	// Generate unique handle ID
	handleID := pm.generateHandleID()

	// Results and acquired locks
	installPaths := make([]string, len(wheels))
	installResults := make([]*installResult, len(wheels))
	sha256s := make([]string, len(wheels))

	// Step 1: Get sha256 for all wheels and verify they have hashes
	for i, wheel := range wheels {
		sha256 := wheel.Hashes.Sha256
		if sha256 == "" {
			return nil, nil, fmt.Errorf("wheel %s missing sha256 hash", wheel.Name)
		}
		sha256s[i] = sha256
	}

	// Ensure all wheels are installed (or install them).
	// This step also acquires a refcount on each wheel if installed successfully.
	eg := errgroup.Group{}
	for i, wheel := range wheels {
		ii, iwheel := i, wheel

		eg.Go(func() error {
			sha256 := sha256s[ii]
			result, err := pm.getOrInstallWheel(iwheel, pyVersion, pyIdx, sha256)
			if err != nil {
				return fmt.Errorf("failed to install %s: %w", iwheel.Name, err)
			}

			installPaths[ii] = result.path
			installResults[ii] = result
			return nil
		})
	}

	err := eg.Wait()
	if err != nil {
		for _, result := range installResults {
			if result != nil && result.err == nil {
				result.refCount.release(&pm.evictionList)
			}
		}
		return nil, nil, err
	}

	// Create the lock handle
	handle := &LockHandle{
		SessionID: sessionID,
		HandleID:  handleID,
		refs:      installResults,
	}

	// Register the handle with the session
	pm.sessionLocksMu.Lock()
	if pm.sessionLocks[sessionID] == nil {
		pm.sessionLocks[sessionID] = make(map[uint64]*LockHandle)
	}
	pm.sessionLocks[sessionID][handleID] = handle
	pm.sessionLocksMu.Unlock()

	return handle, installPaths, nil
}

// getOrInstallWheel ensures a wheel is installed and returns its result.
// Increases the refcount by 1.
func (pm *PyMgr) getOrInstallWheel(wheel *proto.Wheel, pyVersion *PythonVersion, pyIdx int, sha256 string) (*installResult, error) {
	// Fast path - Already installed
	pm.mu.RLock()
	if result := pm.installedWheels[pyIdx][sha256]; result != nil {
		if result.err != nil {
			pm.mu.RUnlock()
			return nil, result.err
		}
		result.refCount.acquire(&pm.evictionList)
		pm.mu.RUnlock()
		return result, nil
	}
	pm.mu.RUnlock()

	// Slow path
	pm.mu.Lock()
	result := pm.installedWheels[pyIdx][sha256]
	if result != nil {
		if result.err != nil {
			pm.mu.Unlock()
			return nil, result.err
		}
		result.refCount.acquire(&pm.evictionList)
		pm.mu.Unlock()
		return result, nil
	}

	// Check file system - this case happens when we start up with a non-empty cache
	if result == nil {
		result = checkIfInstalled(wheel, pyVersion)
		if result != nil {
			pm.installedWheels[pyIdx][sha256] = result
			if result.err != nil {
				pm.mu.Unlock()
				return nil, result.err
			}
			result.refCount.acquire(&pm.evictionList)
			pm.mu.Unlock()
			return result, nil
		}
		pm.installedWheels[pyIdx][sha256] = nil
	}
	pm.mu.Unlock()

	// Need to install
	if wheel.Url == "" {
		return nil, fmt.Errorf("cannot install wheel without URL: %v", wheel.Name)
	}

	// Download (deduplicated by URL)
	wheelPath, err := pm.downloadWheel(wheel)
	if err != nil {
		return nil, err
	}

	// Install (deduplicated by sha256 + version)
	return pm.installWheel(wheel, pyVersion, wheelPath, sha256)
}

// ReleaseLocks releases all locks associated with a handle.
func (pm *PyMgr) ReleaseLocks(handle *LockHandle) error {
	if handle == nil {
		return nil
	}

	// Release all refs exactly once using sync.Once
	handle.releaseOnce.Do(func() {
		for _, ref := range handle.refs {
			isZero := ref.refCount.release(&pm.evictionList)

			if isZero && pm.mode == PrivateMode {
				pm.TryEvict(ref.sha256, ref.pyIdx)
			}
		}
	})

	// Remove from session tracking
	pm.sessionLocksMu.Lock()
	if sessionHandles, ok := pm.sessionLocks[handle.SessionID]; ok {
		delete(sessionHandles, handle.HandleID)
		if len(sessionHandles) == 0 {
			delete(pm.sessionLocks, handle.SessionID)
		}
	}
	pm.sessionLocksMu.Unlock()

	return nil
}

// ReleaseAllSessionLocks releases all locks held by a session.
// This is called automatically when a session disconnects.
func (pm *PyMgr) ReleaseAllSessionLocks(sessionID sessp.Tsession) {
	pm.sessionLocksMu.Lock()
	sessionHandles, ok := pm.sessionLocks[sessionID]
	if !ok {
		pm.sessionLocksMu.Unlock()
		return
	}
	// Copy handles to avoid holding lock during release
	handles := make([]*LockHandle, 0, len(sessionHandles))
	for _, handle := range sessionHandles {
		handles = append(handles, handle)
	}
	delete(pm.sessionLocks, sessionID)
	pm.sessionLocksMu.Unlock()

	// Release all locks (each handle's refs are released at most once via sync.Once)
	for _, handle := range handles {
		handle.releaseOnce.Do(func() {
			for _, ref := range handle.refs {
				isZero := ref.refCount.release(&pm.evictionList)

				if isZero && pm.mode == PrivateMode {
					pm.TryEvict(ref.sha256, ref.pyIdx)
				}
			}
		})
	}
}

// SessionLocksMu returns the mutex for sessionLocks (for external access).
func (pm *PyMgr) SessionLocksMu() *sync.Mutex {
	return &pm.sessionLocksMu
}

// SessionLocks returns the sessionLocks map (for external access).
// Caller must hold SessionLocksMu().
func (pm *PyMgr) SessionLocks() map[sessp.Tsession]map[uint64]*LockHandle {
	return pm.sessionLocks
}

// TryEvict attempts to evict a specific wheel if its refcount is 0.
// Returns true if successfully evicted.
func (pm *PyMgr) TryEvict(sha256 string, pyIdx int) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	result := pm.installedWheels[pyIdx][sha256]
	if result == nil || result.err != nil {
		return false // Not installed or failed installation
	}

	// Try to close (mark as evicted)
	if !result.refCount.tryClose(&pm.evictionList) {
		return false
	}

	// Remove from installed wheels
	delete(pm.installedWheels[pyIdx], sha256)

	// Remove from disk
	err := os.RemoveAll(result.path)
	if err != nil {
		db.DPrintf(db.PYENV_ERR, "Failed to remove wheel at %s: %v", result.path, err)
		return false
	}

	return true
}

func (pm *PyMgr) downloadWheel(wheel *proto.Wheel) (string, error) {
	pm.mu.Lock()
	// Return cached download
	if p, found := pm.downloadedWheels[wheel.Url]; found {
		pm.mu.Unlock()
		return p, nil
	}

	// Wait for pending download
	if cond, pending := pm.pendingDownloads[wheel.Url]; pending {
		pm.mu.Unlock()
		cond.L.Lock()
		cond.Wait()
		cond.L.Unlock()

		pm.mu.RLock()
		p, found := pm.downloadedWheels[wheel.Url]
		pm.mu.RUnlock()
		if !found {
			return "", fmt.Errorf("download failed for %s", wheel.Url)
		}
		return p, nil
	}
	cond := sync.NewCond(&sync.Mutex{})
	pm.pendingDownloads[wheel.Url] = cond
	pm.mu.Unlock()

	// Perform download (verifies hash internally)
	p, err := func() (string, error) {
		pm.downloadSem <- struct{}{}
		defer func() { <-pm.downloadSem }()
		db.DPrintf(db.PYENV, "Downloading wheel %s", wheel.Name)
		return DownloadWheel(wheel)
	}()

	// Update state and notify waiters
	pm.mu.Lock()
	if err == nil {
		pm.downloadedWheels[wheel.Url] = p
	}
	delete(pm.pendingDownloads, wheel.Url)
	pm.mu.Unlock()

	cond.L.Lock()
	cond.Broadcast()
	cond.L.Unlock()

	return p, err
}

// Increases the refcount for the wheel by one if the installation was successful, and returns the installResult.
func (pm *PyMgr) installWheel(wheel *proto.Wheel, pyVersion *PythonVersion, wheelPath string, sha256 string) (*installResult, error) {
	pyIdx := pyVersion.Index()

	pm.mu.Lock()
	// Check if installed while downloading
	if result := pm.installedWheels[pyIdx][sha256]; result != nil {
		if result.err != nil {
			pm.mu.Unlock()
			return nil, result.err
		}
		result.refCount.acquire(&pm.evictionList)
		pm.mu.Unlock()
		return result, nil
	}

	// Wait for pending install
	if cond := pm.pendingInstalls[pyIdx][sha256]; cond != nil {
		pm.mu.Unlock()
		cond.L.Lock()
		cond.Wait()
		cond.L.Unlock()

		pm.mu.RLock()
		result := pm.installedWheels[pyIdx][sha256]
		if result == nil {
			pm.mu.RUnlock()
			return nil, fmt.Errorf("installation failed for %s", wheel.Name)
		}
		if result.err != nil {
			pm.mu.RUnlock()
			return nil, result.err
		}
		result.refCount.acquire(&pm.evictionList)
		pm.mu.RUnlock()
		return result, nil
	}
	cond := sync.NewCond(&sync.Mutex{})
	pm.pendingInstalls[pyIdx][sha256] = cond
	pm.mu.Unlock()

	// Perform installation
	var installPath string
	var tmpInstallPath string
	var err error

	installPath, err = GetWheelInstallPath(wheel, pyVersion)
	if err != nil {
		goto exitUnlocked
	}
	tmpInstallPath, err = func() (string, error) {
		pm.installSem <- struct{}{}
		defer func() { <-pm.installSem }()
		db.DPrintf(db.PYENV, "Installing wheel %s for Python %s at %s", wheel.Name, pyVersion.Version(), installPath)
		return InstallWheel(wheelPath, pyVersion)
	}()

	if err != nil {
		goto exitUnlocked
	}

	err = os.MkdirAll(path.Dir(installPath), 0777)
	if err != nil {
		goto exitUnlocked
	}

	pm.mu.Lock()
	// Moving to the final location, and updating the installedWheels map must be
	// done while holding the lock.
	err = os.Rename(tmpInstallPath, installPath)
	if err != nil {
		os.RemoveAll(tmpInstallPath)
		goto exitLocked
	}

	// Delete the downloaded wheel file after successful installation
	if wheelPath != "" {
		if removeErr := os.Remove(wheelPath); removeErr != nil {
			db.DPrintf(db.PYENV_ERR, "Failed to delete downloaded wheel %s: %v", wheelPath, removeErr)
		}
		delete(pm.downloadedWheels, wheel.Url)
	}

	// Start pre-compiling in background
	go func() {
		if pm.mode != DefaultMode {
			return
		}

		pm.compileSem <- struct{}{}
		defer func() { <-pm.compileSem }()
		pm.mu.RLock()
		result := pm.installedWheels[pyIdx][sha256]
		pm.mu.RUnlock()
		if result == nil || result.err != nil {
			return
		}

		if err := CompilePythonFiles(installPath, pyVersion); err != nil {
			db.DPrintf(db.PYENV_ERR, "Failed to compile wheel %s: %v", wheel.Name, err)
		}
	}()

	goto exitLocked

exitUnlocked:
	pm.mu.Lock()
exitLocked:
	if err != nil {
		installPath = ""
	}
	result := &installResult{
		path: installPath, err: err,
		sha256: sha256, pyIdx: pyIdx,
	}
	pm.installedWheels[pyIdx][sha256] = result
	delete(pm.pendingInstalls[pyIdx], sha256)
	if err == nil {
		result.refCount.acquire(&pm.evictionList)
	}
	pm.mu.Unlock()

	cond.L.Lock()
	cond.Broadcast()
	cond.L.Unlock()
	return result, err
}

// Checks if a wheel installation is already present on disk.
func checkIfInstalled(wheel *proto.Wheel, pyVersion *PythonVersion) *installResult {
	installPath, err := GetWheelInstallPath(wheel, pyVersion)
	if err != nil {
		return &installResult{path: "", err: err}
	}

	sha256 := wheel.Hashes.Sha256
	if s, err := os.Stat(installPath); err == nil && s.IsDir() {
		return &installResult{
			path: installPath, err: nil,
			sha256: sha256, pyIdx: pyVersion.Index(),
		}
	}

	return nil
}
