import { renderHook } from '@testing-library/react';
import { useArtisticMapHeartbeat } from './useArtisticMapHeartbeat';
import { labelService } from '../services/labelService';

vi.mock('../services/labelService', () => ({
    labelService: {
        fetchLabels: vi.fn().mockResolvedValue([]),
    },
}));

describe('useArtisticMapHeartbeat', () => {
    const mockMap = {
        current: {
            getBounds: vi.fn(() => ({
                getNorth: () => 0, getSouth: () => 0, getEast: () => 0, getWest: () => 0,
                contains: vi.fn(() => true)
            })),
            getCanvas: vi.fn(() => ({ clientWidth: 1000, clientHeight: 1000 })),
            project: vi.fn(() => ({ x: 0, y: 0 })),
            getSource: vi.fn(() => ({ setData: vi.fn() })),
            getLayer: vi.fn(() => true),
            setPaintProperty: vi.fn(),
            easeTo: vi.fn(),
            isEasing: vi.fn(() => false),
            getZoom: vi.fn(() => 10),
            getCenter: vi.fn(() => ({ lng: 0, lat: 0 })),
        }
    };

    beforeEach(() => {
        vi.clearAllMocks();
        vi.useFakeTimers();
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ geometry: null })
        }));
    });

    it('skips label computation and sync when stabilizedReplay is true', async () => {
        renderHook(() => useArtisticMapHeartbeat(
            mockMap as any,
            { SimState: 'active' } as any,
            [], 10, [0, 0], true, 0.5, [], new Map(), 1000, 0,
            {}, {
                resetCache: vi.fn(),
                clear: vi.fn(),
                register: vi.fn(),
                forget: vi.fn(),
                compute: vi.fn(() => []),
            } as any, true, vi.fn(), [], 8, true // stabilizedReplay = true
        ));

        // Advance timers to trigger the interval
        await vi.advanceTimersByTimeAsync(2100);

        // Verification: If it returned early, fetchLabels shouldn't be called more than once (initial tick)
        // Wait, the initial tick might still run before we can effectively block it?
        // Actually, tick() is called immediately in useEffect.

        // Let's check how many times fetchLabels was called.
        // If stabilizedReplay is true, the initial tick should also return early.
        expect(labelService.fetchLabels).not.toHaveBeenCalled();
    });

    it('executes label computation when stabilizedReplay is false', async () => {
        renderHook(() => useArtisticMapHeartbeat(
            mockMap as any,
            { SimState: 'active' } as any,
            [], 10, [0, 0], true, 0.5, [], new Map(), 1000, 0,
            {}, {
                resetCache: vi.fn(),
                clear: vi.fn(),
                register: vi.fn(),
                forget: vi.fn(),
                compute: vi.fn(() => []),
            } as any, true, vi.fn(), [], 8, false // stabilizedReplay = false
        ));

        await vi.advanceTimersByTimeAsync(2100);

        // Should have been called at least once (initial or interval)
        expect(labelService.fetchLabels).toHaveBeenCalled();
    });
});
