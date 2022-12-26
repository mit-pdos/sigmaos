package kernel

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Param struct {
	Uname    string   `yalm:"uname"`
	Services []string `yalm:"services"`
}

func ReadParam(pn string) (*Param, error) {
	param := &Param{}
	file, err := os.Open(pn)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	d := yaml.NewDecoder(file)
	if err := d.Decode(&param); err != nil {
		return nil, err
	}
	return param, nil
}
