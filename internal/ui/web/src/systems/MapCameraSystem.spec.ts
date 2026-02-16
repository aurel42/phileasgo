// Removed explicit vitest imports to use globals
import { MapCameraSystem } from './MapCameraSystem';
import type { MapContext, SystemState } from './types';

describe('MapCameraSystem', () => {
    let system: MapCameraSystem;
    let mockMap: any;

    beforeEach(() => {
        mockMap = {
            getBounds: vi.fn(() => ({
                getNorth: () => 50, getSouth: () => 48, getEast: () => 10, getWest: () => 8
            })),
            getCanvas: vi.fn(() => ({ clientWidth: 800, clientHeight: 600 })),
            project: vi.fn((coords) => ({ x: coords[0] * 10, y: coords[1] * 10 })),
            unproject: vi.fn((pos) => ({ lng: pos.x / 10, lat: pos.y / 10 })),
            cameraForBounds: vi.fn(() => ({ zoom: 10, center: { lng: 9, lat: 49 } })),
            getMinZoom: vi.fn(() => 2),
            easeTo: vi.fn(),
            isEasing: vi.fn(() => false),
            getCenter: vi.fn(() => ({ lat: 0, lng: 0 })),
            getZoom: vi.fn(() => 10),
            getSource: vi.fn(),
            getLayer: vi.fn(),
        };
        system = new MapCameraSystem({ current: mockMap } as any);
    });

    it('recalculates zoom using cameraForBounds when a mask is present', async () => {
        // 1. Setup mock fetch for the visibility mask
        const mockMask = {
            type: 'Feature',
            geometry: {
                type: 'Polygon',
                coordinates: [[[8, 48], [10, 48], [10, 50], [8, 50], [8, 48]]]
            }
        };

        vi.stubGlobal('fetch', vi.fn(() =>
            Promise.resolve({
                ok: true,
                json: () => Promise.resolve(mockMask)
            })
        ));

        const ctx: MapContext = {
            telemetry: { SimState: 'active', Latitude: 49, Longitude: 9, Heading: 0 } as any,
            pois: [],
            zoom: 5,
            center: [9, 49],
            mode: 'FLIGHT',
            progress: 0,
            validEvents: [],
            poiDiscoveryTimes: new Map(),
            totalTripTime: 0,
            firstEventTime: 0,
            narratorStatus: {},
            settlementCategories: [],
            beaconMaxTargets: 8,
            stabilizedReplay: false,
            mapWidth: 800,
            mapHeight: 600,
            styleLoaded: true,
            fontsLoaded: true
        };

        const state: SystemState = {
            frame: { center: [9, 49], zoom: 5, offset: [0, 0], maskPath: '', aircraftX: 0, aircraftY: 0, agl: 0, heading: 0, labels: [], bearingLine: null },
            accumulatedSettlements: new Map(),
            accumulatedPois: new Map()
        };

        // Trigger first update to fire fetch. needsRecenter will be true because lockedCenter is null.
        system.update(0.016, ctx, state);

        // Wait for fetch promise and a bit more for background processing
        await new Promise(resolve => setTimeout(resolve, 50));

        // Reset mock to track call from second update
        mockMap.cameraForBounds.mockClear();

        // Trigger second update with a movement to force needsRecenter via smart offset
        const ctx2 = { ...ctx };
        ctx2.telemetry = { ...ctx.telemetry, Latitude: 10, Longitude: 10 } as any;

        system.update(0.016, ctx2, state);

        expect(mockMap.cameraForBounds).toHaveBeenCalled();
        expect(state.frame.zoom).toBe(10); // From mock cameraForBounds
    });

    it('includes active POIs in the framing calculation', async () => {
        const ctx: MapContext = {
            telemetry: { SimState: 'active', Latitude: 49, Longitude: 9, Heading: 0 } as any,
            pois: [],
            zoom: 5,
            center: [9, 49],
            mode: 'FLIGHT',
            progress: 0,
            validEvents: [],
            poiDiscoveryTimes: new Map(),
            totalTripTime: 0,
            firstEventTime: 0,
            narratorStatus: {
                current_poi: { lat: 49.5, lon: 9.5 }
            },
            settlementCategories: [],
            beaconMaxTargets: 8,
            stabilizedReplay: false,
            mapWidth: 800,
            mapHeight: 600,
            styleLoaded: true,
            fontsLoaded: true
        };

        const state: SystemState = {
            frame: { center: [9, 49], zoom: 5, offset: [0, 0], maskPath: '', aircraftX: 0, aircraftY: 0, agl: 0, heading: 0, labels: [], bearingLine: null },
            accumulatedSettlements: new Map(),
            accumulatedPois: new Map()
        };

        system.update(0.016, ctx, state);

        // Verify cameraForBounds was called. We can't easily check the arguments 
        // without more complex mock matching, but visibility of the call is enough here.
        expect(mockMap.cameraForBounds).toHaveBeenCalled();
    });

    it('handles REPLAY mode and follows map easing', () => {
        mockMap.isEasing.mockReturnValue(true);
        mockMap.getCenter.mockReturnValue({ lng: 12, lat: 52 });
        mockMap.getZoom.mockReturnValue(8);

        const ctx: MapContext = {
            mode: 'REPLAY',
            telemetry: { SimState: 'active' } as any,
            center: [9, 49],
            zoom: 5,
            pois: [],
            progress: 0.5,
            validEvents: [
                { timestamp: '2024-01-01T12:00:00Z', position: [50, 10], heading: 0 },
                { timestamp: '2024-01-01T12:01:00Z', position: [51, 11], heading: 10 }
            ],
            poiDiscoveryTimes: new Map(),
            totalTripTime: 60000,
            firstEventTime: 0,
            narratorStatus: {},
            settlementCategories: [],
            beaconMaxTargets: 8,
            stabilizedReplay: false,
            mapWidth: 800,
            mapHeight: 600,
            styleLoaded: true,
            fontsLoaded: true
        };

        const state: SystemState = {
            frame: { center: [9, 49], zoom: 5, offset: [0, 0], maskPath: '', aircraftX: 0, aircraftY: 0, agl: 0, heading: 0, labels: [], bearingLine: null },
            accumulatedSettlements: new Map(),
            accumulatedPois: new Map()
        };

        system.update(0.016, ctx, state);

        expect(state.frame.center).toEqual([12, 52]);
        expect(state.frame.zoom).toBe(8);
    });

    it('handles transition state in recalculateCamera', () => {
        const ctx: MapContext = {
            telemetry: { SimState: 'inactive', Latitude: 49, Longitude: 9, Heading: 0 } as any,
            mode: 'TRANSITION',
            zoom: 5,
            center: [9, 49],
            pois: [],
            progress: 0,
            validEvents: [],
            poiDiscoveryTimes: new Map(),
            totalTripTime: 0,
            firstEventTime: 0,
            narratorStatus: {},
            settlementCategories: [],
            beaconMaxTargets: 8,
            stabilizedReplay: false,
            mapWidth: 800,
            mapHeight: 600,
            styleLoaded: true,
            fontsLoaded: true
        };

        const state: SystemState = {
            frame: { center: [9, 49], zoom: 5, offset: [0, 0], maskPath: '', aircraftX: 0, aircraftY: 0, agl: 0, heading: 0, labels: [], bearingLine: null },
            accumulatedSettlements: new Map(),
            accumulatedPois: new Map()
        };

        // Initialize system state with 'active'
        if (ctx.telemetry) ctx.telemetry.SimState = 'active';
        system.update(0.016, ctx, state);

        // Reset mock
        mockMap.cameraForBounds.mockClear();

        // 2. Trigger Transition to 'inactive'
        // We also need at least one feature to trigger cameraForBounds
        if (ctx.telemetry) ctx.telemetry.SimState = 'inactive';
        ctx.mode = 'TRANSITION';
        ctx.narratorStatus = { current_poi: { lat: 49.5, lon: 9.5 } };

        system.update(0.016, ctx, state);

        expect(mockMap.cameraForBounds).toHaveBeenCalled();
    });

    it('handles preparing_poi in calculateAutoZoom', () => {
        const ctx: MapContext = {
            telemetry: { SimState: 'active', Latitude: 49, Longitude: 9, Heading: 0 } as any,
            mode: 'FLIGHT',
            zoom: 5,
            center: [9, 49],
            pois: [],
            progress: 0,
            validEvents: [],
            poiDiscoveryTimes: new Map(),
            totalTripTime: 0,
            firstEventTime: 0,
            narratorStatus: {
                preparing_poi: { lat: 48.5, lon: 8.5 }
            },
            settlementCategories: [],
            beaconMaxTargets: 8,
            stabilizedReplay: false,
            mapWidth: 800,
            mapHeight: 600,
            styleLoaded: true,
            fontsLoaded: true
        };

        const state: SystemState = {
            frame: { center: [9, 49], zoom: 5, offset: [0, 0], maskPath: '', aircraftX: 0, aircraftY: 0, agl: 0, heading: 0, labels: [], bearingLine: null },
            accumulatedSettlements: new Map(),
            accumulatedPois: new Map()
        };

        system.update(0.016, ctx, state);

        expect(mockMap.cameraForBounds).toHaveBeenCalled();
    });
});
