package metrics

import (
	"errors"
	"testing"

	"github.com/prometheus/common/model"
)

func TestExtractMTUReturnsLatestSample(t *testing.T) {
	value := model.Matrix{
		&model.SampleStream{
			Values: []model.SamplePair{
				{Value: model.SampleValue(1400)},
				{Value: model.SampleValue(1500)},
			},
		},
	}

	mtu, err := extractMTU(value)
	if err != nil {
		t.Fatalf("extractMTU returned unexpected error: %v", err)
	}
	if mtu != 1500 {
		t.Fatalf("extractMTU returned %d, expected 1500", mtu)
	}
}

func TestExtractMTUSkipsEmptyStreams(t *testing.T) {
	value := model.Matrix{
		nil,
		&model.SampleStream{},
		&model.SampleStream{
			Values: []model.SamplePair{
				{Value: model.SampleValue(9000)},
			},
		},
	}

	mtu, err := extractMTU(value)
	if err != nil {
		t.Fatalf("extractMTU returned unexpected error: %v", err)
	}
	if mtu != 9000 {
		t.Fatalf("extractMTU returned %d, expected 9000", mtu)
	}
}

func TestExtractMTURejectsEmptyResponse(t *testing.T) {
	testCases := []struct {
		name  string
		value model.Value
	}{
		{
			name:  "empty matrix",
			value: model.Matrix{},
		},
		{
			name: "streams without samples",
			value: model.Matrix{
				&model.SampleStream{},
				&model.SampleStream{Values: []model.SamplePair{}},
			},
		},
		{
			name:  "unexpected type",
			value: model.Vector{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := extractMTU(tc.value); err == nil {
				t.Fatal("extractMTU succeeded, expected an error")
			}
		})
	}
}

func TestExtractNodeCPUReturnsModeAverages(t *testing.T) {
	value := model.Matrix{
		&model.SampleStream{
			Metric: model.Metric{"mode": "idle"},
			Values: []model.SamplePair{
				{Value: model.SampleValue(10)},
				{Value: model.SampleValue(30)},
			},
		},
		&model.SampleStream{
			Metric: model.Metric{"mode": "user"},
			Values: []model.SamplePair{
				{Value: model.SampleValue(4)},
				{Value: model.SampleValue(6)},
			},
		},
	}

	cpu, err := extractNodeCPU(value, "node-a")
	if err != nil {
		t.Fatalf("extractNodeCPU returned unexpected error: %v", err)
	}
	if cpu.Idle != 20 {
		t.Fatalf("extractNodeCPU idle = %f, expected 20", cpu.Idle)
	}
	if cpu.User != 5 {
		t.Fatalf("extractNodeCPU user = %f, expected 5", cpu.User)
	}
}

func TestExtractNodeCPURejectsMissingUsableSamples(t *testing.T) {
	testCases := []struct {
		name  string
		value model.Value
	}{
		{
			name:  "empty matrix",
			value: model.Matrix{},
		},
		{
			name: "streams without samples",
			value: model.Matrix{
				&model.SampleStream{},
				&model.SampleStream{Values: []model.SamplePair{}},
			},
		},
		{
			name: "series without a known cpu mode",
			value: model.Matrix{
				&model.SampleStream{
					Metric: model.Metric{"mode": "guest"},
					Values: []model.SamplePair{
						{Value: model.SampleValue(1)},
					},
				},
			},
		},
		{
			name:  "unexpected type",
			value: model.Vector{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := extractNodeCPU(tc.value, "node-a"); err == nil {
				t.Fatal("extractNodeCPU succeeded, expected an error")
			}
		})
	}
}

func TestQueryPrometheusWithRetrySucceedsAfterTransientErrors(t *testing.T) {
	attempts := 0
	expected := NodeCPU{Idle: 42}

	cpu, err := queryPrometheusWithRetry("test query", 3, 0, func() (NodeCPU, error) {
		attempts++
		if attempts < 3 {
			return NodeCPU{}, errors.New("transient failure")
		}
		return expected, nil
	})
	if err != nil {
		t.Fatalf("queryPrometheusWithRetry returned unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("queryPrometheusWithRetry attempted %d times, want 3", attempts)
	}
	if cpu != expected {
		t.Fatalf("queryPrometheusWithRetry returned %#v, want %#v", cpu, expected)
	}
}

func TestQueryPrometheusWithRetryReturnsLastError(t *testing.T) {
	attempts := 0
	queryErrs := []error{
		errors.New("first failure"),
		errors.New("second failure"),
		errors.New("last failure"),
	}

	cpu, err := queryPrometheusWithRetry("test query", 3, 0, func() (NodeCPU, error) {
		err := queryErrs[attempts]
		attempts++
		return NodeCPU{}, err
	})
	if err != queryErrs[2] {
		t.Fatalf("queryPrometheusWithRetry returned error %v, want %v", err, queryErrs[2])
	}
	if attempts != 3 {
		t.Fatalf("queryPrometheusWithRetry attempted %d times, want 3", attempts)
	}
	if cpu != (NodeCPU{}) {
		t.Fatalf("queryPrometheusWithRetry returned %#v, want zero NodeCPU", cpu)
	}
}
