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

// Range rings for the mini-map
const MiniMapRangeRings = ({ lat, lon, units }: { lat: number; lon: number; units: 'km' | 'nm' }) => {
    const nmToM = 1852;
    const kmToM = 1000;
    const unitToM = units === 'nm' ? nmToM : kmToM;

    // Show 5, 10, 20, 50, 100 rings
    const RING_DISTANCES = [5, 10, 20, 50, 100];

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
const RingLabels = ({ lat, lon, heading, units }: { lat: number; lon: number; heading: number; units: 'km' | 'nm' }) => {
    const map = useMap();
    const [visibleLabel, setVisibleLabel] = useState<{ dist: number; top: number; left: number } | null>(null);

    useEffect(() => {
        const updatePositions = () => {
            const nmToM = 1852;
            const kmToM = 1000;
            const unitToM = units === 'nm' ? nmToM : kmToM;
            const degPerKm = 1 / 111.11;

            const hdgRad = (heading * Math.PI) / 180;
            const latRad = (lat * Math.PI) / 180;

            const centerPoint = map.latLngToContainerPoint([lat, lon]);
            const mapSize = map.getSize();
            const buffer = 30; // pixels to considering "visible" (avoid edge clipping)

            // Candidates including 5nm
            const candidates = [5, 10, 20, 50, 100].map(dist => {
                const radiusM = dist * unitToM;
                const radiusKm = radiusM / kmToM;

                // Calculate point on ring at current heading
                const dLat = radiusKm * Math.cos(hdgRad) * degPerKm;
                const dLon = (radiusKm * Math.sin(hdgRad) * degPerKm) / Math.cos(latRad);

                const ringLatLng = L.latLng(lat + dLat, lon + dLon);
                const ringPoint = map.latLngToContainerPoint(ringLatLng);

                // Calculate inward offset vector (towards center)
                const dx = centerPoint.x - ringPoint.x;
                const dy = centerPoint.y - ringPoint.y;
                const distPx = Math.sqrt(dx * dx + dy * dy);

                // Offset by ~20px inwards for spacing
                const offsetPx = 20;
                const ratio = distPx > 0 ? offsetPx / distPx : 0;

                const labelX = ringPoint.x + dx * ratio;
                const labelY = ringPoint.y + dy * ratio;

                // Check visibility
                const isVisible =
                    labelX >= buffer &&
                    labelX <= (mapSize.x - buffer) &&
                    labelY >= buffer &&
                    labelY <= (mapSize.y - buffer);

                return { dist, top: labelY, left: labelX, isVisible };
            });

            // Select the largest distance that is visible
            const selected = candidates
                .filter(c => c.isVisible)
                .sort((a, b) => b.dist - a.dist)[0];

            setVisibleLabel(selected || null);
        };

        updatePositions();
        map.on('move', updatePositions);
        map.on('zoom', updatePositions);

        return () => {
            map.off('move', updatePositions);
            map.off('zoom', updatePositions);
        };
    }, [map, lat, lon, heading, units]);

    if (!visibleLabel) return null;

    const { dist, top, left } = visibleLabel;

    return (
        <div
            key={dist}
            className="range-label"
            style={{
                position: 'absolute',
                top: top - 10,
                left: left - 25,
                width: 50,
                height: 20,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                pointerEvents: 'none',
                zIndex: 1000,
            }}
        >
            <span className="role-label-overlay" style={{
                color: 'rgba(74, 158, 255, 0.9)',
                textShadow: '0 1px 2px rgba(0, 0, 0, 0.9)',
                fontSize: '13px',
                fontStyle: 'italic'
            }}>
                {dist}{units}
            </span>
        </div>
    );
};

// Map content that depends on the map being ready
const MapContent = ({ lat, lon, heading, pois, currentNarratedId, preparingId, units }: OverlayMiniMapProps) => {
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
                    selectedPOI={null}
                    currentNarratedId={currentNarratedId}
                    preparingId={preparingId}
                    onPOISelect={() => { }}
                />
            )}
        </WhenMapReady>
    );
};

export const OverlayMiniMap = ({ lat, lon, heading, pois, currentNarratedId, preparingId, units }: OverlayMiniMapProps) => {
    const [map, setMap] = useState<L.Map | null>(null);

    // Re-center map and adapt zoom
    useEffect(() => {
        if (!map) return;

        // 1. Identify "non-blue" POIs (those to be considered for zoom)
        // Matches POIMarker color logic: Blue is played and not highlighted/preparing.
        // We consider anything that is NOT pure "blue".
        const nonBluePois = pois.filter(p => {
            const isPlaying = p.wikidata_id === currentNarratedId;
            const isPreparing = p.wikidata_id === preparingId;
            const isPlayed = p.last_played && p.last_played !== "0001-01-01T00:00:00Z";

            // Non-blue = currently playing, or preparing, or not yet played
            return isPlaying || isPreparing || !isPlayed;
        });

        // 2. Determine optimal zoom level
        let targetZoom = 10; // Default fallback
        if (nonBluePois.length > 0) {
            // Calculate symmetric bounds to keep aircraft at the "virtual center" for zoom calculation
            const maxLatDiff = Math.max(...nonBluePois.map(p => Math.abs(p.lat - lat)));
            const maxLonDiff = Math.max(...nonBluePois.map(p => Math.abs(p.lon - lon)));

            // Add a small minimum buffer (e.g. 0.01 deg ~1km) to avoid infinity/division by zero issues
            const latBuffer = Math.max(maxLatDiff, 0.01);
            const lonBuffer = Math.max(maxLonDiff, 0.01);

            const symmetricBounds = L.latLngBounds(
                [lat - latBuffer, lon - lonBuffer],
                [lat + latBuffer, lon + lonBuffer]
            );

            // getBoundsZoom returns the zoom level that fits these bounds with padding
            targetZoom = map.getBoundsZoom(symmetricBounds, false, L.point(60, 60));
            targetZoom = Math.min(targetZoom, 12); // Cap zoom to avoid extreme close-ups
        }

        // 3. Apply heading-based offset ALWAYS (consistent 25% shift)
        const mapSize = map.getSize();
        const offsetPx = Math.min(mapSize.x, mapSize.y) * 0.25;
        const hdgRad = heading * (Math.PI / 180);

        // Calculate pixel offset (relative to determined targetZoom)
        const dx = offsetPx * Math.sin(hdgRad);
        const dy = -offsetPx * Math.cos(hdgRad);

        // Project aircraft, apply offset, and unproject to find the visible map center
        const aircraftPoint = map.project([lat, lon], targetZoom);
        const centerPoint = L.point(aircraftPoint.x + dx, aircraftPoint.y + dy);
        const centerLatLng = map.unproject(centerPoint, targetZoom);

        // Update the map view
        map.setView(centerLatLng, targetZoom, {
            animate: true,
            duration: nonBluePois.length > 0 ? 0.5 : 0.1
        });

    }, [lat, lon, heading, map, pois, currentNarratedId, preparingId]);

    return (
        <div className="overlay-minimap">
            <MapContainer
                center={[lat, lon]}
                zoom={10}
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
                <TileLayer
                    url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
                    opacity={0.8}
                />

                {/* Ring labels as HTML overlay */}
                <RingLabels lat={lat} lon={lon} heading={heading} units={units} />

                {/* Map content that needs map to be ready */}
                <MapContent
                    lat={lat}
                    lon={lon}
                    heading={heading}
                    pois={pois}
                    currentNarratedId={currentNarratedId}
                    preparingId={preparingId}
                    units={units}
                />
            </MapContainer>
        </div>
    );
};
