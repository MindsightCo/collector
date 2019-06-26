package main

import (
	"errors"
	"log"

	"github.com/spf13/viper"
)

const defaultAPIServer = "https://api.mindsight.io/query"

type Config struct {
	Sources      []string
	ClientID     string
	ClientSecret string
	APIServer    string
	TestMode     bool
}

// ReadConfig retrieves configuration values via viper. If a required
// value was not provided, an error will be returned.
func ReadConfig() (*Config, error) {
	var c Config

	viper.SetConfigName("mindsight-agent")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/mindsight/")

	viper.SetEnvPrefix("mindsight")
	viper.AutomaticEnv()

	// loads viper config
	err := viper.ReadInConfig()
	if err != nil {
		log.Println("(warning) Couldn't open config file:", err)
	}

	c.Sources = viper.GetStringSlice("sources")
	if len(c.Sources) == 0 {
		return nil, errors.New("must provide at least 1 metrics source url")
	}

	c.ClientID = viper.GetString("client_id")
	if c.ClientID == "" {
		return nil, errors.New("env variable MINDSIGHT_CLIENT_ID (or config client_id) must be given")
	}

	c.ClientSecret = viper.GetString("client_secret")
	if c.ClientSecret == "" {
		return nil, errors.New("env variable MINDSIGHT_CLIENT_SECRET (or config client_secret) must be given")
	}

	c.APIServer = viper.GetString("api_server")
	if c.APIServer == "" {
		c.APIServer = defaultAPIServer
	}

	c.TestMode = viper.GetBool("test_mode")

	return &c, nil
}
