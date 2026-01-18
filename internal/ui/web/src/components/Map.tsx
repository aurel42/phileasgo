import { useEffect, Fragment } from 'react';
import { MapContainer, TileLayer, Marker, useMap, Circle } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import { useTelemetry } from '../hooks/useTelemetry';
import { AircraftMarker } from './AircraftMarker';
import { CacheLayer } from './CacheLayer';
import { VisibilityLayer } from './VisibilityLayer';
import { SmartMarkerLayer } from './SmartMarkerLayer';
import type { POI } from '../hooks/usePOIs';
import { useNarrator } from '../hooks/useNarrator';
import { useMapEvents } from 'react-leaflet';



// Zoom level calculations:
// At zoom 13: ~10km visible (min area)
// At zoom 8: ~300km visible (max area)
// We'll use zoom 8-13 to cover 10-200km range
const MIN_ZOOM = 8;  // ~200km area
const MAX_ZOOM = 13; // ~10km area
const DEFAULT_ZOOM = 10;

// Helper to recenter map with heading-based offset
// Offsets the map center forward of the aircraft by 25% of the smaller map dimension
const Recenter = ({ mapCenter, heading }: { mapCenter: [number, number]; heading: number }) => {
    const map = useMap();
    useEffect(() => {
        const [lat, lon] = mapCenter;

        // Calculate offset in pixels (25% of smaller map dimension)
        const mapSize = map.getSize();
        const offsetPx = Math.min(mapSize.x, mapSize.y) * 0.25;

        // Convert heading to radians
        const hdgRad = heading * (Math.PI / 180);

        // Calculate pixel offset from aircraft position
        // Forward direction in map coordinates (y is inverted)
        const dx = offsetPx * Math.sin(hdgRad);
        const dy = -offsetPx * Math.cos(hdgRad);

        // Project aircraft position to screen coordinates
        const aircraftPoint = map.project([lat, lon], map.getZoom());

        // Add offset to get new center point
        const centerPoint = L.point(aircraftPoint.x + dx, aircraftPoint.y + dy);

        // Unproject back to lat/lon
        const centerLatLng = map.unproject(centerPoint, map.getZoom());

        map.panTo(centerLatLng, { animate: true, duration: 0.1 });
    }, [mapCenter, heading, map]);
    return null;
};

const AircraftPaneSetup = () => {
    const map = useMap();
    useEffect(() => {
        if (!map.getPane('aircraftPane')) {
            map.createPane('aircraftPane');
            const pane = map.getPane('aircraftPane');
            if (pane) pane.style.zIndex = '2000';
        }
    }, [map]);
    return null;
};

const MapEvents = ({ onClick }: { onClick: () => void }) => {
    useMapEvents({
        click: () => onClick(),
    });
    return null;
};

// Range rings component
const RangeRings = ({ lat, lon, units }: { lat: number; lon: number; units: 'km' | 'nm' }) => {
    // Conversion to meters
    const kmToM = 1000;
    const nmToM = 1852;
    const unitToM = units === 'nm' ? nmToM : kmToM;

    // Ring distances are the same nice values in user's selected unit
    const RING_DISTANCES = [5, 10, 20, 50, 100];

    // Degrees latitude per km (approx)
    const degPerKm = 1 / 111;

    return (
        <>
            {RING_DISTANCES.map(dist => {
                // Convert to meters based on unit
                const radiusM = dist * unitToM;
                // For label positioning, convert to km for lat offset
                // Add small offset north so label sits ON the ring line
                const radiusKm = radiusM / kmToM;
                const labelOffsetKm = 1; // 1km extra north
                const topLat = lat + ((radiusKm + labelOffsetKm) * degPerKm);

                return (
                    <Fragment key={dist}>
                        <Circle
                            center={[lat, lon]}
                            radius={radiusM}
                            pathOptions={{
                                color: '#4a9eff',
                                weight: 1,
                                opacity: 0.4,
                                fillOpacity: 0,
                                dashArray: '5, 5',
                            }}
                        />
                        {/* Only label rings 10+ */}
                        {dist >= 10 && (
                            <Marker
                                position={[topLat, lon]}
                                icon={L.divIcon({
                                    className: 'range-label',
                                    html: `<span>${dist} ${units}</span>`,
                                    iconSize: [50, 18],
                                    iconAnchor: [25, 9],
                                })}
                            />
                        )}
                    </Fragment>
                );
            })}
        </>
    );
};

import { CoverageLayer } from './CoverageLayer';

