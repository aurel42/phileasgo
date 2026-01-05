export interface AudioStatus {
    is_playing: boolean;
    is_paused: boolean;
    is_user_paused: boolean;
    volume: number;
}

export interface NarratorStatus {
    active: boolean;
    narrated_count: number;
    stats: Record<string, number | boolean>;
}
