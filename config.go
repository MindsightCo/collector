package main

import (
	"context"
	"fmt"
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

	// BUG: workaround for https://github.com/spf13/viper/issues/688
	viper.BindEnv("client_id", "MINDSIGHT_CLIENT_ID")
	viper.BindEnv("client_secret", "MINDSIGHT_CLIENT_SECRET")
	viper.BindEnv("api_server", "MINDSIGHT_API_SERVER")
	viper.BindEnv("cache_age", "MINDSIGHT_CACHE_AGE")
	viper.BindEnv("cache_depth", "MINDSIGHT_CACHE_DEPTH")
	viper.BindEnv("scrape_interval", "MINDSIGHT_SCRAPE_INTERVAL")
	viper.BindEnv("refresh_sources_interval", "MINDSIGHT_REFRESH_SOURCES_INTERVAL")

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

const strFmt = `
client_id: %s
client secret: XXXX
api_server: %s
cache_age: %s
cache_depth: %d
scrape_interval: %s
refresh_sources_interval: %s
`

func (c *Config) String() string {
	if c == nil {
		return "<nil>"
	}

	return fmt.Sprintf(strFmt, c.ClientID, c.APIServer, c.CacheAge, c.CacheDepth, c.ScrapeInterval, c.RefreshSourcesInterval)
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
	log.Println(c.String())

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
