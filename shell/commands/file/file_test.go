package file

import (
	"bytes"
	"sigmaos/shell/shellctx"
	sp "sigmaos/sigmap"
	"sort"
	"strings"
	"testing"
)

func TestCatCommand(t *testing.T) {
	ts, err := shellctx.NewTstateAll()
	if err != nil {
		t.Fatalf("Error NewTstateAll: %v", err)
	}
	// Create a test file
	file := "name/test.txt"
	_, err = ts.PutFile(file, 0777, sp.OWRITE, []byte("Hello, World!"))
	if err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}
	tests := []struct {
		name           string
		args           []string
		fileContent    string
		expectedOutput string
		expectedError  string
		expectedResult bool
	}{
		{
			name:           "Valid file",
			args:           []string{"test.txt"},
			fileContent:    "Hello, World!",
			expectedOutput: "Hello, World!",
			expectedResult: true,
		},
		{
			name:           "File not found",
			args:           []string{"nonexistent.txt"},
			expectedError:  "Error reading file: {Err: \"file not found\" Obj: \"nonexistent.txt\" ()}\n",
			expectedResult: false,
		},
		{
			name:           "Invalid number of arguments",
			args:           []string{},
			expectedError:  "Invalid number of arguments\n cat <filename>",
			expectedResult: false,
		},
		{
			name:           "Help command",
			args:           []string{"--help"},
			expectedOutput: "cat <filename>\n",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCatCommand()
			ctx, _ := shellctx.NewShellContext(ts)

			var stdout, stderr bytes.Buffer
			result := cmd.Execute(ctx, tt.args, nil, &stdout, &stderr)

			if result != tt.expectedResult {
				t.Errorf("Expected result %v, got %v", tt.expectedResult, result)
			}

			if stdout.String() != tt.expectedOutput {
				t.Errorf("Expected output %q, got %q", tt.expectedOutput, stdout.String())
			}

			if stderr.String() != tt.expectedError {
				t.Errorf("Expected error %q, got %q", tt.expectedError, stderr.String())
			}
		})
	}
}

func TestCdCommand(t *testing.T) {
	ts, err := shellctx.NewTstateAll()
	if err != nil {
		t.Fatalf("Error NewTstateAll: %v", err)
	}

	// Create test directories
	dirs := []string{"name/testdir", "name/testdir/subdir"}
	for _, dir := range dirs {
		err = ts.MkDir(dir, 0777)
		if err != nil {
			t.Fatalf("Error creating test directory %s: %v", dir, err)
		}
	}

	tests := []struct {
		name           string
		args           []string
		initialDir     string
		expectedDir    string
		expectedOutput string
		expectedError  string
		expectedResult bool
	}{
		{
			name:           "Valid directory",
			args:           []string{"testdir"},
			initialDir:     "/",
			expectedDir:    "/testdir/",
			expectedResult: true,
		},
		{
			name:           "Valid subdirectory",
			args:           []string{"subdir"},
			initialDir:     "/testdir",
			expectedDir:    "/testdir/subdir/",
			expectedResult: true,
		},
		{
			name:           "Directory not found",
			args:           []string{"nonexistent"},
			initialDir:     "/",
			expectedDir:    "/",
			expectedError:  "Error changing to directory /nonexistent: {Err: \"file not found\" Obj: \"nonexistent\" ()}\n",
			expectedResult: false,
		},
		{
			name:           "Invalid number of arguments",
			args:           []string{},
			initialDir:     "/",
			expectedDir:    "/",
			expectedError:  "Invalid number of arguments\n cd <directory>",
			expectedResult: false,
		},
		{
			name:           "Help command",
			args:           []string{"--help"},
			initialDir:     "/",
			expectedDir:    "/",
			expectedOutput: "cd <directory>\n",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCdCommand()
			ctx, _ := shellctx.NewShellContext(ts)
			ctx.CurrentDir = tt.initialDir

			var stdout, stderr bytes.Buffer
			result := cmd.Execute(ctx, tt.args, nil, &stdout, &stderr)

			if result != tt.expectedResult {
				t.Errorf("Expected result %v, got %v", tt.expectedResult, result)
			}

			if ctx.CurrentDir != tt.expectedDir {
				t.Errorf("Expected directory %q, got %q", tt.expectedDir, ctx.CurrentDir)
			}

			if stdout.String() != tt.expectedOutput {
				t.Errorf("Expected output %q, got %q", tt.expectedOutput, stdout.String())
			}

			if stderr.String() != tt.expectedError {
				t.Errorf("Expected error %q, got %q", tt.expectedError, stderr.String())
			}
		})
	}
}

