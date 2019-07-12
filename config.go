package main

import (
	"context"
	"log"
	"time"

	"github.com/MindsightCo/collector/apiclient"
	"github.com/MindsightCo/collector/cache"
	auth0grant "github.com/ereyes01/go-auth0-grant"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	defaultAPIServer              = "https://sre-api.mindsight.io/"
	defaultCacheDepth             = 1000
	defaultCacheAge               = time.Minute * 5
	defaultScrapeInterval         = time.Second * 5
	defaultRefreshSourcesInterval = time.Hour

	credsAudience = "https://api.mindsight.io/"
	auth0TokenURL = "https://mindsight.auth0.com/oauth/token/"
)

type Config struct {
	Sources                []cache.Source
	ClientID               string        `mapstructure:"client_id"`
	ClientSecret           string        `mapstructure:"client_secret"`
	APIServer              string        `mapstructure:"api_server"`
	TestMode               bool          `mapstructure:"test_mode"`
	CacheAge               time.Duration `mapstructure:"cache_age"`
	CacheDepth             int           `mapstructure:"cache_depth"`
	ScrapeInterval         time.Duration `mapstructure:"scrape_interval"`
	RefreshSourcesInterval time.Duration `mapstructure:"refresh_sources_interval"`

	auth    *auth0grant.Grant
	cache   *cache.Cache
	pusher  *apiclient.MetricsPusher
	queryer *apiclient.Queryer
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
	viper.SetDefault("scrape_interval", defaultScrapeInterval)
	viper.SetDefault("refresh_sources_interval", defaultRefreshSourcesInterval)

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

func (c *Config) initAuth() error {
	credRequest := auth0grant.CredentialsRequest{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Audience:     credsAudience,
		GrantType:    auth0grant.CLIENT_CREDS_GRANT_TYPE,
	}

	grant := auth0grant.NewGrant(auth0TokenURL, credRequest)

	// test the token
	if _, err := grant.GetAccessToken(); err != nil {
		return errors.Wrap(err, "testing credentials")
	}

	c.auth = grant
	return nil
}

func (c *Config) init() error {
	if err := c.initAuth(); err != nil {
		return errors.Wrap(err, "init auth")
	}

	cache, err := cache.NewCache(c.Sources, c.CacheDepth, c.CacheAge)
	if err != nil {
		return errors.Wrap(err, "init cache")
	}

	pusher, err := apiclient.NewMetricsPusher(c.APIServer, c.auth)
	if err != nil {
		return errors.Wrap(err, "init metrics pusher")
	}

	queryer, err := apiclient.NewQueryer(c.APIServer, c.auth)
	if err != nil {
		return errors.Wrap(err, "init queryer")
	}

	c.cache = cache
	c.pusher = pusher
	c.queryer = queryer

	if err := c.refreshSources(context.Background()); err != nil {
		return errors.Wrap(err, "init refresh sources")
	}

	return nil
}

func (c *Config) scrape(ctx context.Context) error {
	data, err := c.cache.Collect(ctx)
	if err != nil {
		return errors.Wrap(err, "scrape")
	}

	if data != nil {
		if err := c.pusher.Push(ctx, data); err != nil {
			return errors.Wrap(err, "push from scrape")
		}
	}

	return nil
}

func (c *Config) refreshSources(ctx context.Context) error {
	sources, err := c.queryer.QuerySources(ctx)
	if err != nil {
		return errors.Wrap(err, "query sources")
	}

	data, err := c.cache.NewSources(sources)
	if err != nil {
		return errors.Wrap(err, "set new sources")
	}

	if data != nil {
		if err := c.pusher.Push(ctx, data); err != nil {
			return errors.Wrap(err, "push after refresh sources")
		}
	}

	return nil
}

func (c *Config) Loop() error {
	if err := c.init(); err != nil {
		return errors.Wrap(err, "init metrics collector")
	}

	scrapeTimer := time.NewTimer(c.ScrapeInterval)
	refreshSourcesTimer := time.NewTimer(c.RefreshSourcesInterval)
	ctx := context.Background()

	for {
		select {
		case <-scrapeTimer.C:
			if err := c.scrape(ctx); err != nil {
				log.Println("WARNING (scrape):", err)
			}
			scrapeTimer.Reset(c.ScrapeInterval)

		// TODO: instead of a timer, replace with a subscription notification
		// from the API
		case <-refreshSourcesTimer.C:
			if err := c.refreshSources(ctx); err != nil {
				log.Println("WARNING (refreshSources):", err)
			}
			refreshSourcesTimer.Reset(c.RefreshSourcesInterval)
		}
	}
}
