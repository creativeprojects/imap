package cfg

import (
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type AccountType string

const (
	IMAP    AccountType = "imap"
	MAILDIR AccountType = "maildir"
	LOCAL   AccountType = "local"
)

type Config struct {
	Accounts map[string]Account `yaml:"accounts"`
}

type Account struct {
	Type                AccountType `yaml:"type"`
	ServerURL           string      `yaml:"serverURL"`
	Username            string      `yaml:"username"`
	Password            string      `yaml:"password"`
	Root                string      `yaml:"root"`
	File                string      `yaml:"file"`
	SkipTLSVerification bool        `yaml:"skipTLSverification"`
}

func newConfig() *Config {
	return &Config{}
}

// LoadFromFile loads the configuration from the file
func LoadFromFile(fileName string) (*Config, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	return loadConfig(file)
}

// loadConfig from a io.ReadCloser
func loadConfig(reader io.ReadCloser) (*Config, error) {
	defer reader.Close()
	decoder := yaml.NewDecoder(reader)
	config := newConfig()
	err := decoder.Decode(config)
	if err != nil {
		return nil, err
	}
	validateConfiguration(config)
	return config, nil
}

func validateConfiguration(config *Config) {

}
