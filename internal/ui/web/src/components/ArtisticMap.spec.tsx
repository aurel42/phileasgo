import { render } from '@testing-library/react';
import { ArtisticMap } from './ArtisticMap';

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

describe('ArtisticMap', () => {
    const defaultProps = {
        center: [0, 0] as [number, number],
        zoom: 10,
        telemetry: null,
        pois: [],
        settlementTier: 1,
        settlementCategories: [],
        onPOISelect: vi.fn(),
        onMapClick: vi.fn(),
        paperOpacityFog: 0.7,
        paperOpacityClear: 0.1,
        parchmentSaturation: 1.0,
        mapFactory: vi.fn(() => ({
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
});
