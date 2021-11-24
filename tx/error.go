package tx

import (
	"fmt"
)

func ErrNotBegun() error {
	return fmt.Errorf("Transaction not begun")
}

func ErrAlreadyBegun() error {
	return fmt.Errorf("Transaction already begun")
}

func ErrAlreadyCommitted() error {
	return fmt.Errorf("Transaction already committed")
}

func ErrAborted() error {
	return fmt.Errorf("Transaction aborted")
}
