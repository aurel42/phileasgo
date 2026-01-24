package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"phileasgo/pkg/geo"
)

type GeographyHandler struct {
	geoSvc *geo.Service
}

func NewGeographyHandler(geoSvc *geo.Service) *GeographyHandler {
	return &GeographyHandler{geoSvc: geoSvc}
}

type GeographyResponse struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
}

func (h *GeographyHandler) Handle(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")

	lat, err1 := strconv.ParseFloat(latStr, 64)
	lon, err2 := strconv.ParseFloat(lonStr, 64)

	if err1 != nil || err2 != nil {
		http.Error(w, "Invalid lat/lon", http.StatusBadRequest)
		return
	}

	loc := h.geoSvc.GetLocation(lat, lon)

	// Determine display values based on zone
	var country, region string
	switch loc.Zone {
	case geo.ZoneLand:
		country = loc.CountryName
		if country == "" {
			country = loc.CountryCode // Fallback
		}
		region = loc.Admin1Name
		if region == "" {
			region = loc.Admin1Code
		}
	case geo.ZoneTerritorial:
		country = loc.CountryName
		if country == "" {
			country = loc.CountryCode
		}
		region = "Territorial Waters"
	case geo.ZoneEEZ:
		country = loc.CountryName
		if country == "" {
			country = loc.CountryCode
		}
		region = "Exclusive Economic Zone"
	default: // ZoneInternational or empty
		country = "International Waters"
		region = ""
	}

	resp := GeographyResponse{
		City:    loc.CityName,
		Region:  region,
		Country: country,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode geography response", "error", err)
	}
}
