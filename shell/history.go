package shell

import (
	"fmt"
	"os"
)

func loadHistory(file string) []string {
	history := []string{}
	if f, err := os.Open(file); err == nil {
		defer f.Close()
		var line string
		for {
			_, err := fmt.Fscanln(f, &line)
			if err != nil {
				break
			}
			history = append(history, line)
		}
	}
	return history
}

func saveHistory(file string, history []string) {
	f, err := os.Create(file)
	if err != nil {
		fmt.Printf("Error saving history: %v\n", err)
		return
	}
	defer f.Close()

	for _, entry := range history {
		fmt.Fprintln(f, entry)
	}
}
