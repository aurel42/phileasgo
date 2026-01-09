package probe

import (
	"context"
	"errors"
	"testing"
)

func TestRun(t *testing.T) {
	probes := []Probe{
		{
			Name: "Success Probe",
			Check: func(ctx context.Context) error {
				return nil
			},
			Critical: true,
		},
		{
			Name: "Failure Probe (Non-Critical)",
			Check: func(ctx context.Context) error {
				return errors.New("minor issue")
			},
			Critical: false,
		},
	}

	results := Run(context.Background(), probes)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if results[0].Error != nil {
		t.Errorf("Expected success probe to pass, got error: %v", results[0].Error)
	}

	if results[1].Error == nil {
		t.Error("Expected failure probe to fail, got nil")
	}
}

func TestAnalyzeResults(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		wantErr bool
	}{
		{
			name: "All Pass",
			results: []Result{
				{Probe: Probe{Name: "P1", Critical: true}, Error: nil},
			},
			wantErr: false,
		},
		{
			name: "Critical Failure",
			results: []Result{
				{Probe: Probe{Name: "P1", Critical: true}, Error: errors.New("fail")},
			},
			wantErr: true,
		},
		{
			name: "Non-Critical Failure",
			results: []Result{
				{Probe: Probe{Name: "P1", Critical: false}, Error: errors.New("fail")},
			},
			wantErr: false,
		},
		{
			name: "Mixed Failure",
			results: []Result{
				{Probe: Probe{Name: "P1", Critical: false}, Error: errors.New("fail")},
				{Probe: Probe{Name: "P2", Critical: true}, Error: errors.New("fail")},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AnalyzeResults(tt.results)
			if (err != nil) != tt.wantErr {
				t.Errorf("AnalyzeResults() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
