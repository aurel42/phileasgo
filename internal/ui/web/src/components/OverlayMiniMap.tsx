import { useEffect, useState } from 'react';
import { MapContainer, TileLayer, Circle, useMap } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import { AircraftMarker } from './AircraftMarker';
import { SmartMarkerLayer } from './SmartMarkerLayer';
import type { POI } from '../hooks/usePOIs';
import { Fragment } from 'react';

interface OverlayMiniMapProps {
    lat: number;
    lon: number;
    heading: number;
    pois: POI[];
    minPoiScore?: number;
    currentNarratedId?: string;
    preparingId?: string;
    units: 'km' | 'nm';
}

// Wrapper that waits for map to be ready and creates required panes
const WhenMapReady = ({ children }: { children: React.ReactNode }) => {
    const map = useMap();
    const [ready, setReady] = useState(false);

    useEffect(() => {
        // Wait for the map's panes to be available and create aircraftPane
        const checkReady = () => {
            const panes = map.getPanes();
            if (panes && panes.markerPane && panes.overlayPane) {
                // Create aircraftPane if it doesn't exist
                if (!map.getPane('aircraftPane')) {
                    map.createPane('aircraftPane');
                    const pane = map.getPane('aircraftPane');
                    if (pane) pane.style.zIndex = '2000';
                }
                setReady(true);
            } else {
                // Retry after a short delay
                setTimeout(checkReady, 50);
            }
        };
        checkReady();
    }, [map]);

    if (!ready) return null;
    return <>{children}</>
};

// Range rings for the mini-map (only 5, 10, 20) - no Markers to avoid pane issues
const MiniMapRangeRings = ({ lat, lon, units }: { lat: number; lon: number; units: 'km' | 'nm' }) => {
    const nmToM = 1852;
    const kmToM = 1000;
    const unitToM = units === 'nm' ? nmToM : kmToM;

    // Only show 5, 10, 20 rings for the mini-map
    const RING_DISTANCES = [5, 10, 20];

    return (
        <>
            {RING_DISTANCES.map(dist => {
                const radiusM = dist * unitToM;

                return (
                    <Fragment key={dist}>
                        <Circle
                            center={[lat, lon]}
                            radius={radiusM}
                            pathOptions={{
                                color: '#4a9eff',
                                weight: 1.5,
                                opacity: 0.6,
                                fillOpacity: 0,
                                dashArray: '5, 5',
                            }}
                        />
                    </Fragment>
                );
            })}
        </>
    );
};

// Simplified ring labels using a custom component instead of Marker
const RingLabels = ({ lat, units }: { lat: number; units: 'km' | 'nm' }) => {
    const map = useMap();
    const [positions, setPositions] = useState<{ dist: number; top: number; left: number }[]>([]);

    useEffect(() => {
        const updatePositions = () => {
            const nmToM = 1852;
            const kmToM = 1000;
            const unitToM = units === 'nm' ? nmToM : kmToM;
            const degPerKm = 1 / 111;

            const newPositions = [10, 20].map(dist => {
                const radiusM = dist * unitToM;
                const radiusKm = radiusM / kmToM;
                const labelOffsetKm = 1;
                const topLat = lat + ((radiusKm + labelOffsetKm) * degPerKm);
                const point = map.latLngToContainerPoint([topLat, map.getCenter().lng]);
                return { dist, top: point.y, left: point.x };
            });
            setPositions(newPositions);
        };

        updatePositions();
        map.on('move', updatePositions);
        map.on('zoom', updatePositions);

        return () => {
            map.off('move', updatePositions);
            map.off('zoom', updatePositions);
        };
    }, [map, lat, units]);

    return (
        <>
            {positions.map(({ dist, top, left }) => (
                <div
                    key={dist}
                    className="range-label"
                    style={{
                        position: 'absolute',
                        top: top - 9,
                        left: left - 25,
                        width: 50,
                        height: 18,
                        textAlign: 'center',
                        pointerEvents: 'none',
                        zIndex: 1000,
                    }}
                >
                    <span style={{
                        fontSize: '14px',
                        fontFamily: 'Inter, monospace',
                        color: 'rgba(74, 158, 255, 0.7)',
                        textShadow: '0 1px 2px rgba(0, 0, 0, 0.8)',
                    }}>
                        {dist} {units}
                    </span>
                </div>
            ))}
        </>
    );
};

// Map content that depends on the map being ready
const MapContent = ({ lat, lon, heading, pois, minPoiScore, currentNarratedId, preparingId, units }: OverlayMiniMapProps) => {
    return (
        <WhenMapReady>
            {/* Range rings */}
            <MiniMapRangeRings lat={lat} lon={lon} units={units} />

            {/* Aircraft marker */}
            <AircraftMarker lat={lat} lon={lon} heading={heading} />

            {/* POI markers - only if we have POIs */}
            {pois.length > 0 && (
                <SmartMarkerLayer
                    pois={pois}
                    minPoiScore={minPoiScore ?? -100}
                    selectedPOI={null}
                    currentNarratedId={currentNarratedId}
                    preparingId={preparingId}
                    onPOISelect={() => { }}
                />
            )}
        </WhenMapReady>
    );
};

export const OverlayMiniMap = ({ lat, lon, heading, pois, minPoiScore, currentNarratedId, preparingId, units }: OverlayMiniMapProps) => {
    const [map, setMap] = useState<L.Map | null>(null);

    // Fixed zoom for mini-map (zoom 9 shows ~150km, good for 20nm ring visibility)
    const FIXED_ZOOM = 9;

    // Re-center map with heading-based offset (preferring space in front)
    useEffect(() => {
        if (map) {
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
            const aircraftPoint = map.project([lat, lon], FIXED_ZOOM);

            // Add offset to get new center point of the map
            // We want the map center to be "in front" of the aircraft
            const centerPoint = L.point(aircraftPoint.x + dx, aircraftPoint.y + dy);

            // Unproject back to lat/lon
            const centerLatLng = map.unproject(centerPoint, FIXED_ZOOM);

            map.panTo(centerLatLng, { animate: true, duration: 0.1 });
        }
    }, [lat, lon, heading, map]);

    return (
        <div className="overlay-minimap">
            <MapContainer
                center={[lat, lon]}
                zoom={FIXED_ZOOM}
                style={{ height: '100%', width: '100%', background: 'transparent' }}
                zoomControl={false}
                dragging={false}
                scrollWheelZoom={false}
                doubleClickZoom={false}
                touchZoom={false}
                keyboard={false}
                attributionControl={false}
                ref={setMap}
            >
                {/* Faint basemap for geographic context */}
                {/* Faint basemap for geographic context */}
                <TileLayer
                    url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
                    opacity={0.8}
                />

                {/* Ring labels as HTML overlay */}
                <RingLabels lat={lat} units={units} />

                {/* Map content that needs map to be ready */}
                <MapContent
                    lat={lat}
                    lon={lon}
                    heading={heading}
                    pois={pois}
                    minPoiScore={minPoiScore}
                    currentNarratedId={currentNarratedId}
                    preparingId={preparingId}
                    units={units}
                />
            </MapContainer>
        </div>
    );
};
