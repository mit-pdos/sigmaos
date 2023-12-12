package ckpt_test

import (
	"log"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	criu "github.com/checkpoint-restore/go-criu/v7"
	"github.com/checkpoint-restore/go-criu/v7/rpc"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestExerciseProcSimple(t *testing.T) {
	ts := test.NewTstateAll(t)

	log.Printf("starting")
	chkptProc := proc.NewProc("ckpt-example", []string{"10", "1s"})
	err := ts.Spawn(chkptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(chkptProc.GetPid())
	assert.Nil(t, err)

	status, err := ts.WaitExit(chkptProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	ts.Shutdown()
}

func TestExerciseProcCkpt(t *testing.T) {
	ts := test.NewTstateAll(t)

	chkptProc := proc.NewProc("ckpt-example", []string{"30", "1s"})
	err := ts.Spawn(chkptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(chkptProc.GetPid())
	assert.Nil(t, err)

	// let her run for a sec
	time.Sleep(5 * time.Second)

	pn := sp.S3 + "~any/fkaashoek/" + chkptProc.GetPid().String() + "/"

	db.DPrintf(db.TEST, "checkpointing %q", pn)
	osPid, err := ts.Checkpoint(chkptProc, pn)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "checkpoint pid: %d", osPid)

	// ----------------------------
	// pause between chkpt and rest
	// ----------------------------
	log.Printf("taking a beat... ")
	time.Sleep(10 * time.Second)

	restProc := proc.MakeRestoreProc(chkptProc.GetPid(), pn, osPid)

	// spawn and run it
	err = ts.Spawn(restProc)
	assert.Nil(t, err)

	log.Printf("spawned")
	time.Sleep(10 * time.Second)
	log.Printf("wait exit")
	status, err := ts.WaitExit(restProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	ts.Shutdown()
}

const DIR = "ckpt"

func TestCriuDump(t *testing.T) {
	type NoNotify struct {
		criu.NoNotify
	}
	cmd := exec.Command("../bin/user/example-nonsigma", []string{"20", "1s"}...)
	//cmd.Stdout = outfile
	//cmd.Stderr = outfile
	//cmd.Stdin = os.Stdin
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
		// Setctty: true,
		//Setpgid: true,
		//Noctty: true,
	}
	err := cmd.Start()
	assert.Nil(t, err, "err %v", err)
	pid := cmd.Process.Pid

	time.Sleep(10 * time.Second)

	err = os.MkdirAll(DIR, os.ModePerm)
	assert.Nil(t, err)
	img, err := os.Open(DIR)
	assert.Nil(t, err)
	defer img.Close()

	criu := criu.MakeCriu()
	opts := &rpc.CriuOpts{}
	opts = &rpc.CriuOpts{
		Pid:         proto.Int32(int32(pid)),
		ImagesDirFd: proto.Int32(int32(img.Fd())),
		LogLevel:    proto.Int32(4),
		//TcpEstablished: proto.Bool(true),
		//Unprivileged:   proto.Bool(true),
		LogFile: proto.String("dump.log"),
	}
	err = criu.Dump(opts, NoNotify{})
	assert.Nil(t, err, "err %v", err)
}

func TestCriuRestore(t *testing.T) {
	img, err := os.Open(DIR)
	assert.Nil(t, err)
	defer img.Close()

	criu := criu.MakeCriu()
	opts := &rpc.CriuOpts{}
	opts = &rpc.CriuOpts{
		ImagesDirFd: proto.Int32(int32(img.Fd())),
		LogLevel:    proto.Int32(4),
		//TcpEstablished: proto.Bool(true),
		//Unprivileged:   proto.Bool(true),
		LogFile: proto.String("restore.log"),
	}
	err = criu.Restore(opts, nil)
	assert.Nil(t, err, "err %v", err)

	time.Sleep(10 * time.Second)

	os.RemoveAll(DIR)
	os.Remove("log.txt")
	os.Remove("out.txt")
}
