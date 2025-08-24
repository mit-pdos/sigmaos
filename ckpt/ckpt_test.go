package ckpt_test

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"os/exec"
	"sigmaos/apps/hotel"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	rd "sigmaos/util/rand"
)

const (
	NPAGES  = "1000"
	PROGRAM = "ckpt-proc"
	GEO     = "hotel-geod"
	RUN     = 5
)

// Geo constants
const (
	DEF_GEO_N_IDX         = 1000
	DEF_GEO_SEARCH_RADIUS = 10
	DEF_GEO_N_RESULTS     = 5
)

func listAllPIDs() (string, error) {
	cmd := exec.Command("pgrep", "-a", "") // "-a" lists PIDs with process names

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error running pgrep: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
func straceProcess(pid string, index int) (*exec.Cmd, error) {
	// Define output file path using a counter
	outputFile := fmt.Sprintf("/home/freddietang/sigmaos/lazy_%d.txt", index)

	// Create the output file
	file, err := os.Create(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file %s: %w", outputFile, err)
	}
	//defer file.Close()

	// Run `strace -f -p <pid>` and direct output to the file
	cmd := exec.Command("sudo", "strace", "-o", outputFile, "-tt", "-f", "-p", pid)
	cmd.Stdout = file
	cmd.Stderr = file

	// Start strace asynchronously
	return cmd, cmd.Start()
}
func trace(name string) ([]*exec.Cmd, error) {
	cmd := exec.Command("ps", "aux")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		fmt.Println("Error running ps aux:", err)
		return nil, fmt.Errorf("CMD did not run")
	}
	cnter := 0
	var cmds []*exec.Cmd
	// Split output into lines and filter for "ckpt-proc"
	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		if strings.Contains(line, name) {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				db.DPrintf(db.TEST, "found lazypages %v", fields[1])
				cmd, err := straceProcess(fields[1], cnter)
				if err != nil {
					return nil, err
				}
				cmds = append(cmds, cmd)
				cnter += 1
				//	return fields[1], nil // PID is the second column
			}
		}
	}
	return cmds, nil
}
func listAllProcesses() (string, error) {
	cmd := exec.Command("ps", "aux")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		fmt.Println("Error running ps aux:", err)
		return "", fmt.Errorf("CMD did not run")
	}

	// Split output into lines and filter for "ckpt-proc"
	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		if strings.Contains(line, "ckpt-proc") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				return fields[1], nil // PID is the second column
			}
		}
	}
	return "", fmt.Errorf("BADD")
}
func TestSpawnCkptProc(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	pid := sp.GenPid(PROGRAM)
	pn := sp.UX + "~any/" + pid.String() + "/"

	ckptProc := proc.NewProcPid(pid, PROGRAM, []string{strconv.Itoa(RUN), NPAGES, pn})
	err = ts.Spawn(ckptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(ckptProc.GetPid())

	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait until proc %v has checkpointed itself", ckptProc.GetPid())
	db.DPrintf(db.TEST, "pid %v", pid)
	status, err := ts.WaitExit(ckptProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusErr())

	pid = sp.GenPid("ckpt-proc-copy")

	db.DPrintf(db.TEST, "Spawn from checkpoint %v", pid)

	restProc := proc.NewProcFromCheckpoint(pid, PROGRAM, pn)
	restProc.Args = []string{"5", "1000", "name/ux/~any/ckpt-proc-7f03b979fbb54ec6/"}
	err = ts.Spawn(restProc)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait until start %v %v", pid, restProc)

	err = ts.WaitStart(restProc.GetPid())
	assert.Nil(t, err)
	// ret, err := listAllPIDs()
	// assert.Nil(t, err)
	// db.DPrintf(db.ALWAYS, ret)
	// cmd2 := exec.Command("ps", "aux")
	// var out bytes.Buffer
	// cmd2.Stdout = &out
	// if err := cmd2.Run(); err != nil {
	// 	fmt.Println("Error running ps aux:", err)
	// 	return
	// }

	// // Split output into lines and filter for "ckpt-proc"
	// lines := strings.Split(out.String(), "\n")
	// for _, line := range lines {
	// 	if strings.Contains(line, "lazypages") {
	// 		db.DPrintf(db.ALWAYS, "line: %s", line) // Print as an integer

	// 	}
	// }
	// linuxpid, err := listAllProcesses()
	assert.Nil(t, err)

	s, err := ts.WaitExit(restProc.GetPid())
	assert.Nil(t, err, "Err waitexit %v status: %v", err, s)
	db.DPrintf(db.TEST, "Started %v", restProc.GetPid())
	time.Sleep(2000 * time.Millisecond)
	ts.Shutdown()
}

func TestSpawnCkptGeo(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	pid := sp.GenPid(GEO)
	pn := sp.UX + "~any/" + pid.String() + "/"

	job := rd.String(8)
	err = hotel.InitHotelFs(ts.FsLib, job)
	//ts.MkDir(filepath.Join("name/hotel/geo", job), 0777)

	assert.Nil(t, err)

	//db.DPrintf(db.TEST, "Spawn proc %v %v", job, pn)

	//ckptProc := proc.NewProcPid(pid, GEO, []string{job, pn, "1000", "10", "20"})
	ckptProc := proc.NewProcPid(pid, GEO, []string{job, pn, "1000", "10", "20"})
	db.DPrintf(db.TEST, "Spawn proc %v %v", job, pn)
	err = ts.Spawn(ckptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(ckptProc.GetPid())
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait until proc %v has checkpointed itself", ckptProc.GetPid())

	status, err := ts.WaitExit(ckptProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusErr())
	//time.Sleep(100 * time.Millisecond)

	pid = sp.GenPid(GEO + "-copy")

	db.DPrintf(db.TEST, "Spawn from checkpoint %v", pid)

	restProc := proc.NewProcFromCheckpoint(pid, GEO, pn)
	err = ts.Spawn(restProc)
	assert.Nil(t, err)

	//db.DPrintf(db.TEST, "Wait until start %v", pid)
	//db.DPrintf(db.TEST, "Wait until start %v", pid)

	err = ts.WaitStart(restProc.GetPid())
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "Started %v", pid)

	time.Sleep(1000 * time.Millisecond)
	status, err = ts.WaitExit(restProc.GetPid())
	db.DPrintf(db.TEST, "exited %v", status)
	db.DPrintf(db.TEST, "Spawn from checkpoint")
	pid = sp.GenPid(GEO + "-copy2")
	restProc2 := proc.NewProcFromCheckpoint(pid, GEO+"-copy2", pn)
	err = ts.Spawn(restProc2)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait until start again %v", pid)

	err = ts.WaitStart(restProc2.GetPid())
	db.DPrintf(db.TEST, "Started %v", pid)
	time.Sleep(2000 * time.Millisecond)
	assert.Nil(t, err)
	ts.Shutdown()
}
