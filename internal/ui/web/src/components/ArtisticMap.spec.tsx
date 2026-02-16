import { render, act } from '@testing-library/react';
import { ArtisticMap } from './ArtisticMap';

// Mock global fetch to prevent network calls (defaulting to localhost:3000 in happy-dom)
// We use vi.stubGlobal to ensure it patches window.fetch in the happy-dom environment
const fetchMock = vi.fn(() =>
    Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ lat: 48.8566, lon: 2.3522 }), // Valid mock response
    })
);
vi.stubGlobal('fetch', fetchMock);

// Mock dependencies
vi.mock('maplibre-gl', () => {
    const MapMock = vi.fn().mockImplementation(() => ({
        on: vi.fn(),
        remove: vi.fn(),
        addControl: vi.fn(),
        getCanvas: vi.fn(() => ({ style: {} })),
        getContainer: vi.fn(() => ({ appendChild: vi.fn() })),
        project: vi.fn(() => ({ x: 0, y: 0 })),
        unproject: vi.fn(() => ({ lat: 0, lng: 0 })),
        getZoom: vi.fn(() => 10),
        getCenter: vi.fn(() => ({ lat: 0, lng: 0 })),
        getPitch: vi.fn(() => 0),
        getBearing: vi.fn(() => 0),
        getBounds: vi.fn(() => ({ getNorth: () => 0, getSouth: () => 0, getEast: () => 0, getWest: () => 0 })),
    }));
    return {
        default: {
            Map: MapMock,
        },
        Map: MapMock,
    };
});

vi.mock('@tanstack/react-query', () => ({
    useQueryClient: vi.fn(() => ({
        getQueryData: vi.fn(),
    })),
}));

vi.mock('../hooks/useTripEvents', () => ({
    useTripEvents: vi.fn(() => ({ data: [] })),
}));

vi.mock('../hooks/useNarrator', () => ({
    useNarrator: vi.fn(() => ({ status: {} })),
}));

vi.mock('../services/labelService', () => ({
    labelService: {
        fetchLabels: vi.fn().mockResolvedValue([]),
    }
}));

import { labelService } from '../services/labelService';

describe('ArtisticMap', () => {
    const defaultProps = {
        center: [0, 0] as [number, number],
        zoom: 10,
        telemetry: { SimState: 'active', Latitude: 0, Longitude: 0, Heading: 0, Altitude: 1000, AltitudeAGL: 1000 } as any, // Active state needed for non-idle logic if any
        pois: [],
        settlementTier: 1,
        settlementCategories: [],
        onPOISelect: vi.fn(),
        onMapClick: vi.fn(),
        paperOpacityFog: 0.7,
        paperOpacityClear: 0.1,
        parchmentSaturation: 1.0,
        mapFactory: vi.fn(() => ({
            on: vi.fn((event, cb) => {
                if (event === 'load' && typeof cb === 'function') {
                    cb();
                }
            }),
            remove: vi.fn(),
            addControl: vi.fn(),
            getCanvas: vi.fn(() => ({ style: {}, clientWidth: 800, clientHeight: 600 })), // Valid dims
            getContainer: vi.fn(() => ({ appendChild: vi.fn() })),
            project: vi.fn(() => ({ x: 0, y: 0 })),
            unproject: vi.fn(() => ({ lat: 0, lng: 0 })),
            getZoom: vi.fn(() => 10),
            getCenter: vi.fn(() => ({ lat: 0, lng: 0, lngLat: { lng: 0, lat: 0 } })),
            getPitch: vi.fn(() => 0),
            getBearing: vi.fn(() => 0),
            getBounds: vi.fn(() => ({ getNorth: () => 10, getSouth: () => -10, getEast: () => 10, getWest: () => -10 })),
            getMinZoom: vi.fn(() => 0),
            cameraForBounds: vi.fn(),
            easeTo: vi.fn(),
            isEasing: vi.fn(() => false),
            addSource: vi.fn(),
            addLayer: vi.fn(),
            getSource: vi.fn(),
            getLayer: vi.fn(),
            setPaintProperty: vi.fn(),
        })),
    };

    it('renders without crashing', () => {
        const { container } = render(<ArtisticMap {...defaultProps} />);
        expect(container).toBeDefined();
    });

    it('renders with aircraft customization props', () => {
        const { container } = render(
            <ArtisticMap
                {...defaultProps}
                aircraftIcon="jet"
                aircraftSize={48}
                aircraftColorMain="#ff0000"
            />
        );
        expect(container).toBeDefined();
    });

    it('calls label service to sync labels', async () => {
        vi.useFakeTimers();
        render(<ArtisticMap {...defaultProps} />);

        // Fast-forward time to trigger interval (2000ms fetch throttler + 16ms tick)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(3000);
        });

        expect(labelService.fetchLabels).toHaveBeenCalled();
        vi.useRealTimers();
    });
});
