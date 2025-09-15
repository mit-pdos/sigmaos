package shell

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	sp "sigmaos/sigmap"
	"strings"

	"github.com/ergochat/readline"
)

func startREPL(ctx *shellctx.ShellContext) {
	historyFile := filepath.Join(os.TempDir(), ".myshell_history")
	ctx.History = loadHistory(historyFile)

	suggestionsEnabled := true

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[31mSigmaOS:\033[0m\033[32m" + ctx.CurrentDir + "\033[0m> ",
		HistoryFile:     historyFile,
		AutoComplete:    getCompleter(ctx, &suggestionsEnabled),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	fmt.Println("Welcome to SigmaOS Shell! Type 'exit' or 'Ctrl+C' to leave.")

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				if len(line) == 0 {
					break
				} else {
					continue
				}
			} else if err == io.EOF {
				break
			}
			fmt.Println("Error:", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "exit" {
			break
		}

		ctx.History = append(ctx.History, line)
		saveHistory(historyFile, ctx.History)

		commands := parseCommands(line)
		if len(commands) > 0 {
			executeCommands(ctx, commands)
		}

		rl.SetPrompt("\033[31mSigmaOS:\033[0m\033[32m" + ctx.CurrentDir + "\033[0m> ")
	}

	fmt.Println("\nGoodbye!")
}

func parseCommands(input string) []string {
	return strings.Split(input, "|")
}
func parseCommandWithRedirections(cmd string) (string, []string, map[string]string) {
	parts := strings.Fields(cmd)
	redirections := make(map[string]string)
	var cleanParts []string

	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "<", ">", ">>", "2>", "2>>", "&>", "&>>":
			if i+1 < len(parts) {
				redirections[parts[i]] = parts[i+1]
				i++
			}
		default:
			cleanParts = append(cleanParts, parts[i])
		}
	}

	if len(cleanParts) == 0 {
		return "", nil, redirections
	}
	return cleanParts[0], cleanParts[1:], redirections
}

func executeCommands(ctx *shellctx.ShellContext, commands []string) {
	var lastOutput io.Reader
	for i, cmd := range commands {
		cmdName, args, redirections := parseCommandWithRedirections(cmd)
		command := ctx.GetCommand(cmdName)
		if command == nil {
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmdName)
			break
		}

		stdin, stdout, stderr := setupIO(ctx, lastOutput, redirections)
		var outputBuffer bytes.Buffer
		if i < len(commands)-1 {
			stdout = &outputBuffer
		}

		success := command.Execute(ctx, args, stdin, stdout, stderr)
		if !success {
			break
		}

		if i < len(commands)-1 {
			lastOutput = bytes.NewReader(outputBuffer.Bytes())
		}
	}
}

func setupIO(ctx *shellctx.ShellContext, lastOutput io.Reader, redirections map[string]string) (io.Reader, io.Writer, io.Writer) {
	stdin := getStdin(ctx, lastOutput, redirections)
	stdout := getStdout(ctx, redirections)
	stderr := getStderr(ctx, redirections)

	if combinedFile, ok := redirections["&>"]; ok {
		stdout, stderr = getCombinedOutput(ctx, combinedFile, false)
	} else if combinedFile, ok := redirections["&>>"]; ok {
		stdout, stderr = getCombinedOutput(ctx, combinedFile, true)
	}

	return stdin, stdout, stderr
}

func getStdin(ctx *shellctx.ShellContext, lastOutput io.Reader, redirections map[string]string) io.Reader {
	if lastOutput != nil {
		return lastOutput
	}
	if inFile, ok := redirections["<"]; ok {
		content, _ := ctx.Tstate.GetFile(shellctx.FILEPATH_OFFSET + util.ResolvePath(ctx, inFile))
		return bytes.NewReader([]byte(content))
	}
	return os.Stdin
}

func getStdout(ctx *shellctx.ShellContext, redirections map[string]string) io.Writer {
	if outFile, ok := redirections[">"]; ok {
		return &fileWriter{ctx, outFile, false}
	} else if outFile, ok := redirections[">>"]; ok {
		return &fileWriter{ctx, outFile, true}
	}
	return os.Stdout
}

func getStderr(ctx *shellctx.ShellContext, redirections map[string]string) io.Writer {
	if errFile, ok := redirections["2>"]; ok {
		return &fileWriter{ctx, errFile, false}
	} else if errFile, ok := redirections["2>>"]; ok {
		return &fileWriter{ctx, errFile, true}
	}
	return os.Stderr
}

func getCombinedOutput(ctx *shellctx.ShellContext, filename string, append bool) (io.Writer, io.Writer) {
	f := &fileWriter{ctx, filename, append}
	return f, f
}

type fileWriter struct {
	ctx    *shellctx.ShellContext
	path   string
	append bool
}

func (fw *fileWriter) Write(p []byte) (n int, err error) {
	mode := sp.OWRITE
	if fw.append {
		mode = sp.OAPPEND
	}
	fw.ctx.Tstate.PutFile(shellctx.FILEPATH_OFFSET+util.ResolvePath(fw.ctx, fw.path), 0777, mode, []byte(p))
	return len(p), nil
}
