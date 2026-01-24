package model

import "testing"

func TestPOIDisplayName(t *testing.T) {
	tests := []struct {
		name string
		p    POI
		want string
	}{
		{
			name: "All names empty",
			p:    POI{WikidataID: "Q123"},
			want: "Q123",
		},
		{
			name: "Only local name",
			p:    POI{WikidataID: "Q123", NameLocal: "Local"},
			want: "Local",
		},
		{
			name: "English name overrides local",
			p:    POI{WikidataID: "Q123", NameLocal: "Local", NameEn: "English"},
			want: "English",
		},
		{
			name: "User name overrides all",
			p:    POI{WikidataID: "Q123", NameLocal: "Local", NameEn: "English", NameUser: "UserChoice"},
			want: "UserChoice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.DisplayName(); got != tt.want {
				t.Errorf("POI.DisplayName() = %v, want %v", got, tt.want)
			}
		})
	}
}
