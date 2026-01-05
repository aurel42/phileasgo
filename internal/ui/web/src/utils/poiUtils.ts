import type { POI } from '../hooks/usePOIs';

export const isPOIVisible = (poi: POI, minScore: number): boolean => {
    // 1. Score Check
    if (poi.score > minScore) {
        return true;
    }

    // 2. Recent Play Check (1 hour)
    if (poi.last_played && poi.last_played !== "0001-01-01T00:00:00Z") {
        const played = new Date(poi.last_played);
        const now = new Date();
        const diffMs = now.getTime() - played.getTime();
        const oneHourMs = 60 * 60 * 1000;
        if (diffMs < oneHourMs) {
            return true;
        }
    }

    return false;
};
