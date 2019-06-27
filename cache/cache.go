package cache

import (
	promclient "github.com/MindsightCo/metrics-agent/prometheus_client"
	"github.com/pkg/errors"
	prommodel "github.com/prometheus/common/model"
)

type Source struct {
	SourceID int `mapstructure:"source_id"`
	URL      string
	Queries  []string
	client   *promclient.PromClient
}

type Cache struct {
	sources []Source
	values  map[int]prommodel.Vector
}

func (c *Cache) NewSources(sources []Source) (map[int]prommodel.Vector, error) {
	sourcesCopy := append([]Source{}, sources...)
	for idx, src := range sourcesCopy {
		client, err := promclient.NewPromClient(src.URL)
		if err != nil {
			// TODO: close all previous connections?
			return nil, errors.Wrap(err, "connect to prometheus server")
		}

		sourcesCopy[idx].client = client
	}

	prevValues := c.values
	c.values = make(map[int]prommodel.Vector)
	c.sources = sourcesCopy

	return prevValues, nil
}
