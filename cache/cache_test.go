package cache

import (
	"context"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
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
					"__name__": "joeblow",
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

func cacheMustEqual(t *testing.T, c1, c2 Cache) {
	t.Helper()

	// The reason we have to write this custom comparer...
	// The client field is hard to compare because it could be a mock,
	// and there's many levels deep of unexported fields.
	ign := cmpopts.IgnoreUnexported(Source{})

	if !cmp.Equal(c1.sources, c2.sources, ign) {
		t.Fatal("cache sources differ:", cmp.Diff(c1.sources, c2.sources, ign))
	}
	if !cmp.Equal(c1.values, c2.values) {
		t.Fatal("cache values differ", cmp.Diff(c1.values, c2.values))
	}
	if c1.nCache != c2.nCache {
		t.Fatalf("cache nCache differs: %d vs %d\n", c1.nCache, c2.nCache)
	}
	if c1.limit != c2.limit {
		t.Fatalf("cache limit differs: %d vs %d\n", c1.limit, c2.limit)
	}
	if !c1.lastFlush.Equal(c2.lastFlush) {
		t.Fatalf("cache lastFlush differs: %s vs %s\n", c1.lastFlush.Format(time.RFC3339), c2.lastFlush.Format(time.RFC3339))
	}
}

func testNow() time.Time {
	return epoch.Time()
}

func testNowElapsed() time.Time {
	return epoch.Time().Add(10 * time.Minute)
}

type cacheTestCase struct {
	testName string
	cache    *Cache

	// what we expect Collect() to return
	expReturn map[int]prommodel.Vector

	// what we expect each source's query will return:
	expQueryResults map[int]prommodel.Vector

	// expected state of the cache after the Collect() call
	expCache Cache
}

func TestCache(t *testing.T) {
	setup := func(t *testing.T, tc cacheTestCase) (context.Context, func(*testing.T)) {
		t.Helper()

		testCtx := context.WithValue(context.Background(), "MSTEST", "mstest")
		ctl := gomock.NewController(t)
		mockQueryer := NewMockqueryer(ctl)

		for idx, src := range tc.cache.sources {
			if vec, present := tc.expQueryResults[src.SourceID]; present {
				mockQueryer.EXPECT().Query(testCtx, src.Query).Return(vec, nil)
			}

			tc.cache.sources[idx].client = mockQueryer
		}

		tearDown := func(t *testing.T) {
			t.Helper()
			ctl.Finish()
		}

		return testCtx, tearDown
	}

	cases := []cacheTestCase{
		{
			testName: "queries when there's no sources (empty/nil value returned)",
			cache: &Cache{
				limit:     1,
				nowFn:     testNow,
				lastFlush: epoch.Time(),
				timeLimit: 5 * time.Minute,
			},
			expCache: Cache{
				limit:     1,
				nowFn:     testNow,
				lastFlush: epoch.Time(),
				timeLimit: 5 * time.Minute,
			},
		},
		{
			testName: "collect query result without filling the cache",
			cache: &Cache{
				sources: []Source{
					{
						SourceID: 1,
						URL:      "a-url",
						Query:    "a-query",
					},
				},
				values:    map[int]prommodel.Vector{},
				limit:     2,
				nowFn:     testNow,
				lastFlush: epoch.Time(),
				timeLimit: 5 * time.Minute,
			},
			expQueryResults: map[int]prommodel.Vector{
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
			expCache: Cache{
				sources: []Source{
					{
						SourceID: 1,
						URL:      "a-url",
						Query:    "a-query",
					},
				},
				values: map[int]prommodel.Vector{
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
				limit:     2,
				nCache:    1,
				nowFn:     testNow,
				lastFlush: epoch.Time(),
				timeLimit: 5 * time.Minute,
			},
		},
		{
			testName: "collect query result that fills and flushes the cache",
			cache: &Cache{
				sources: []Source{
					{
						SourceID: 1,
						URL:      "a-url",
						Query:    "a-query",
					},
				},
				values: map[int]prommodel.Vector{
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
				limit:     2,
				nCache:    1,
				nowFn:     testNow,
				lastFlush: epoch.Time(),
				timeLimit: 5 * time.Minute,
			},
			expCache: Cache{
				sources: []Source{
					{
						SourceID: 1,
						URL:      "a-url",
						Query:    "a-query",
					},
				},
				values:    map[int]prommodel.Vector{},
				limit:     2,
				nowFn:     testNow,
				lastFlush: epoch.Time(),
				timeLimit: 5 * time.Minute,
			},
			expQueryResults: map[int]prommodel.Vector{
				1: prommodel.Vector{
					&prommodel.Sample{
						Timestamp: epoch,
						Value:     prommodel.SampleValue(14.4),
						Metric: prommodel.Metric{
							"__name__": "gomie",
						},
					},
				},
			},
			expReturn: map[int]prommodel.Vector{
				1: prommodel.Vector{
					&prommodel.Sample{
						Timestamp: epoch,
						Value:     prommodel.SampleValue(13.3),
						Metric: prommodel.Metric{
							"__name__": "joeblow",
						},
					},
					&prommodel.Sample{
						Timestamp: epoch,
						Value:     prommodel.SampleValue(14.4),
						Metric: prommodel.Metric{
							"__name__": "gomie",
						},
					},
				},
			},
		},
		{
			testName: "collect query result without filling the cache, but exceeding maximum cache age",
			cache: &Cache{
				sources: []Source{
					{
						SourceID: 1,
						URL:      "a-url",
						Query:    "a-query",
					},
				},
				values:    map[int]prommodel.Vector{},
				limit:     2,
				nowFn:     testNowElapsed,
				lastFlush: epoch.Time(),
				timeLimit: 5 * time.Minute,
			},
			expQueryResults: map[int]prommodel.Vector{
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
			expCache: Cache{
				sources: []Source{
					{
						SourceID: 1,
						URL:      "a-url",
						Query:    "a-query",
					},
				},
				values:    map[int]prommodel.Vector{},
				limit:     2,
				nowFn:     testNowElapsed,
				lastFlush: epoch.Time().Add(10 * time.Minute),
				timeLimit: 5 * time.Minute,
			},
			expReturn: map[int]prommodel.Vector{
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
		},
	}

	for _, tc := range cases {
		t.Run(tc.testName, func(t *testing.T) {
			testCtx, tearDown := setup(t, tc)
			defer tearDown(t)

			values, err := tc.cache.Collect(testCtx)
			if err != nil {
				t.Fatal("collect failed:", err)
			}
			if !cmp.Equal(tc.expReturn, values) {
				t.Fatal("collect: unexpected values returned:", cmp.Diff(tc.expReturn, values))
			}
			cacheMustEqual(t, tc.expCache, *tc.cache)
		})
	}
}
