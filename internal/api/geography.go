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
	City            string `json:"city"`
	Region          string `json:"region"`
	Country         string `json:"country"`
	LegalCountry    string `json:"legal_country"`
	CityRegion      string `json:"city_region"`
	CityCountry     string `json:"city_country"`
	CountryCode     string `json:"country_code"`
	CityCountryCode string `json:"city_country_code"`
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
		if region == "" && !isNumeric(loc.Admin1Code) {
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
		City:            loc.CityName,
		Region:          region,
		Country:         country,
		LegalCountry:    loc.CountryName,
		CityRegion:      loc.CityAdmin1Name,
		CityCountry:     loc.CityCountryName,
		CountryCode:     loc.CountryCode,
		CityCountryCode: loc.CityCountryCode,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode geography response", "error", err)
	}
}
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (h *GeographyHandler) HandleRandomStart(w http.ResponseWriter, r *http.Request) {
	lat, lon, err := h.geoSvc.GetRandomMajorCity()
	if err != nil {
		http.Error(w, "Failed to find random city", http.StatusInternalServerError)
		return
	}

	resp := struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	}{
		Lat: lat,
		Lon: lon,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode random start response", "error", err)
	}
}
