import { useState, useEffect } from 'react';

export interface POI {
    wikidata_id: string;
    source: string;
    category: string;
    specific_category?: string;
    icon: string;
    lat: number;
    lon: number;
    name_en: string;
    name_local: string;
    name_user: string;
    wp_url: string;
    score: number;
    score_details: string;
    last_played: string; // ISO timestamp
    thumbnail_url?: string;
    is_msfs_poi?: boolean;
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
