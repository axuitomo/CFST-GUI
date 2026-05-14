package probecore

import (
	"reflect"
	"testing"
)

func TestPortSummaryUsesActualPortGroups(t *testing.T) {
	tests := []struct {
		name        string
		entries     []string
		sourcePorts map[string]int
		wantCurrent int
		wantGrouped []int
		wantSource  []int
	}{
		{
			name:        "global only",
			entries:     []string{"1.1.1.1", "8.8.8.8"},
			wantCurrent: 443,
			wantGrouped: []int{443},
			wantSource:  []int{},
		},
		{
			name:        "single source port only",
			entries:     []string{"1.1.1.1"},
			sourcePorts: map[string]int{"1.1.1.1": 2053},
			wantCurrent: 2053,
			wantGrouped: []int{2053},
			wantSource:  []int{2053},
		},
		{
			name:        "single source port plus global fallback",
			entries:     []string{"1.1.1.1", "8.8.8.8"},
			sourcePorts: map[string]int{"1.1.1.1": 2053},
			wantCurrent: 0,
			wantGrouped: []int{443, 2053},
			wantSource:  []int{2053},
		},
		{
			name:        "multiple source ports",
			entries:     []string{"1.1.1.1", "1.1.1.2"},
			sourcePorts: map[string]int{"1.1.1.1": 2053, "1.1.1.2": 8443},
			wantCurrent: 0,
			wantGrouped: []int{2053, 8443},
			wantSource:  []int{2053, 8443},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := PortSummary(tt.entries, tt.sourcePorts, 443)
			if got := summary["current_test_port"]; got != tt.wantCurrent {
				t.Fatalf("current_test_port = %#v, want %d", got, tt.wantCurrent)
			}
			if got := summaryIntSlice(summary, "grouped_ports"); !reflect.DeepEqual(got, tt.wantGrouped) {
				t.Fatalf("grouped_ports = %#v, want %#v", got, tt.wantGrouped)
			}
			if got := summaryIntSlice(summary, "source_port_values"); !reflect.DeepEqual(got, tt.wantSource) {
				t.Fatalf("source_port_values = %#v, want %#v", got, tt.wantSource)
			}
		})
	}
}

func summaryIntSlice(summary map[string]any, key string) []int {
	values, ok := summary[key].([]int)
	if !ok {
		return nil
	}
	return values
}
