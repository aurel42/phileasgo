import { renderHook } from '@testing-library/react';
import { useArtisticMapReplay } from './useArtisticMapReplay';
import { useTripEvents } from '../hooks/useTripEvents';
import { useNarrator } from '../hooks/useNarrator';

vi.mock('../hooks/useTripEvents', () => ({
    useTripEvents: vi.fn(),
}));

vi.mock('../hooks/useNarrator', () => ({
    useNarrator: vi.fn(),
}));

vi.mock('@tanstack/react-query', () => ({
    useQueryClient: vi.fn(() => ({
        invalidateQueries: vi.fn(),
    })),
}));

describe('useArtisticMapReplay', () => {
    it('calculates firstEventTime correctly from tripEvents', () => {
        const mockEvents = [
            { timestamp: '2026-02-15T12:00:00Z', lat: 0, lon: 0 },
            { timestamp: '2026-02-15T12:05:00Z', lat: 1, lon: 1 },
        ];
        (useTripEvents as any).mockReturnValue({ data: mockEvents });
        (useNarrator as any).mockReturnValue({ status: {} });

        const { result } = renderHook(() => useArtisticMapReplay({}, {} as any));

        const expectedTime = new Date('2026-02-15T12:00:00Z').getTime();
        expect(result.current.firstEventTime).toBe(expectedTime);
    });

    it('returns 0 for firstEventTime if no events', () => {
        (useTripEvents as any).mockReturnValue({ data: [] });
        (useNarrator as any).mockReturnValue({ status: {} });

        const { result } = renderHook(() => useArtisticMapReplay({}, {} as any));

        expect(result.current.firstEventTime).toBe(0);
    });
});
