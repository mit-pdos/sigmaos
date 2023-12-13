package example_test

import (
	// Go imports:

	"log"
	"os"
	gopath "path"
	"strings"
	"testing"
	"time"

	// External imports:

	"github.com/stretchr/testify/assert"

	// SigmaOS imports:

	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestExerciseNamed(t *testing.T) {
	dir := sp.NAMED
	ts := test.NewTstatePath(t, dir)

	sts, err := ts.GetDir(dir)
	assert.Nil(t, err)

	log.Printf("%v: %v\n", dir, sp.Names(sts))
	log.Printf("here")

	// Your code here

	// create test file
	// diff between create and putfile?
	file_name := gopath.Join(dir, "file")
	fd, err := ts.Create(file_name, 0777, sp.OWRITE)
	assert.Equal(t, nil, err)

	// write content
	d := []byte("hi I wrote this")
	_, err = ts.Write(fd, d)
	assert.Equal(t, nil, err)

	// close
	err = ts.Close(fd)
	assert.Equal(t, nil, err)

	// list dir - test file is there
	sts, _ = ts.GetDir(dir)
	log.Printf("%v: %v\n", dir, sp.Names(sts))

	// open file - test contents are correct
	read_contents, _ := ts.GetFile(file_name)
	assert.Equal(t, "hi I wrote this", string(read_contents))

	// remove file?

	ts.Shutdown()
}

func TestExerciseS3(t *testing.T) {
	// ts := test.NewTstateAll(t)
	dir := sp.S3
	ts := test.NewTstatePath(t, dir)

	testDir := sp.S3 + "~any/hmngtestbucket/"

	ts.MkDir(testDir, 0777)
	log.Printf("created dir: %v\n", testDir)

	filePath := testDir + "ztest.txt"
	dstFd, err := ts.Create(filePath, 0777, sp.OWRITE)
	if err != nil {
		log.Printf("error: %s", err.Error())
	}
	assert.Nil(t, err)

	log.Printf("created file\n")

	// read local content
	fileContents, err := os.ReadFile("./test.txt")
	if err != nil {
		log.Printf("error: %s", err.Error())
	}
	assert.Nil(t, err)
	log.Printf("read local file\n")

	// write content to s3
	_, err = ts.Write(dstFd, fileContents)
	if err != nil {
		log.Printf("error: %s", err.Error())
	}
	assert.Nil(t, err)
	log.Printf("wrote remote file\n")

	// close
	err = ts.Close(dstFd)
	if err != nil {
		log.Printf("error: %s", err.Error())
	}
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestExerciseProc(t *testing.T) {
	ts := test.NewTstateAll(t)

	log.Printf("starting")
	chkptProc := proc.NewProc("example", []string{})

	err := ts.Spawn(chkptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(chkptProc.GetPid())
	log.Printf("started")
	assert.Nil(t, err)

	log.Printf("checkpointing")
	chkptLoc, osPid, err := ts.Checkpoint(chkptProc)
	assert.Nil(t, err)
	log.Printf("checkpoint location: %s", chkptLoc)
	log.Printf("checkpoint pid: %d", osPid)

	// ----------------------------
	// pause between chkpt and rest
	// ----------------------------
	log.Printf("taking a beat... ")
	time.Sleep(5 * time.Second)

	log.Printf("restoring")
	chkptLocList := strings.Split(chkptLoc, "/")
	sigmaPid := chkptLocList[len(chkptLocList)-1]
	restProc := proc.MakeRestoreProc(chkptLoc, osPid, sigmaPid)

	// spawn and run it
	err = ts.Spawn(restProc)
	assert.Nil(t, err)
	err = ts.WaitStart(restProc.GetPid())
	assert.Nil(t, err)
	log.Printf("started")

	status, err := ts.WaitExit(restProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	ts.Shutdown()
}

func TestExerciseRestore(t *testing.T) {
	ts := test.NewTstateAll(t)

	log.Printf("starting")
	// gotten from returned values from checkpointing
	osPid := 16
	chkptLoc := "name/s3/~any/sigmaoscheckpoint/example-2fdda82cb4aef13a/"

	// make restore proc
	// TODO make this be perf?
	chkptLocList := strings.Split(chkptLoc, "/")
	sigmaPid := chkptLocList[len(chkptLocList)-2]
	p := proc.MakeRestoreProc(chkptLoc, osPid, sigmaPid)

	// log.Printf("procEnvProto: %+v", p.ProcEnvProto)

	// spawn and run it
	err := ts.Spawn(p)
	assert.Nil(t, err)
	// err = ts.WaitStart(p.GetPid())
	// log.Printf("started")
	// assert.Nil(t, err)

	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	ts.Shutdown()
}

func TestExerciseOut(t *testing.T) {
	ts := test.NewTstateAll(t)
	// Your code here

	testDir := sp.S3 + "~any/hmngtestbucket/"
	fileName := testDir + "example-out.txt"

	// fileName := "name/s3/~any/sigmaoscheckpoint/example-800a8b07b7aad4df/restore.log"

	_, err := ts.Open(fileName, sp.OREAD)
	if err != nil {
		log.Fatalf("error opening: %s", err.Error())
	}

	// open file - test contents are correct
	fileContents, err := ts.GetFile(fileName)
	if err != nil {
		log.Printf("error: %s", err.Error())
	} else {
		log.Printf("file contents: %s", fileContents)
	}

	// err = os.WriteFile("dump.log", fileContents, 0777)
	// if err != nil {
	// 	log.Fatalf("error writing: %s", err.Error())
	// }

	ts.Shutdown()
}

func TestExerciseParallel(t *testing.T) {
	ts := test.NewTstateAll(t)

	// Your code here

	ts.Shutdown()
}
