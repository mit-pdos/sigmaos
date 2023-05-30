package tutorial_test

import (
	"flag"
	"github.com/stretchr/testify/assert"
	gopath "path"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"testing"
)

var pathname string

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
}

func ls(t *testing.T, ts *test.Tstate, pathname string) {
	dir, err := ts.GetDir(pathname)
	assert.Equal(t, nil, err)
	directory := ""
	for _, file := range dir {
		directory += file.GetName()
		directory += " "
	}
	db.DPrintf(db.TEST, "List of every file in the %v directory: %v", pathname, directory)
}

func TestStartStop(t *testing.T) {
	ts := test.MakeTstateAll(t)
	db.DPrintf(db.TEST, "Successfully started SigmaOS")
	ts.Shutdown()
}

func TestExerciseOneShort(t *testing.T) {
	ts := test.MakeTstateAll(t)
	db.DPrintf(db.TEST, "Successfully started SigmaOS")
	fn := gopath.Join(pathname, "fileToTest")
	db.DPrintf(db.TEST, "Directory: %v ; Filepath: %v", pathname, fn)

	// Create a file in named and write the data to the file
	dataIn := []byte("This is some test data!!!")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, dataIn)
	assert.Equal(t, nil, err)
	db.DPrintf(db.TEST, "Successfully created a file in named")

	// List directory
	ls(t, ts, pathname)

	// Read contents
	dataOut, err := ts.GetFile(fn)
	// Ensure contents is equal to original
	assert.Equal(t, nil, err)
	assert.Equal(t, dataIn, dataOut)
	db.DPrintf(db.TEST, "Data written to file: %v", string(dataOut))

	ts.Shutdown()
}

func TestExerciseOneFull(t *testing.T) {
	ts := test.MakeTstateAll(t)
	db.DPrintf(db.TEST, "Successfully started SigmaOS")
	fn := gopath.Join(pathname, "fileToTest2")
	db.DPrintf(db.TEST, "Directory: %v ; Filepath: %v", pathname, fn)

	// Create a file
	writer, err := ts.CreateWriter(fn, 0777, sp.OWRITE)
	assert.Equal(t, nil, err)
	db.DPrintf(db.TEST, "Successfully created a file")

	// Write to the file
	dataIn := []byte("This is some test data!!! v2")
	_, err = writer.Write(dataIn)
	assert.Equal(t, nil, err)
	db.DPrintf(db.TEST, "Successfully wrote to a file")

	// Close the file
	err = writer.Close()
	assert.Equal(t, nil, err)
	db.DPrintf(db.TEST, "Successfully closed a file writer")

	// List directory
	ls(t, ts, pathname)

	// Open file
	reader, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)
	db.DPrintf(db.TEST, "Successfully opened a file reader")

	// Read Contents
	dataOut, err := reader.GetData()
	// Ensure contents is equal to the original data
	assert.Equal(t, nil, err)
	assert.Equal(t, dataIn, dataOut)
	db.DPrintf(db.TEST, "Data written to file: %v", string(dataOut))

	// Close file
	err = reader.Close()
	assert.Equal(t, nil, err)
	db.DPrintf(db.TEST, "Successfully closed a file reader")

	ts.Shutdown()
}

func TestExerciseThree(t *testing.T) {
	ts := test.MakeTstateAll(t)
	db.DPrintf(db.TEST, "Successfully started SigmaOS")

	// Create a file in named and write the data to the file
	fn := gopath.Join(pathname, "fileToTest")
	dataIn := []byte("This is some test data!!!")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, dataIn)
	assert.Equal(t, nil, err)
	db.DPrintf(db.TEST, "Successfully created a file in named")

	// Broken =(
	dataIn2 := []byte("Read me on the server please!")
	err = ts.ExerciseThree(pathname, dataIn2)
	assert.Equal(t, nil, err)
	db.DPrintf(db.TEST, "Successfully ran Exercise Three")

	// Read the file
	dataOut, err := ts.GetFile(fn)
	// Ensure contents is equal to original
	assert.Equal(t, nil, err)
	assert.Equal(t, dataIn, dataOut)
	db.DPrintf(db.TEST, "Data written to file: %v", string(dataOut))

	ts.Shutdown()
}
