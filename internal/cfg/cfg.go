package cfg

import (
	"gopkg.in/yaml.v2"
	"io"
)

type CheckService struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type ServiceCfg struct {
	Name string        `yaml:"service"`
	URLs []string      `yaml:"urls"`
	CS   *CheckService `yaml:"require-auth"`
}

type ServicesConfigs struct {
	Services []ServiceCfg `yaml:"services"`
}

func GetCfgs(r io.Reader) (ServicesConfigs, error) {
	cfgTxt, err := io.ReadAll(r)

	if err != nil {
		return ServicesConfigs{}, err
	}

	cfgs := ServicesConfigs{}

	err = yaml.Unmarshal(cfgTxt, &cfgs)

	if err != nil {
		return ServicesConfigs{}, err
	}

	return cfgs, nil
}
