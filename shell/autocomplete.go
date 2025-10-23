package shell

import (
	"path/filepath"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	sp "sigmaos/sigmap"
	"strings"

	"github.com/ergochat/readline"
)

type DynamicCompleter struct {
	ctx                *shellctx.ShellContext
	suggestionsEnabled *bool
}

// Implement the AutoCompleter interface
func (c *DynamicCompleter) Do(line []rune, pos int) ([][]rune, int) {
	input := string(line)
	words := strings.Fields(input)

	// to handle empty input or trailing space (previous word ended)
	if input == "" || input[len(input)-1] == ' ' {
		words = append(words, "")
	}

	// Case 1: Command Completion (First Word)
	if len(words) == 1 {
		var suggestions [][]rune
		for name := range c.ctx.Commands {
			if strings.HasPrefix(name, words[0]) {
				suggestions = append(suggestions, []rune(name[len(words[0]):]+" "))
			}
		}
		return suggestions, pos
	} else {

		// Case 2: File Path Completion (After First Word)
		lastWord := words[len(words)-1]
		if lastWord == "" {
			lastWord = "./"
		}

		// Resolve the file path
		dir, base := filepath.Split(lastWord)
		resolvedDir := util.ResolvePath(c.ctx, dir)
		// Fetch directory contents dynamically
		sts, err := c.ctx.Tstate.GetDir(shellctx.FILEPATH_OFFSET + resolvedDir)
		if err != nil {
			return nil, 0
		}

		// Filter file suggestions based on prefix
		files := sp.Names(sts)
		var suggestions [][]rune
		if base == "." {
			suggestions = append(suggestions, []rune("/"), []rune("./"))
		} else if base == ".." {
			suggestions = append(suggestions, []rune("/"))
		}
		for _, file := range files {
			if strings.HasPrefix(file, base) {
				// get the file name after the prefix
				var toAppend []rune
				isDir, err := c.ctx.Tstate.IsDir(shellctx.FILEPATH_OFFSET + resolvedDir + file)
				if err == nil && isDir {
					toAppend = []rune(file[len(base):] + "/")
				} else {
					toAppend = []rune(file[len(base):] + " ")
				}
				suggestions = append(suggestions, toAppend)
			}
		}

		return suggestions, len(base)
	}
}

func getCompleter(ctx *shellctx.ShellContext, suggestionsEnabled *bool) readline.AutoCompleter {
	return &DynamicCompleter{ctx: ctx, suggestionsEnabled: suggestionsEnabled}
}
