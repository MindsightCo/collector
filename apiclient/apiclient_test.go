package apiclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	prommodel "github.com/prometheus/common/model"
)

var epoch = prommodel.TimeFromUnix(10)

const testToken = "joeblows-token"

type testConfig struct {
	server  *httptest.Server
	nCalls  int
	handler http.HandlerFunc
	token   *MockTokenBuilder
}

func (c *testConfig) handlerWrapper(w http.ResponseWriter, r *http.Request) {
	c.nCalls++
	c.handler(w, r)
}

func setup(t *testing.T, h http.HandlerFunc) (*testConfig, func(*testing.T)) {
	t.Helper()

	c := &testConfig{handler: h}
	c.server = httptest.NewServer(http.HandlerFunc(c.handlerWrapper))

	ctl := gomock.NewController(t)
	c.token = NewMockTokenBuilder(ctl)

	tearDown := func(t *testing.T) {
		t.Helper()
		ctl.Finish()

		c.server.Close()
		if c.nCalls != 1 {
			t.Fatalf("unexpected number of calls got: %d expected: 1\n", c.nCalls)
		}
	}

	return c, tearDown
}

func TestPushMetrics(t *testing.T) {
	metrics := map[int]prommodel.Vector{
		1: prommodel.Vector{
			&prommodel.Sample{
				Timestamp: epoch,
				Value:     prommodel.SampleValue(13.3),
				Metric: prommodel.Metric{
					"__name__": "joeblow",
				},
			},
		},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("wrong method, got: %s expected: POST", r.Method)
		}
		if r.URL.Path != "/metricsin/" {
			t.Fatalf("wrong path got: %s expected: /metricsin/", r.URL.Path)
		}
		if r.Header.Get("Content-type") != "application/json" {
			t.Fatalf("unexpected mime type, got %s, wanted application/json", r.Header.Get("Content-type"))
		}

		defer r.Body.Close()
		var metricsIn map[int]prommodel.Vector

		if err := json.NewDecoder(r.Body).Decode(&metricsIn); err != nil {
			t.Fatal("decode request body:", err)
		}
		if !cmp.Equal(metrics, metricsIn) {
			t.Fatal("unexpected input:", cmp.Diff(metrics, metricsIn))
		}
	}

	fixture, tearDown := setup(t, handler)
	defer tearDown(t)

	fixture.token.EXPECT().GetAccessToken().Return(testToken, nil)

	pusher := &MetricsPusher{url: fixture.server.URL + "/metricsin/", grant: fixture.token}
	if err := pusher.Push(metrics); err != nil {
		t.Fatal("push metrics:", err)
	}
}

func TestAPIClient(t *testing.T) {
	// input: map[source-id]prometheus-vector
	// expect: POST to metricsin/ route

	// input: query latest metric sources from graphql API
	// expect: cache sources are replaced, flushed metrics POSTed to server

	// TODO
	// subscribe to API metric source changes (needs backend support)
	// do test #2 above on metric source change
}
