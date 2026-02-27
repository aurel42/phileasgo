
export interface POI {
    wikidata_id: string;
    id?: string; // Some portions of code use id
    name_en: string;
    name_local: string;
    name_user: string;
    lat: number;
    lon: number;
    score: number;
    category?: string;
    icon?: string;
    is_hidden_feature?: boolean;
    is_on_cooldown?: boolean;
    beacon_color?: string;
    badges?: string[];
    visibility?: number;
    los_status?: number;
    last_played?: string;
    is_msfs_poi?: boolean;
    has_balloon?: boolean;
    size?: 'S' | 'M' | 'L' | 'XL';
}

export interface Settlement {
    id: string;
    name: string;
    city_name?: string;
    lat: number;
    lon: number;
    pop: number;
    population?: number;
    tier?: string;
}

export interface SettlementData {
    labels: Settlement[];
}

export interface Telemetry {
    Latitude: number;
    Longitude: number;
    Heading: number;
    Altitude: number;
    AltitudeMSL: number;
    AltitudeAGL: number;
    GroundSpeed: number;
    OnGround: boolean;
    SimState: string;
    Valid: boolean;
    FlightStage: number;
}

export interface NarratorStatus {
    current_poi?: POI;
    preparing_poi?: POI;
    is_user_paused?: boolean;
    narration_frequency?: number;
    text_length?: number;
}

export interface AircraftConfig {
    aircraft_icon?: string;
    aircraft_size?: number;
    aircraft_color_main?: string;
    aircraft_color_accent?: string;
    narration_frequency?: number;
    text_length?: number;
    filter_mode?: string;
    min_poi_score?: number;
    target_poi_count?: number;
}

export interface RegionalCategory {
    qid: string;
    name: string;
    category?: string;
}

export interface Geography {
    city?: string;
    country?: string;
    region?: string;
    country_code?: string;
    city_country_code?: string;
    city_region?: string;
    city_country?: string;
}

export interface ApiStats {
    providers?: Record<string, {
        api_success: number;
        api_errors: number;
    }>;
    diagnostics?: {
        name: string;
        memory_mb: number;
        cpu_sec: number;
    }[];
    tracking?: {
        active_pois: number;
    };
}
