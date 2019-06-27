package cache

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	prommodel "github.com/prometheus/common/model"
)

var epoch = prommodel.TimeFromUnix(10)

func TestNewSources(t *testing.T) {
	prevValues := map[int]prommodel.Vector{
		23: prommodel.Vector{
			&prommodel.Sample{
				Timestamp: 0,
				Value:     prommodel.SampleValue(13.3),
				Metric: prommodel.Metric{
					"__name__": "fred",
				},
			},
		},
	}

	c := &Cache{
		sources: []Source{{SourceID: 2}},
		values:  prevValues,
	}

	newSources := []Source{
		{SourceID: 1, URL: "blah"},
		{SourceID: 2, URL: "a-url"},
		{SourceID: 3, URL: "a-url"},
	}

	values, err := c.NewSources(newSources)
	if err != nil {
		t.Fatal("set new sources:", err)
	}
	if !cmp.Equal(values, prevValues) {
		t.Fatalf("unexpected values returned, diff: %s", cmp.Diff(values, prevValues))
	}

	ignoreUnexp := cmpopts.IgnoreUnexported(Source{})

	if !cmp.Equal(c.sources, newSources, ignoreUnexp) {
		t.Fatalf("unexpected sources in cache: %s", cmp.Diff(c.sources, newSources, ignoreUnexp))
	}
	if len(c.values) != 0 {
		t.Fatal("unexpected values in cache after new sources:", c.values)
	}

	for _, src := range c.sources {
		if src.client == nil {
			t.Fatal("found a source without a prometheus client:", src)
		}
	}

	if c.sources[1].client != c.sources[2].client {
		t.Fatal("src 1 and 2 have the same url but do not share the same client")
	}
}

func TestCache(t *testing.T) {
	// needs a func that runs all the queries in its sources and caches the results,
	// potentially flushing the cache
}
