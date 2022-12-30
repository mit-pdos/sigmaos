package yaml

import (
	"os"

	"gopkg.in/yaml.v3"
)

func ReadYaml(pn string, v interface{}) error {
	file, err := os.Open(pn)
	if err != nil {
		return err
	}
	defer file.Close()
	d := yaml.NewDecoder(file)
	if err := d.Decode(v); err != nil {
		return err
	}
	return nil
}
