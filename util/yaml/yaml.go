package yaml

import (
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

func ReadYaml(pn string, v interface{}) error {
	file, err := os.Open(pn)
	if err != nil {
		return err
	}
	defer file.Close()
	return ReadYamlRdr(file, v)
}

func Marshal(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

func ReadYamlRdr(rdr io.Reader, v interface{}) error {
	d := yaml.NewDecoder(rdr)
	if err := d.Decode(v); err != nil {
		return err
	}
	return nil
}
