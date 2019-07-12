package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/MindsightCo/collector/cache"
	"github.com/machinebox/graphql"
	"github.com/pkg/errors"
	prommodel "github.com/prometheus/common/model"
)

type apiAddr struct {
	base, query, metrics *url.URL
}

func newAPIAddr(serverURL string) (*apiAddr, error) {
	base, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}

	query, err := url.Parse("query")
	if err != nil {
		return nil, err
	}

	metrics, err := url.Parse("metricsin/")
	if err != nil {
		return nil, err
	}

	return &apiAddr{
		base:    base,
		query:   query,
		metrics: metrics,
	}, nil
}

func (a *apiAddr) queryAddr() string {
	return a.base.ResolveReference(a.query).String()
}

func (a *apiAddr) metricsAddr() string {
	return a.base.ResolveReference(a.metrics).String()
}

type TokenBuilder interface {
	GetAccessToken() (string, error)
}

type MetricsPusher struct {
	url  string
	auth TokenBuilder
}

func NewMetricsPusher(url string, auth TokenBuilder) (*MetricsPusher, error) {
	addr, err := newAPIAddr(url)
	if err != nil {
		return nil, err
	}

	return &MetricsPusher{
		url:  addr.metricsAddr(),
		auth: auth,
	}, nil
}

func (p *MetricsPusher) Push(ctx context.Context, metrics map[int]prommodel.Vector) error {
	token, err := p.auth.GetAccessToken()
	if err != nil {
		return errors.Wrap(err, "get access token:")
	}

	payload, err := json.Marshal(metrics)
	if err != nil {
		return errors.Wrap(err, "json marshal metrics")
	}

	req, err := http.NewRequest("POST", p.url, bytes.NewBuffer(payload))
	if err != nil {
		return errors.Wrap(err, "create http request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "bearer "+token)

	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "do http request")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := ioutil.ReadAll(resp.Body)
		return errors.Errorf("response status: %s, body: %s", resp.Status, string(body))
	}

	return nil
}

type Queryer struct {
	client *graphql.Client
	auth   TokenBuilder
}

const metricSourcesQuery = `{
	metricSources {
		id
		sourceURL
		query
	}
}`

func NewQueryer(url string, token TokenBuilder) (*Queryer, error) {
	serverURL, err := newAPIAddr(url)
	if err != nil {
		return nil, err
	}

	client := graphql.NewClient(serverURL.queryAddr())

	return &Queryer{
		client: client,
		auth:   token,
	}, nil
}

func (q *Queryer) QuerySources(ctx context.Context) ([]cache.Source, error) {
	authToken, err := q.auth.GetAccessToken()
	if err != nil {
		return nil, errors.Wrap(err, "get auth token")
	}

	request := graphql.NewRequest(metricSourcesQuery)
	request.Header.Set("Authorization", "bearer "+authToken)

	var sources []cache.Source
	if err := q.client.Run(ctx, request, &sources); err != nil {
		return nil, errors.Wrap(err, "query new sources:")
	}

	return sources, nil
}
