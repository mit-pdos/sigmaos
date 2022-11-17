package hotel

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func WebLogin(u, p string) (string, error) {
	vals := map[string][]string{
		"username": []string{u},
		"password": []string{p},
	}
	resp, err := http.PostForm("http://localhost:8090/user", vals)
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%v %s", resp.StatusCode, body)
	}
	repl := make(map[string]interface{})
	err = json.Unmarshal(body, &repl)
	if err != nil {
		return "", err
	}
	return repl["message"].(string), nil
}

func WebSearch(inDate, outDate string, lat, lon float64) error {
	client := &http.Client{}

	data := url.Values{}
	data.Set("inDate", inDate)
	data.Set("outDate", outDate)
	data.Add("lat", fmt.Sprintf("%f", lat))
	data.Add("lon", fmt.Sprintf("%f", lon))
	encodedData := data.Encode()

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8090/hotels", strings.NewReader(encodedData))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%v %s", resp.StatusCode, body)
	}
	log.Printf("%v", string(body))
	return nil
}
