import { renderHook } from '@testing-library/react';
import { useMapOrchestrator } from './useMapOrchestrator';
// Removed explicit vitest imports to use globals

describe('useMapOrchestrator', () => {
    const mockMapRef = {
        current: {
            getCanvas: vi.fn(() => ({ clientWidth: 800, clientHeight: 600 })),
            getBounds: vi.fn(() => ({ getNorth: () => 50, getSouth: () => 48, getEast: () => 10, getWest: () => 8 })),
            project: vi.fn(() => ({ x: 0, y: 0 })),
            unproject: vi.fn(() => ({ lat: 0, lng: 0 })),
            getZoom: vi.fn(() => 10),
            getCenter: vi.fn(() => ({ lat: 0, lng: 0 })),
            isEasing: vi.fn(() => false),
            easeTo: vi.fn(),
            on: vi.fn(),
            remove: vi.fn(),
            addSource: vi.fn(),
            addLayer: vi.fn(),
            getSource: vi.fn(),
            getLayer: vi.fn(),
            getMinZoom: vi.fn(() => 2),
        }
    } as any;

    const defaultProps = {
        mapRef: mockMapRef,
        telemetry: null,
        pois: [],
        narratorStatus: {},
        tripEvents: [],
        validEvents: [],
        poiDiscoveryTimes: new Map(),
        progress: 0,
        effectiveReplayMode: false,
        settlementCategories: [],
        beaconMaxTargets: 8,
        styleLoaded: false,
        fontsLoaded: true,
        accumulatedSettlements: { current: new Map() },
        accumulatedPois: { current: new Map() }
    };

    it('initializes with provided initialZoom and initialCenter', () => {
        const { result } = renderHook(() => useMapOrchestrator({
            ...defaultProps,
            initialZoom: 12,
            initialCenter: [10, 50]
        }));

        expect(result.current.frame.zoom).toBe(12);
        expect(result.current.frame.center).toEqual([10, 50]);
    });

    it('defaults to zoom 2 and center [0,0] if not provided', () => {
        const { result } = renderHook(() => useMapOrchestrator(defaultProps));

        expect(result.current.frame.zoom).toBe(2);
        expect(result.current.frame.center).toEqual([0, 0]);
    });
});
