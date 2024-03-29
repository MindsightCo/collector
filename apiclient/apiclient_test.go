package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MindsightCo/collector/cache"
	gomock "github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	prommodel "github.com/prometheus/common/model"
)

var epoch = prommodel.TimeFromUnix(10)

const testToken = "joeblows-token"

type testConfig struct {
	server  *httptest.Server
	nCalls  int
	handler http.HandlerFunc
	token   *MockTokenBuilder
	ctx     context.Context
}

func (c *testConfig) handlerWrapper(w http.ResponseWriter, r *http.Request) {
	c.nCalls++
	c.handler(w, r)
}

func setup(t *testing.T, h http.HandlerFunc, nCalls int) (*testConfig, func(*testing.T)) {
	t.Helper()
	ctl := gomock.NewController(t)

	c := &testConfig{
		handler: h,
		ctx:     context.WithValue(context.Background(), "MSTEST", "mstest"),
		token:   NewMockTokenBuilder(ctl),
	}
	c.server = httptest.NewServer(http.HandlerFunc(c.handlerWrapper))

	tearDown := func(t *testing.T) {
		t.Helper()
		ctl.Finish()

		c.server.Close()
		if c.nCalls != nCalls {
			t.Fatalf("unexpected number of calls got: %d expected: %d\n", c.nCalls, nCalls)
		}
	}

	return c, tearDown
}

func pushHandler(t *testing.T, expMetrics map[int]prommodel.Vector) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("wrong method, got: %s expected: POST", r.Method)
		}
		if r.URL.Path != "/metrics/" {
			t.Fatalf("wrong path got: %s expected: /metrics/", r.URL.Path)
		}
		if r.Header.Get("Content-type") != "application/json" {
			t.Fatalf("unexpected mime type, got %s, wanted application/json", r.Header.Get("Content-type"))
		}

		defer r.Body.Close()
		var metricsIn map[int]prommodel.Vector

		if err := json.NewDecoder(r.Body).Decode(&metricsIn); err != nil {
			t.Fatal("decode request body:", err)
		}
		if !cmp.Equal(expMetrics, metricsIn) {
			t.Fatal("unexpected input:", cmp.Diff(expMetrics, metricsIn))
		}
	}
}

func TestPushMetrics(t *testing.T) {
	var cases = []struct {
		name        string
		metrics     map[int]prommodel.Vector
		nCalls      int
		expectToken bool
	}{
		{
			name: "Pushes a prometheus vector",
			metrics: map[int]prommodel.Vector{
				1: prommodel.Vector{
					&prommodel.Sample{
						Timestamp: epoch,
						Value:     prommodel.SampleValue(13.3),
						Metric: prommodel.Metric{
							"__name__": "joeblow",
						},
					},
				},
			},
			nCalls:      1,
			expectToken: true,
		},
		{
			name:        "Pushes empty data",
			metrics:     map[int]prommodel.Vector{},
			nCalls:      0,
			expectToken: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture, tearDown := setup(t, pushHandler(t, tc.metrics), tc.nCalls)
			defer tearDown(t)

			if tc.expectToken {
				fixture.token.EXPECT().GetAccessToken().Return(testToken, nil)
			}

			pusher, err := NewMetricsPusher(fixture.server.URL, fixture.token)
			if err != nil {
				t.Fatal("new metrics pusher:", err)
			}

			if err := pusher.Push(fixture.ctx, tc.metrics); err != nil {
				t.Fatal("push metrics:", err)
			}
		})
	}
}

const sourcesJSON = `
{
	"data": {
		"metricSources": [
			{"id":77, "sourceURL": "http://source-1", "query":"query{num=\"1\"}"},
			{"id":77, "sourceURL": "http://source-2", "query":"query{num=\"2\"}"}
		]
	}
}
`

func TestRefreshSources(t *testing.T) {
	expSources := []cache.Source{
		{
			SourceID: 77,
			URL:      "http://source-1",
			Query:    "query{num=\"1\"}",
		},
		{
			SourceID: 77,
			URL:      "http://source-2",
			Query:    "query{num=\"2\"}",
		},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/query" {
			t.Fatalf("wrong path got: %s expected: /query", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "bearer "+testToken {
			t.Fatalf("auth header got: ``%s'' expected: ``bearer %s''", r.Header.Get("Authorization"), testToken)
		}

		defer r.Body.Close()
		var gqlRequest struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}

		if err := json.NewDecoder(r.Body).Decode(&gqlRequest); err != nil {
			t.Fatal("decode request body:", err)
		}
		if gqlRequest.Variables != nil {
			t.Fatal("didn't expect any graphql variables")
		}
		if gqlRequest.Query != metricSourcesQuery {
			t.Fatalf("graphql query got: ``%s'' expected: ``%s''", gqlRequest, metricSourcesQuery)
		}

		if _, err := w.Write([]byte(sourcesJSON)); err != nil {
			t.Fatal("write sources response:", err)
		}
	}

	fixture, tearDown := setup(t, handler, 1)
	defer tearDown(t)

	fixture.token.EXPECT().GetAccessToken().Return(testToken, nil)

	q, err := NewQueryer(fixture.server.URL, fixture.token)
	if err != nil {
		t.Fatal("new queryer:", err)
	}

	newSources, err := q.QuerySources(fixture.ctx)
	if err != nil {
		t.Fatal("query sources:", err)
	}
	ign := cmpopts.IgnoreUnexported(cache.Source{})
	if !cmp.Equal(expSources, newSources, ign) {
		t.Fatal("unexpected sources response:", cmp.Diff(expSources, newSources, ign))
	}
}
