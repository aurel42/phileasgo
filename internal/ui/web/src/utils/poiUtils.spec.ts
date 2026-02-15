import { isPOIVisible, getPOIDisplayName } from './poiUtils';
import type { POI } from '../hooks/usePOIs';

describe('poiUtils', () => {
    describe('isPOIVisible', () => {
        const mockPOI = (score: number, lastPlayed?: string): POI => ({
            wikidata_id: 'Q1',
            name_en: 'Test POI',
            lat: 0,
            lon: 0,
            score,
            last_played: lastPlayed || "0001-01-01T00:00:00Z"
        } as POI);

        it('returns true if score is above threshold', () => {
            expect(isPOIVisible(mockPOI(15), 10)).toBe(true);
        });

        it('returns false if score is at or below threshold', () => {
            expect(isPOIVisible(mockPOI(10), 10)).toBe(false);
            expect(isPOIVisible(mockPOI(5), 10)).toBe(false);
        });

        it('returns true if POI was played regardless of score', () => {
            expect(isPOIVisible(mockPOI(0, '2024-01-01T12:00:00Z'), 10)).toBe(true);
        });
    });

    describe('getPOIDisplayName', () => {
        it('prefers name_user', () => {
            const poi = { name_user: 'User Name', name_en: 'English Name', name_local: 'Local Name' };
            expect(getPOIDisplayName(poi)).toBe('User Name');
        });

        it('falls back to name_en', () => {
            const poi = { name_en: 'English Name', name_local: 'Local Name' };
            expect(getPOIDisplayName(poi)).toBe('English Name');
        });

        it('falls back to name_local', () => {
            const poi = { name_local: 'Local Name' };
            expect(getPOIDisplayName(poi)).toBe('Local Name');
        });

        it('returns "Unknown" if no name is available', () => {
            expect(getPOIDisplayName({})).toBe('Unknown');
        });
    });
});
