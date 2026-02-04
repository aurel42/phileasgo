export interface POI {
    wikidata_id: string;
    source: string;
    category: string;
    specific_category?: string;
    icon: string;
    lat: number;
    lon: number;
    sitelinks: number;
    name_en: string;
    name_local: string;
    name_user: string;
    wp_url: string;
    score: number;
    score_details: string;
    last_played: string;
    thumbnail_url?: string;
    is_visible: boolean;
    is_msfs_poi?: boolean;
}

export interface NarratorStatusResponse {
    active: boolean;
    playback_status: 'idle' | 'preparing' | 'playing' | 'paused';
    current_poi?: POI;
    preparing_poi?: POI;
    current_title?: string;
    current_type?: string;
    current_image_path?: string;
    display_title?: string;
    display_thumbnail?: string;
    narrated_count: number;
    stats: Record<string, unknown>;
    narration_frequency?: number;
    text_length?: number;
    show_info_panel?: boolean;
}
