import type { POI } from '../hooks/usePOIs';

export const isPOIVisible = (poi: POI, minScore: number): boolean => {
    // 1. Score Check
    if (poi.score > minScore) {
        return true;
    }

    // 2. Played Check (Always visible if played)
    if (poi.last_played && poi.last_played !== "0001-01-01T00:00:00Z") {
        return true;
    }

    return false;
};

export const getPOIDisplayName = (poi: { name_user?: string; name_en?: string; name_local?: string }): string => {
    if (poi.name_user) return poi.name_user;
    if (poi.name_en) return poi.name_en;
    return poi.name_local || 'Unknown';
};
