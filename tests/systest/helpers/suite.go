package helpers

import (
	"io"
	"os"

	// . "github.com/onsi/ginkgo/v2"
	// . "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

type SuiteConfig struct {
	KubeConfig  string `yaml:"kubeConfig"`
	BIGIPConfig struct {
		Username  string `yaml:"username"`
		Password  string `yaml:"password"`
		IPAddress string `yaml:"ipAddress"`
		Port      int    `yaml:"port"`
	} `yaml:"bigipConfig"`
}

func (sc *SuiteConfig) Load(filepath string) error {
	if f, err := os.Open(filepath); err != nil {
		return err
	} else {
		b, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		sc.BIGIPConfig.Username = "admin"
		sc.BIGIPConfig.Port = 443
		if err := yaml.Unmarshal(b, sc); err != nil {
			return err
		}
	}

	return nil
}
