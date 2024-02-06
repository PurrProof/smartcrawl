package app

import (
	"strings"

	"github.com/juju/errors"
	"github.com/spf13/viper"
)

type ItemProviderConfig map[string]interface{}

type JobConfig struct {
	Priority   int
	TimeoutSec int
	MaxRetry   int
}

type QueueConfig struct {
	Addr     string
	User     string
	Password string
	Job      *JobConfig
}

type StorageConfig struct {
	Uri      string
	User     string
	Password string
	DbName   string
}

type AppConfig struct {
	Providers map[string]*ItemProviderConfig
	LogLevel  string
	Queue     *QueueConfig
	Storage   *StorageConfig
}

func NewConfig(path ...string) (*AppConfig, error) {
	//suppose that env vars already loaded
	//there is no sensitive information in config
	//it should be populated there from env file

	config := AppConfig{}

	//https://github.com/spf13/viper#reading-config-files
	confPath := "."
	if len(path) != 0 {
		confPath = path[0]
	}
	viper.AddConfigPath(confPath)
	viper.SetConfigName("config")

	err := viper.ReadInConfig()
	if err != nil {
		return nil, errors.Annotate(err, "can't read config.json")
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err = viper.Unmarshal(&config)
	if err != nil {
		return nil, errors.Annotate(err, "can't unmarshal config file")
	} else if 0 == len(config.Providers) {
		return nil, errors.New("provider(s) not found in config.json")
	}

	return &config, nil
}

func (conf *AppConfig) GetProviderConfigByKey(provKey string) (*ItemProviderConfig, error) {
	// Keys in the config map are in lowercase, as Viper reads them
	if pconf, found := conf.Providers[strings.ToLower(provKey)]; found {
		return pconf, nil
	}
	return nil, errors.Errorf("provider config not found, key=%s", provKey)
}
