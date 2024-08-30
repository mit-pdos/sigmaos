package remote

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

// Configuration for local test runner machine's FS
type LocalFSConfig struct {
	RootDir string `json:"project_root"`
	// Script directories
	ScriptDir      string `json:"script_dir"`
	GraphScriptDir string `json:"graph_script_dir"`
	// Output directories
	OutputDir      string `json:"output_dir"`
	GraphOutputDir string `json:"graph_output_dir"`
	// Global script running options
	Parallel bool `json:"parallel"`
}

func NewLocalFSConfig(platform sp.Tplatform, version string, parallel bool) (*LocalFSConfig, error) {
	pkgpath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("Err os.Executable: %v", err)
	}
	root := filepath.Dir(filepath.Dir(pkgpath))
	var scriptDir string
	switch platform {
	case sp.PLATFORM_AWS:
		scriptDir = filepath.Join(root, AWS_DIR_REL)
	case sp.PLATFORM_CLOUDLAB:
		scriptDir = filepath.Join(root, CLOUDLAB_DIR_REL)
	default:
		return nil, fmt.Errorf("unknown script dir for platform %v", platform)
	}
	lcfg := &LocalFSConfig{
		RootDir:        root,
		ScriptDir:      scriptDir,
		GraphScriptDir: filepath.Join(root, GRAPH_SCRIPT_DIR_REL),
		OutputDir:      filepath.Join(root, OUTPUT_PARENT_DIR_REL, version),
		GraphOutputDir: filepath.Join(root, GRAPH_OUTPUT_DIR_REL),
		Parallel:       parallel,
	}
	if err := lcfg.setupFS(); err != nil {
		return nil, err
	}
	return lcfg, nil
}

// Get the path of the output directory for a given benchmark
func (lcfg *LocalFSConfig) GetOutputDirPath(benchName string) string {
	return filepath.Join(lcfg.OutputDir, benchName)
}

// Create an output directory
func (lcfg *LocalFSConfig) CreateOutputDir(outputDirPath string) error {
	return os.MkdirAll(outputDirPath, 0777)
}

// Check if output directory already exists
func (lcfg *LocalFSConfig) OutputExists(outputDirPath string) bool {
	_, err := os.Stat(outputDirPath)
	return err == nil
}

func (lcfg *LocalFSConfig) WriteBenchmarkConfig(outputDirPath string, bcfg *BenchConfig, ccfg *ClusterConfig, leaderBenchCmd, followerBenchCmd string) error {
	return os.WriteFile(filepath.Join(outputDirPath, BENCH_CONFIG_FILE), []byte(fmt.Sprintf("Bench config:\n%v\nCluster config:\n%v\nLeader benchCmd:\n%v\nFollower benchCmd:\n%v", bcfg, ccfg, leaderBenchCmd, followerBenchCmd)), 0644)
}

// Set up the file system for benchmarking
func (lcfg *LocalFSConfig) setupFS() error {
	// Check that script directories exist
	if fi, err := os.Stat(lcfg.RootDir); err != nil {
		return fmt.Errorf("Can't stat RootDir: %v", err)
	} else if !fi.Mode().IsDir() {
		return fmt.Errorf("RootDir isn't dir")
	}
	if fi, err := os.Stat(lcfg.ScriptDir); err != nil {
		return fmt.Errorf("Can't stat ScriptDir: %v", err)
	} else if !fi.Mode().IsDir() {
		return fmt.Errorf("ScriptDir isn't dir")
	}
	if fi, err := os.Stat(lcfg.GraphScriptDir); err != nil {
		return fmt.Errorf("Can't stat GraphScriptDir: %v", err)
	} else if !fi.Mode().IsDir() {
		return fmt.Errorf("GraphScriptDir isn't dir")
	}
	// Make output directories, if necessary
	if err := os.MkdirAll(lcfg.OutputDir, 0777); err != nil {
		return fmt.Errorf("Can't make OutputDir: %v", err)
	}
	if err := os.MkdirAll(lcfg.GraphOutputDir, 0777); err != nil {
		return fmt.Errorf("Can't make OutputDir: %v", err)
	}
	// Clear the cluster init log
	os.Remove(CLUSTER_INIT_LOG)
	return nil
}

func (lcfg *LocalFSConfig) String() string {
	b, err := json.MarshalIndent(lcfg, "", "\t")
	if err != nil {
		db.DFatalf("Marshal local FS config: %v", err)
	}
	return string(b)
}

func (lcfg *LocalFSConfig) getScriptCmd(scriptName string, wr io.Writer, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	if lcfg.Parallel {
		a := []string{"--parallel"}
		cmd = exec.Command(scriptName, append(a, args...)...)
	} else {
		cmd = exec.Command(scriptName, args...)
	}
	cmd.Dir = lcfg.ScriptDir
	if wr != nil {
		cmd.Stdout = wr
		cmd.Stderr = wr
	}
	return cmd
}

func (lcfg *LocalFSConfig) runScriptRedirectOutput(scriptName string, wr io.Writer, args ...string) error {
	cmd := lcfg.getScriptCmd(scriptName, wr, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Err runScriptRedirectOutput %v:\n%v:\n%s", filepath.Join(lcfg.ScriptDir, scriptName), err, err.(*exec.ExitError).Stderr)
	}
	return nil
}

// Run a script synchronously and return its output
func (lcfg *LocalFSConfig) RunScriptGetOutput(scriptName string, args ...string) (string, error) {
	cmd := lcfg.getScriptCmd(scriptName, nil, args...)
	b, err := cmd.Output()
	out := strings.TrimSpace(string(b))
	if err != nil {
		return out, fmt.Errorf("Err runScript %v:\n%v:\n%s", filepath.Join(lcfg.ScriptDir, scriptName), err, err.(*exec.ExitError).Stderr)
	}
	return out, nil
}

// Run a script synchronously and redirect its output to stdout
func (lcfg *LocalFSConfig) RunScriptRedirectOutputStdout(scriptName string, args ...string) error {
	return lcfg.runScriptRedirectOutput(scriptName, os.Stdout, args...)
}

// Run a script synchronously and redirect its output to a file, in append mode
func (lcfg *LocalFSConfig) RunScriptRedirectOutputFile(scriptName, outFilePath string, args ...string) error {
	// Create the output file, or append to it if it exists already
	outFile, err := os.OpenFile(outFilePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Errorf("Err open outFile [%v]: %v", outFilePath, err)
	}
	defer outFile.Close()
	return lcfg.runScriptRedirectOutput(scriptName, outFile, args...)
}
