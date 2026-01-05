export interface POI {
    wikidata_id: string;
    source: string;
    category: string;
    icon: string;
    lat: number;
    lon: number;
    sitelinks: number;
    name_en: string;
    name_local: string;
    name_user: string;
    score: number;
    is_visible: boolean;
}

export interface NarratorStatusResponse {
    active: boolean;
    playback_status: 'idle' | 'preparing' | 'playing' | 'paused';
    current_poi?: POI;
    current_title?: string;
    narrated_count: number;
    stats: Record<string, any>;
}
