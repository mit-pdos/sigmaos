package wwwclnt

import (
	"os/exec"
	"strconv"
)

func Get(name string) ([]byte, error) {
	return exec.Command("wget", "-qO-", "http://localhost:8080/static/"+name).Output()
}

func View() ([]byte, error) {
	return exec.Command("wget", "-qO-", "http://localhost:8080/book/view/").Output()
}

func Edit(book string) ([]byte, error) {
	return exec.Command("wget", "-qO-", "http://localhost:8080/book/edit/"+book).Output()
}

func Save() ([]byte, error) {
	return exec.Command("wget", "-qO-", "--post-data", "title=Odyssey", "http://localhost:8080/book/save/Odyssey").Output()
}

func MatMul(n int) error {
	_, err := exec.Command("wget", "-qO-", "http://localhost:8080/matmul/"+strconv.Itoa(n)).Output()
	return err
}
