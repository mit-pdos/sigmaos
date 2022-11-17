package hotel

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

func webRequest(url string, vals url.Values) ([]byte, error) {
	resp, err := http.PostForm(url, vals)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v %s", resp.StatusCode, body)
	}
	return body, nil
}

func WebLogin(u, p string) (string, error) {
	vals := url.Values{}
	vals.Set("username", u)
	vals.Set("password", p)
	body, err := webRequest("http://localhost:8090/user", vals)
	if err != nil {
		return "", err
	}
	repl := make(map[string]interface{})
	err = json.Unmarshal(body, &repl)
	if err != nil {
		return "", err
	}
	return repl["message"].(string), nil
}

func WebSearch(inDate, outDate string, lat, lon float64) error {
	vals := url.Values{}
	vals.Set("inDate", inDate)
	vals.Set("outDate", outDate)
	vals.Add("lat", fmt.Sprintf("%f", lat))
	vals.Add("lon", fmt.Sprintf("%f", lon))
	body, err := webRequest("http://localhost:8090/hotels", vals)
	if err != nil {
		return err
	}
	log.Printf("%v", string(body))
	return nil
}
