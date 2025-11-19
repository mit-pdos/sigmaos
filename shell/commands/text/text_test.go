package text

import (
	"bytes"
	"io"
	"sigmaos/shell/shellctx"
	sp "sigmaos/sigmap"
	"strings"
	"testing"
)

func TestEchoCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			name:           "Single word",
			args:           []string{"Hello"},
			expectedOutput: "Hello\n",
		},
		{
			name:           "Multiple words",
			args:           []string{"Hello", "World"},
			expectedOutput: "Hello World\n",
		},
		{
			name:           "Empty argument",
			args:           []string{},
			expectedOutput: "\n",
		},
		{
			name:           "Special characters",
			args:           []string{"Hello!", "@#$%^&*()"},
			expectedOutput: "Hello! @#$%^&*()\n",
		},
		{
			name:           "Quoted string",
			args:           []string{"\"Hello", "World\""},
			expectedOutput: "\"Hello World\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewEchoCommand()
			ctx, _ := shellctx.NewShellContext(nil) // Assuming NewShellContext can accept nil for Tstate in this case

			var stdout, stderr bytes.Buffer
			result := cmd.Execute(ctx, tt.args, nil, &stdout, &stderr)

			if !result {
				t.Errorf("Expected result true, got false")
			}

			if stdout.String() != tt.expectedOutput {
				t.Errorf("Expected output %q, got %q", tt.expectedOutput, stdout.String())
			}

			if stderr.String() != "" {
				t.Errorf("Expected no error, got %q", stderr.String())
			}
		})
	}
}
func TestGrepCommand(t *testing.T) {
	ts, err := shellctx.NewTstateAll()
	if err != nil {
		t.Fatalf("Error NewTstateAll: %v", err)
	}

	// Create a test file
	testFile := "name/test.txt"
	testContent := "Hello, World!\nThis is a test file.\nGrep is searching for patterns.\nHello again!"
	_, err = ts.PutFile(testFile, 0777, sp.OWRITE, []byte(testContent))
	if err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}

	tests := []struct {
		name           string
		args           []string
		input          string
		expectedOutput string
		expectedError  string
		expectedResult bool
	}{
		{
			name:           "Grep from file",
			args:           []string{"Hello", "/test.txt"},
			expectedOutput: "/test.txt:1:Hello, World!\n/test.txt:4:Hello again!\n",
			expectedResult: true,
		},
		{
			name:           "Grep from stdin",
			args:           []string{"test"},
			input:          testContent,
			expectedOutput: "2:This is a test file.\n",
			expectedResult: true,
		},
		{
			name:           "No matches",
			args:           []string{"nonexistent", "/test.txt"},
			expectedError:  "No matches found\n",
			expectedResult: false,
		},
		{
			name:           "Invalid regex",
			args:           []string{"[invalid", "/test.txt"},
			expectedError:  "Invalid regular expression: error parsing regexp: missing closing ]: `[invalid`\n",
			expectedResult: false,
		},
		{
			name:           "File not found",
			args:           []string{"pattern", "nonexistent.txt"},
			expectedError:  "Error reading file 'nonexistent.txt': {Err: \"file not found\" Obj: \"nonexistent.txt\" ()}\n",
			expectedResult: false,
		},
		{
			name:           "Invalid number of arguments",
			args:           []string{},
			expectedError:  "Invalid number of arguments\ngrep <pattern> [file]\n",
			expectedResult: false,
		},
		{
			name:           "Help command",
			args:           []string{"--help"},
			expectedOutput: "grep <pattern> [file]\n\nSearches for lines matching the specified pattern.\nIf no file is provided, grep reads from standard input.\n",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewGrepCommand()
			ctx, _ := shellctx.NewShellContext(ts)

			var stdin io.Reader
			if tt.input != "" {
				stdin = strings.NewReader(tt.input)
			}

			var stdout, stderr bytes.Buffer
			result := cmd.Execute(ctx, tt.args, stdin, &stdout, &stderr)

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
