package cache

import (
	promclient "github.com/MindsightCo/metrics-agent/prometheus_client"
	prommodel "github.com/prometheus/common/model"
)

type Source struct {
	SourceID int `mapstructure:"source_id"`
	URL      string
	Queries  []string
	client   *promclient.PromClient
}

type Cache struct {
	source  *Source
	values  map[int]prommodel.Vector
	limit   int
	nCached int
}
