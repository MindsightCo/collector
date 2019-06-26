package main

import (
	"log"

	auth0grant "github.com/ereyes01/go-auth0-grant"
	"github.com/pkg/errors"
)

const (
	credsAudience = "https://api.mindsight.io/"
	auth0TokenURL = "https://mindsight.auth0.com/oauth/token/"
)

func initGrant(c *Config) (*auth0grant.Grant, error) {
	if c.TestMode {
		return nil, nil
	}

	credRequest := auth0grant.CredentialsRequest{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Audience:     credsAudience,
		GrantType:    auth0grant.CLIENT_CREDS_GRANT_TYPE,
	}

	grant := auth0grant.NewGrant(auth0TokenURL, credRequest)

	// test the token
	if _, err := grant.GetAccessToken(); err != nil {
		return nil, errors.Wrap(err, "testing credentials")
	}

	return grant, nil
}

func main() {
	config, err := ReadConfig()
	if err != nil {
		log.Fatal("error verifying config:", err)
	}

	_, err = initGrant(config)
	if err != nil {
		log.Fatal("auth error:", err)
	}
}
