package version

import (
	"testing"
)

func TestVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantSet bool
	}{
		{
			name:    "VersionConstant",
			version: Version,
			wantSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (tt.version != "") != tt.wantSet {
				t.Errorf("Version = %q, wantSet %v", tt.version, tt.wantSet)
			}
		})
	}
}
