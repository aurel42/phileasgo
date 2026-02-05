package config

// Persistent state keys (Registry)
const (
	KeyMinPOIScore                 = "min_poi_score"
	KeyFilterMode                  = "filter_mode"
	KeyTargetPOICount              = "target_poi_count"
	KeyNarrationFrequency          = "narration_frequency"
	KeyTextLength                  = "text_length"
	KeyUnits                       = "units"            // Prompt template units (imperial/hybrid/metric)
	KeyRangeRingUnits              = "range_ring_units" // Map display units (km/nm)
	KeyShowCacheLayer              = "show_cache_layer"
	KeyShowVisibility              = "show_visibility_layer"
	KeySimSource                   = "sim_source"
	KeyTeleportDistance            = "teleport_distance"
	KeyMockLat                     = "mock_start_lat"
	KeyMockLon                     = "mock_start_lon"
	KeyMockAlt                     = "mock_start_alt"
	KeyMockHeading                 = "mock_start_heading"
	KeyMockDurParked               = "mock_duration_parked"
	KeyMockDurTaxi                 = "mock_duration_taxi"
	KeyMockDurHold                 = "mock_duration_hold"
	KeyStyleLibrary                = "style_library"
	KeyActiveStyle                 = "active_style"
	KeySecretWordLibrary           = "secret_word_library"
	KeyActiveSecretWord            = "active_secret_word"
	KeyTargetLanguageLibrary       = "target_language_library"
	KeyActiveTargetLanguage        = "active_target_language"
	KeyDeferralProximityBoostPower = "scorer.deferral_proximity_boost_power"
	KeyTwoPassScriptGeneration     = "narrator.two_pass_script_generation"

	// Beacon settings
	KeyBeaconEnabled           = "beacon.enabled"
	KeyBeaconFormationEnabled  = "beacon.formation_enabled"
	KeyBeaconFormationDistance = "beacon.formation_distance"
	KeyBeaconFormationCount    = "beacon.formation_count"
	KeyBeaconMinSpawnAltitude  = "beacon.min_spawn_altitude"
	KeyBeaconAltitudeFloor     = "beacon.altitude_floor"
	KeyBeaconSinkDistanceFar   = "beacon.target_sink_distance_far"
	KeyBeaconSinkDistanceClose = "beacon.target_sink_distance_close"
	KeyBeaconTargetFloorAGL    = "beacon.target_floor_agl"
	KeyBeaconMaxTargets        = "beacon.max_targets"
)