func TestCpCommand(t *testing.T) {
	ts, err := shellctx.NewTstateAll()
	if err != nil {
		t.Fatalf("Error NewTstateAll: %v", err)
	}

	// Create test files and directories
	srcFile := "name/test.txt"
	srcDir := "name/testdir"

	_, err = ts.PutFile(srcFile, 0777, sp.OWRITE, []byte("Hello, World!"))
	if err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}

	err = ts.MkDir(srcDir, 0777)
	if err != nil {
		t.Fatalf("Error creating test directory: %v", err)
	}

	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectedError  string
		expectedResult bool
	}{
		{
			name:           "Copy valid file",
			args:           []string{"/test.txt", "/test_copy.txt"},
			expectedOutput: "File copied successfully from /test.txt to /test_copy.txt\n",
			expectedResult: true,
		},
		// {
		// 	name:           "Copy valid directory",
		// 	args:           []string{"/testdir", "/testdir_copy"},
		// 	expectedOutput: "Directory copied successfully from /testdir to /testdir_copy\n",
		// 	expectedResult: true,
		// },
		{
			name:           "Source file not found",
			args:           []string{"/nonexistent.txt", "/test_copy.txt"},
			expectedError:  "Error checking source: {Err: \"file not found\" Obj: \"nonexistent.txt\" ()}\n",
			expectedResult: false,
		},
		{
			name:           "Invalid number of arguments",
			args:           []string{},
			expectedError:  "Invalid number of arguments\n cp <source> <destination>",
			expectedResult: false,
		},
		{
			name:           "Help command",
			args:           []string{"--help"},
			expectedOutput: "cp <source> <destination>\n",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCpCommand()
			ctx, _ := shellctx.NewShellContext(ts)

			var stdout, stderr bytes.Buffer
			result := cmd.Execute(ctx, tt.args, nil, &stdout, &stderr)

			if result != tt.expectedResult {
				t.Errorf("Expected result %v, got %v", tt.expectedResult, result)
			}

			if stdout.String() != tt.expectedOutput {
				t.Errorf("Expected output %q, got %q", tt.expectedOutput, stdout.String())
			}

			if stderr.String() != tt.expectedError {
				t.Errorf("Expected error %q, got %q", tt.expectedError, stderr.String())
			}

			// Verify file/directory copy
			if tt.expectedResult && len(tt.args) == 2 {
				srcPath := "name" + tt.args[0]
				dstPath := "name" + tt.args[1]

				// Check if destination exists
				_, dstErr := ts.Stat(dstPath)
				if dstErr != nil {
					t.Errorf("Destination %s does not exist after copy: %v", dstPath, dstErr)
				}

				// For file copy, check content
				if tt.name == "Copy valid file" {
					srcContent, _ := ts.GetFile(srcPath)
					dstContent, readErr := ts.GetFile(dstPath)
					if readErr != nil {
						t.Errorf("Error reading copied file: %v", readErr)
					}
					if string(srcContent) != string(dstContent) {
						t.Errorf("Copied file content mismatch. Expected %q, got %q", string(srcContent), string(dstContent))
					}
				}

				// For directory copy, check if it's a directory
				if tt.name == "Copy valid directory" {
					if isDir, err := ts.IsDir(dstPath); !isDir || err != nil {
						t.Errorf("Copied directory %s is not a directory", dstPath)
					}
				}
			}
		})
	}
}
func TestLsCommand(t *testing.T) {
	ts, err := shellctx.NewTstateAll()
	if err != nil {
		t.Fatalf("Error NewTstateAll: %v", err)
	}

	// Create test files and directories
	testDir := "name/testdir"
	testFiles := []string{"file1.txt", "file2.txt"}
	testSubDir := "subdir"

	err = ts.MkDir(testDir, 0777)
	if err != nil {
		t.Fatalf("Error creating test directory: %v", err)
	}

	for _, file := range testFiles {
		_, err = ts.PutFile(testDir+"/"+file, 0777, sp.OWRITE, []byte("Test content"))
		if err != nil {
			t.Fatalf("Error creating test file %s: %v", file, err)
		}
	}

	err = ts.MkDir(testDir+"/"+testSubDir, 0777)
	if err != nil {
		t.Fatalf("Error creating test subdirectory: %v", err)
	}

	tests := []struct {
		name           string
		args           []string
		currentDir     string
		expectedOutput []string
		expectedError  string
		expectedResult bool
	}{
		{
			name:           "List current directory",
			args:           []string{},
			currentDir:     "/testdir",
			expectedOutput: append(testFiles, testSubDir),
			expectedResult: true,
		},
		{
			name:           "List specific directory",
			args:           []string{"/testdir"},
			currentDir:     "/",
			expectedOutput: append(testFiles, testSubDir),
			expectedResult: true,
		},
		{
			name:           "List non-existent directory",
			args:           []string{"/nonexistent"},
			currentDir:     "/",
			expectedError:  "Error listing directory /nonexistent: {Err: \"file not found\" Obj: \"nonexistent\" ()}\n",
			expectedResult: false,
		},
		{
			name:           "Invalid number of arguments",
			args:           []string{"/dir1", "/dir2"},
			currentDir:     "/",
			expectedError:  "Invalid number of arguments\n ls [directory]",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewLsCommand()
			ctx, _ := shellctx.NewShellContext(ts)
			ctx.CurrentDir = tt.currentDir

			var stdout, stderr bytes.Buffer
			result := cmd.Execute(ctx, tt.args, nil, &stdout, &stderr)

			if result != tt.expectedResult {
				t.Errorf("Expected result %v, got %v", tt.expectedResult, result)
			}

			if tt.expectedResult {
				output := strings.Split(strings.TrimSpace(stdout.String()), "\n")
				sort.Strings(output)
				sort.Strings(tt.expectedOutput)
				if !stringSlicesEqual(output, tt.expectedOutput) {
					t.Errorf("Expected output %v, got %v", tt.expectedOutput, output)
				}
			}

			if stderr.String() != tt.expectedError {
				t.Errorf("Expected error %q, got %q", tt.expectedError, stderr.String())
			}

			// Verify the listed directory contents
			if tt.expectedResult {
				var dirToCheck string
				if len(tt.args) > 0 {
					dirToCheck = "name" + tt.args[0]
				} else {
					dirToCheck = "name" + tt.currentDir
				}

				sts, err := ts.GetDir(dirToCheck)
				if err != nil {
					t.Errorf("Error getting directory contents: %v", err)
				}

				dirContents := sp.Names(sts)
				sort.Strings(dirContents)
				if !stringSlicesEqual(dirContents, tt.expectedOutput) {
					t.Errorf("Actual directory contents %v don't match expected %v", dirContents, tt.expectedOutput)
				}
			}
		})
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