// Helper to control map interaction based on connection state
const MapStateController = ({ isConnected }: { isConnected: boolean }) => {
    const map = useMap();
    useEffect(() => {
        if (!isConnected) {
            map.dragging.enable();
            map.doubleClickZoom.enable();
            map.boxZoom.enable();
            map.keyboard.enable();

            // Unlock zoom for world exploration
            map.setMinZoom(2);
            map.setMaxZoom(18);
        } else {
            map.dragging.disable();
            map.touchZoom.disable();
            map.doubleClickZoom.disable();
            map.boxZoom.disable();
            map.keyboard.disable();

            // Lock zoom to aircraft constraints
            map.setMinZoom(MIN_ZOOM);
            map.setMaxZoom(MAX_ZOOM);

            // Force-snap zoom if out of bounds
            if (map.getZoom() < MIN_ZOOM) map.setZoom(MIN_ZOOM);
            if (map.getZoom() > MAX_ZOOM) map.setZoom(MAX_ZOOM);
        }
    }, [isConnected, map]);
    return null;
};

interface MapProps {
    units: 'km' | 'nm';
    showCacheLayer: boolean;
    showVisibilityLayer: boolean;
    pois: POI[];
    selectedPOI: POI | null;
    onPOISelect: (poi: POI) => void;
    onMapClick: () => void;
}

export const Map = ({ units, showCacheLayer, showVisibilityLayer, pois, selectedPOI, onPOISelect, onMapClick }: MapProps) => {

    const { data: telemetry, isLoading: isConnecting } = useTelemetry();
    const isConnected = telemetry?.SimState === 'active';

    // Prevent rendering fallback map until we are sure we are disconnected
    const showFallbackMap = !isConnecting && !isConnected;

    const { status: narratorStatus } = useNarrator();

    // Determine the currently narrated POI
    const currentNarratedPoi = narratorStatus?.playback_status !== 'idle' ? narratorStatus?.current_poi : null;
    const currentNarratedId = currentNarratedPoi?.wikidata_id;

    // Determine the POI being prepared (pipeline)
    const preparingPoi = narratorStatus?.preparing_poi;
    const preparingId = preparingPoi?.wikidata_id;

    // Merge active POI if missing from the main list (e.g. filtered out by backend)
    const displayPois = [...pois];
    if (currentNarratedPoi && !displayPois.find(p => p.wikidata_id === currentNarratedId)) {
        displayPois.push(currentNarratedPoi);
    }
    if (preparingPoi && !displayPois.find(p => p.wikidata_id === preparingId)) {
        displayPois.push(preparingPoi);
    }

    // Default to Berlin if no telemetry yet
    const center: [number, number] = telemetry ? [telemetry.Latitude, telemetry.Longitude] : [52.52, 13.40];

    return (
        <MapContainer
            center={center}
            zoom={isConnected ? DEFAULT_ZOOM : 3}
            minZoom={isConnected ? MIN_ZOOM : 2}
            maxZoom={isConnected ? MAX_ZOOM : 18}
            style={{ height: '100%', width: '100%' }}
            zoomControl={false}
            dragging={!isConnected}
            scrollWheelZoom={true}
            doubleClickZoom={!isConnected}
            touchZoom={!isConnected}
        >
            <MapStateController isConnected={isConnected} />
            <MapEvents onClick={onMapClick} />
            <AircraftPaneSetup />
            <TileLayer
                attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'
                url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
            />
            {showFallbackMap && <CoverageLayer />}
            {showCacheLayer && isConnected && <CacheLayer />}
            <VisibilityLayer enabled={showVisibilityLayer} />
            {isConnected && telemetry && (
                <>
                    <RangeRings lat={telemetry.Latitude} lon={telemetry.Longitude} units={units} />
                    <Recenter mapCenter={[telemetry.Latitude, telemetry.Longitude]} heading={telemetry.Heading} />
                    <div style={{ display: 'none' }}>Reference for Pane Creation</div>
                    <AircraftMarker
                        lat={telemetry.Latitude}
                        lon={telemetry.Longitude}
                        heading={telemetry.Heading}
                    />
                </>
            )}

            {/* Smart Marker Layer handles collision avoidance and rendering */}

            {isConnected && (
                <SmartMarkerLayer
                    pois={displayPois}
                    selectedPOI={selectedPOI}
                    currentNarratedId={currentNarratedId}
                    preparingId={preparingId}
                    onPOISelect={onPOISelect}
                />
            )}
        </MapContainer>
    );
};
