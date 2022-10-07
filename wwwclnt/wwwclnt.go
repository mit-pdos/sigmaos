package wwwclnt

import (
	"io/ioutil"
	"net/http"
	"strconv"
)

const (
	BASE_URL = "http://localhost:8080"
	STATIC   = "static/"
	MATMUL   = "matmul/"
	BOOK     = "book/"
	VIEW     = BOOK + "view/"
	EDIT     = BOOK + "edit/"
	SAVE     = BOOK + "save/"
)

func get(url, path string) ([]byte, error) {
	resp, err := http.Get(url + "/" + path)
	if err != nil {
		return []byte{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}

func post(url, path string, vals map[string][]string) ([]byte, error) {
	resp, err := http.PostForm(url+"/"+path, vals)
	if err != nil {
		return []byte{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}

func Get(name string) ([]byte, error) {
	return get(BASE_URL, STATIC+name)
}

func View() ([]byte, error) {
	return get(BASE_URL, VIEW)
}

func Edit(book string) ([]byte, error) {
	return get(BASE_URL, EDIT+book)
}

func Save() ([]byte, error) {
	vals := map[string][]string{
		"title": []string{"Odyssey"},
	}
	return post(BASE_URL, SAVE+"Odyssey", vals)
}

func MatMul(n int) error {
	_, err := get(BASE_URL, MATMUL+strconv.Itoa(n))
	return err
}
