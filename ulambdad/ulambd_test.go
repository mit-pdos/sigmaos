package ulambd

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestSimple(t *testing.T) {
	a := Attr{".yyyy", []string{"xxxx", "xxx2"}, "zzz"}

	b, err := json.Marshal(a)
	if err != nil {
		fmt.Print("Marshal error ", err)
	}
	fmt.Println("b = ", string(b))

	b1 := `"small" ["regular","large"] "unrecognized"`
	err = json.Unmarshal([]byte(b1), &a)
	if err != nil {
		fmt.Print("Unmarshal error ", err)
	}
	fmt.Println("a = ", a)
}
