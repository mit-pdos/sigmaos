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

	testDir := sp.S3 + "~local/fkaashoek/"

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
	chkptProc := proc.NewProc("example", []string{"1", "1s"})
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
	chkptProc := proc.NewProc("example", []string{"10", "1s"})
	err := ts.Spawn(chkptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(chkptProc.GetPid())
	assert.Nil(t, err)

	log.Printf("started")

	// let her run for a sec
	time.Sleep(3 * time.Second)

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
	sigmaPid := chkptLocList[len(chkptLocList)-2]
	restProc := proc.MakeRestoreProc(chkptLoc, osPid, sigmaPid)

	// spawn and run it
	err = ts.Spawn(restProc)
	assert.Nil(t, err)

	status, err := ts.WaitExit(restProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

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

func TestExerciseOut(t *testing.T) {
	ts := test.NewTstateAll(t)
	// Your code here

	// testDir := sp.S3 + "~any/hmngtestbucket/"
	// fileName := testDir + "example-out.txt"

	// fileName := "name/s3/~any/fkaashoek/example-558436b294fad070/mountpoints-12.img"
	// w/ everything + tramp:
	// fileName := "name/s3/~any/fkaashoek/example-3d4aaf938bf06900/fdinfo-2.img"
	// w/ everything, no tramp:
	// fileName := "name/s3/~any/fkaashoek/example-8cbcc31036d18cfc/fdinfo-2.img"
	// w/ fslib, but no pathclnt
	// fileName := "name/s3/~any/fkaashoek/example-3dbdf5ba54df88ab/fdinfo-2.img"

	// w/ nilled out sc
	// fileName := "name/s3/~any/fkaashoek/example-cd3bca6a83315115/mm-18.img"

	fileName := "name/s3/~any/fkaashoek/example-cc3e923ed91b0af5/mm-18.img"

	// fileName := "name/s3/~any/fkaashoek/example-800a8b07b7aad4df/restore.log"

	// chkptDir := "name/s3/~any/hmngtestbucket/"

	// sts, _ := ts.GetDir(chkptDir)
	// log.Printf("%v: %v\n", chkptDir, sp.Names(sts))

	// fileName := chkptDir + "dump.log"
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

	err = os.WriteFile("fdinfo.img", fileContents, 0777)
	if err != nil {
		log.Fatalf("error writing: %s", err.Error())
	}

	ts.Shutdown()
}

func TestExerciseParallel(t *testing.T) {
	ts := test.NewTstateAll(t)

	// Your code here

	ts.Shutdown()
}
