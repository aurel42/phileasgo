import { useState, useEffect } from 'react';

export interface POI {
    wikidata_id: string;
    badges?: string[]; // "deferred", "msfs", etc.
    source: string;
    category: string;
    specific_category?: string;
    icon: string;
    icon_artistic?: string;
    lat: number;
    lon: number;
    name_en: string;
    name_local: string;
    name_user: string;
    wp_url: string;
    score: number;
    score_details: string;
    is_visible?: boolean;
    is_deferred?: boolean;
    visibility?: number;
    has_balloon?: boolean;
    last_played: string; // ISO timestamp
    is_on_cooldown?: boolean;
    thumbnail_url?: string;
    is_msfs_poi?: boolean;
    narration_strategy?: string;
    los_status?: number; // 0=unknown, 1=visible, 2=blocked
    size?: string; // S, M, L, XL
    beacon_color?: string;
    last_time?: string; // User-requested sorting field, maps to last_played or similar
}

export function useTrackedPOIs() {
    const [pois, setPois] = useState<POI[]>([]);

    useEffect(() => {
        const fetchPOIs = async () => {
            try {
                const response = await fetch('/api/pois/tracked');
                if (!response.ok) return;
                const data = await response.json();
                setPois(data);
            } catch (error) {
                console.error("Failed to fetch POIs", error);
            }
        };

        // Fetch immediately
        fetchPOIs();

        // Poll every 5 seconds
        const interval = setInterval(fetchPOIs, 5000);
        return () => clearInterval(interval);
    }, []);

    return pois;
}
