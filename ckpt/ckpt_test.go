package ckpt_test

import (
	// Go imports:

	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	// External imports:

	"github.com/stretchr/testify/assert"

	criu "github.com/checkpoint-restore/go-criu/v7"
	"github.com/checkpoint-restore/go-criu/v7/rpc"
	"google.golang.org/protobuf/proto"

	// SigmaOS imports:

	"sigmaos/proc"
	// sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestExerciseProc(t *testing.T) {
	ts := test.NewTstateAll(t)

	log.Printf("starting")
	chkptProc := proc.NewProc("example", []string{"10", "1s"})
	err := ts.Spawn(chkptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(chkptProc.GetPid())
	assert.Nil(t, err)

	log.Printf("started")

	status, err := ts.WaitExit(chkptProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	ts.Shutdown()
}

func TestExerciseProcCkpt(t *testing.T) {
	ts := test.NewTstateAll(t)

	log.Printf("starting")
	chkptProc := proc.NewProc("example", []string{"100", "1s"})
	err := ts.Spawn(chkptProc)
	assert.Nil(t, err)
	//err = ts.WaitStart(chkptProc.GetPid())
	//assert.Nil(t, err)

	//log.Printf("started")

	// let her run for a sec
	time.Sleep(5 * time.Second)

	log.Printf("checkpointing")
	chkptLoc, osPid, err := ts.Checkpoint(chkptProc)
	assert.Nil(t, err)
	log.Printf("checkpoint location: %s", chkptLoc)
	log.Printf("checkpoint pid: %d", osPid)

	// ----------------------------
	// pause between chkpt and rest
	// ----------------------------
	log.Printf("taking a beat... ")
	time.Sleep(1000 * time.Second)

	// log.Printf("restoring")

	// chkptLocList := strings.Split(chkptLoc, "/")
	// sigmaPid := chkptLocList[len(chkptLocList)-2]
	// restProc := proc.MakeRestoreProc(chkptLoc, osPid, sigmaPid)

	// // spawn and run it
	// err = ts.Spawn(restProc)
	// assert.Nil(t, err)

	// status, err := ts.WaitExit(restProc.GetPid())
	// assert.Nil(t, err)
	// assert.True(t, status.IsStatusOK())

	ts.Shutdown()
}

func TestExerciseRestore(t *testing.T) {
	ts := test.NewTstateAll(t)

	log.Printf("starting")
	// gotten from returned values from checkpointing
	osPid := 18
	chkptLoc := "name/s3/~any/fkaashoek/example-3592d035307de6d7/"

	// make restore proc
	// TODO make this be perf?
	chkptLocList := strings.Split(chkptLoc, "/")
	sigmaPid := chkptLocList[len(chkptLocList)-2]
	p := proc.MakeRestoreProc(chkptLoc, osPid, sigmaPid)

	// log.Printf("procEnvProto: %+v", p.ProcEnvProto)

	// spawn and run it
	err := ts.Spawn(p)
	assert.Nil(t, err)
	err = ts.WaitStart(p.GetPid())
	log.Printf("started")
	assert.Nil(t, err)

	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	ts.Shutdown()
}

const DIR = "ckpt"

func TestCriuDump(t *testing.T) {
	type NoNotify struct {
		criu.NoNotify
	}

	outfile, err := os.Create("./out.txt")
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("../bin/user/example-nonsigma", []string{"10", "1s"}...)
	cmd.Stdout = outfile
	cmd.Stderr = outfile
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	err = cmd.Start()
	assert.Nil(t, err)
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
		Pid:            proto.Int32(int32(pid)),
		ImagesDirFd:    proto.Int32(int32(img.Fd())),
		LogLevel:       proto.Int32(4),
		TcpEstablished: proto.Bool(true),
		Unprivileged:   proto.Bool(true),
		ShellJob:       proto.Bool(true),
		LogFile:        proto.String("dump.log"),
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
		ImagesDirFd:    proto.Int32(int32(img.Fd())),
		LogLevel:       proto.Int32(4),
		TcpEstablished: proto.Bool(true),
		Unprivileged:   proto.Bool(true),
		ShellJob:       proto.Bool(true),
		LogFile:        proto.String("restore.log"),
	}
	err = criu.Restore(opts, nil)
	assert.Nil(t, err, "err %v", err)

	time.Sleep(10 * time.Second)

	os.RemoveAll(DIR)
	os.Remove("log.txt")
	os.Remove("out.txt")
}
