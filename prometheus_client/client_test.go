package promclient

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	prommodel "github.com/prometheus/common/model"
)

var (
	epoch      = prommodel.TimeFromUnix(10)
	testQuery  = "im-a-query"
	testSample = 42.2
)

func testContext(t *testing.T) context.Context {
	t.Helper()

	return context.WithValue(context.Background(), "MSTEST", "mstest")
}

func testTime() time.Time {
	return epoch.Time()
}

func TestPrometheusClient(t *testing.T) {
	testCtx := testContext(t)

	expectedResult := prommodel.Vector{
		&prommodel.Sample{
			Timestamp: epoch,
			Value:     prommodel.SampleValue(13.3),
			Metric: prommodel.Metric{
				"__name__": "fred",
			},
		},
	}

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mockAPI := NewMockAPI(ctl)
	mockAPI.EXPECT().Query(testCtx, testQuery, epoch.Time()).Return(expectedResult, nil)

	promClient := &PromClient{api: mockAPI, nowFn: testTime}
	result, err := promClient.Query(testCtx, testQuery)
	if err != nil {
		t.Fatal("promclient execute query:", err)
	} else if !cmp.Equal(result, expectedResult) {
		t.Fatalf("invalid query result: %s", cmp.Diff(result, expectedResult))
	}
}
