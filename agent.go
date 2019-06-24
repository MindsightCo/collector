package main

import (
	"log"

	auth0grant "github.com/ereyes01/go-auth0-grant"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	CREDS_AUDIENCE     = "https://api.mindsight.io/"
	DEFAULT_API_SERVER = "https://api.mindsight.io/query"
	AUTH0_TOKEN_URL    = "https://mindsight.auth0.com/oauth/token/"
)

func initGrant(c *Config) (*auth0grant.Grant, error) {
	if c.TestMode {
		return nil, nil
	}

	credRequest := auth0grant.CredentialsRequest{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Audience:     CREDS_AUDIENCE,
		GrantType:    auth0grant.CLIENT_CREDS_GRANT_TYPE,
	}

	grant := auth0grant.NewGrant(AUTH0_TOKEN_URL, credRequest)

	// test the token
	if _, err := grant.GetAccessToken(); err != nil {
		return nil, errors.Wrap(err, "testing credentials")
	}

	return grant, nil
}

func main() {
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

	// builds my config object
	config, err := ReadConfig()
	if err != nil {
		log.Fatal("error verifying config:", err)
	}

	_, err = initGrant(config)
	if err != nil {
		log.Fatal("auth error:", err)
	}
}
