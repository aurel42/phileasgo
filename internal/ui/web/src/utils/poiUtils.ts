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
