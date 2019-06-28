package main

import (
	"log"
	"time"

	"github.com/MindsightCo/metrics-agent/cache"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	defaultAPIServer  = "https://sre-api.mindsight.io/metricsin/"
	defaultCacheDepth = 1000
	defaultCacheAge   = time.Minute * 5
)

type ApiAddr string

func (a ApiAddr) QueryAddr() string {
	return string(a) + "query"
}

func (a ApiAddr) MetricsAddr() string {
	return string(a) + "metricsin/"
}

type Config struct {
	Sources      []cache.Source
	ClientID     string        `mapstructure:"client_id"`
	ClientSecret string        `mapstructure:"client_secret"`
	APIServer    ApiAddr       `mapstructure:"api_server"`
	TestMode     bool          `mapstructure:"test_mode"`
	CacheAge     time.Duration `mapstructure:"cache_age"`
	CacheDepth   int           `mapstructure:"cache_depth"`
}

// ReadConfig retrieves configuration values via viper. If a required
// value was not provided, an error will be returned.
func ReadConfig() (*Config, error) {
	var c Config

	viper.SetConfigName("mindsight-agent")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/mindsight/")
	viper.SetConfigType("yaml")

	viper.SetEnvPrefix("mindsight")
	viper.AutomaticEnv()

	viper.SetDefault("api_server", defaultAPIServer)
	viper.SetDefault("cache_age", defaultCacheAge)
	viper.SetDefault("cache_depth", defaultCacheDepth)

	// loads viper config
	err := viper.ReadInConfig()
	if err != nil {
		log.Println("(warning) Couldn't open config file:", err)
	}

	if err := viper.Unmarshal(&c); err != nil {
		return nil, errors.Wrap(err, "unmarshal configuration")
	}

	if c.ClientID == "" {
		return nil, errors.New("env variable MINDSIGHT_CLIENT_ID (or config client_id) must be given")
	}
	if c.ClientSecret == "" {
		return nil, errors.New("env variable MINDSIGHT_CLIENT_SECRET (or config client_secret) must be given")
	}

	return &c, nil
}
