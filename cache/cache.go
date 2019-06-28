package cache

import (
	"context"
	"time"

	promclient "github.com/MindsightCo/metrics-agent/prometheus_client"
	"github.com/pkg/errors"
	prommodel "github.com/prometheus/common/model"
)

type queryer interface {
	Query(ctx context.Context, query string) (prommodel.Vector, error)
}

type Source struct {
	SourceID int `mapstructure:"source_id"`
	URL      string
	Query    string
	client   queryer
}

type Cache struct {
	sources       []Source
	values        map[int]prommodel.Vector
	nCache, limit int
	lastFlush     time.Time
	timeLimit     time.Duration
	nowFn         func() time.Time
}

func (c *Cache) NewSources(sources []Source) (map[int]prommodel.Vector, error) {
	conns := make(map[string]*promclient.PromClient)

	sourcesCopy := append([]Source{}, sources...)
	for idx, src := range sourcesCopy {
		if client, present := conns[src.URL]; present {
			sourcesCopy[idx].client = client
			continue
		}

		client, err := promclient.NewPromClient(src.URL)
		if err != nil {
			return nil, errors.Wrap(err, "connect to prometheus server")
		}

		sourcesCopy[idx].client = client
		conns[src.URL] = client
	}

	prevValues := c.values
	c.values = make(map[int]prommodel.Vector)
	c.sources = sourcesCopy

	return prevValues, nil
}

func (c *Cache) Collect(ctx context.Context) (map[int]prommodel.Vector, error) {
	for _, src := range c.sources {
		results, err := src.client.Query(ctx, src.Query)
		if err != nil {
			return nil, errors.Wrapf(err, "query: %s url: %s", src.Query, src.URL)
		}

		c.values[src.SourceID] = append(c.values[src.SourceID], results...)
		c.nCache += len(results)
	}

	var flushed map[int]prommodel.Vector
	deadline := c.lastFlush.Add(c.timeLimit)
	now := c.nowFn()

	if c.nCache >= c.limit || now.After(deadline) {
		flushed = c.values
		c.values = make(map[int]prommodel.Vector)
		c.nCache = 0
		c.lastFlush = now
	}

	return flushed, nil
}
