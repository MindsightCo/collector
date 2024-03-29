// package promclient contains abstracted operations that need to be performed
// against the Prometheus API.
package promclient

import (
	"context"
	"time"

	"github.com/pkg/errors"

	promapi "github.com/prometheus/client_golang/api"
	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	prommodel "github.com/prometheus/common/model"
)

// PromClient contains a connection to a prometheus server and allows query execution.
type PromClient struct {
	api   prometheus.API
	nowFn func() time.Time
}

// NewPromClient initializes a connection to a Prometheus server at the given url.
func NewPromClient(url string) (*PromClient, error) {
	client, err := promapi.NewClient(promapi.Config{
		Address: url,
	})
	if err != nil {
		return nil, errors.Wrap(err, "new prometheus client connection")
	}

	return &PromClient{
		api:   prometheus.NewAPI(client),
		nowFn: time.Now,
	}, nil
}

// Query executes the given PromQL query and returns the resulting instant vector,
// or an error if one occurred.
func (c *PromClient) Query(ctx context.Context, query string) (prommodel.Vector, error) {
	result, _, err := c.api.Query(ctx, query, c.nowFn())
	if err != nil {
		return nil, errors.Wrap(err, "execute prometheus query")
	}

	if result.Type() != prommodel.ValVector {
		return nil, errors.Errorf("expected vector result type, got: %s", result.Type())
	}

	v := result.(prommodel.Vector)

	if len(v) == 0 {
		return nil, errors.New("empty result vector")
	}

	return v, nil
}
