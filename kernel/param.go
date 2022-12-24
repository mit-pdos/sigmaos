package kernel

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Param struct {
	Path  string `yalm:"path"`
	Uname string `yalm:"uname"`
	Realm string `yalm:"realm"`
}

func readParam(pn string) (*Param, error) {
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
