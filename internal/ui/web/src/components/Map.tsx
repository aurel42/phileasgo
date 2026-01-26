import { useEffect, Fragment, useMemo, useState, useRef } from 'react';
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
import { MapBranding } from './MapBranding';
import { CoverageLayer } from './CoverageLayer';

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
        // Recenter now reacts immediately to the passed props, which are already throttled upstream
        const [lat, lon] = mapCenter;
        const mapSize = map.getSize();
        const offsetPx = Math.min(mapSize.x, mapSize.y) * 0.25;
        const hdgRad = heading * (Math.PI / 180);
        const dx = offsetPx * Math.sin(hdgRad);
        const dy = -offsetPx * Math.cos(hdgRad);
        const aircraftPoint = map.project([lat, lon], map.getZoom());
        const centerPoint = L.point(aircraftPoint.x + dx, aircraftPoint.y + dy);
        const centerLatLng = map.unproject(centerPoint, map.getZoom());

        map.panTo(centerLatLng, { animate: false });
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

const MapEvents = ({ onInteraction }: { onInteraction: () => void }) => {
    useMapEvents({
        zoomstart: () => onInteraction(),
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
    const displayPois = useMemo(() => {
        const dp = [...pois];
        if (currentNarratedPoi && !dp.find(p => p.wikidata_id === currentNarratedId)) {
            dp.push(currentNarratedPoi);
        }
        if (preparingPoi && !dp.find(p => p.wikidata_id === preparingId)) {
            dp.push(preparingPoi);
        }
        return dp;
    }, [pois, currentNarratedPoi, preparingPoi]); // preparingId is derived from preparingPoi

    const [throttledPos, setThrottledPos] = useState<{ lat: number, lon: number, heading: number } | null>(null);
    const lastThrottleRef = useRef(0);

    // Centralized Throttling (2s)
    useEffect(() => {
        if (!telemetry) return;
        const now = Date.now();
        if (now - lastThrottleRef.current > 2000) {
            setThrottledPos({ lat: telemetry.Latitude, lon: telemetry.Longitude, heading: telemetry.Heading });
            lastThrottleRef.current = now;
        }
    }, [telemetry]);

    // Use default center if no telemetry
    const center: [number, number] = throttledPos ? [throttledPos.lat, throttledPos.lon] : [52.52, 13.40];

    const [autoZoom, setAutoZoom] = useState(true);
    const [map, setMap] = useState<L.Map | null>(null);

    const isAutomatedMoveRef = useRef(false);
    const lastInteractionAllowedRef = useRef(0);

    useEffect(() => {
        // Simple grace period on mount to ignore Leaflet init events
        lastInteractionAllowedRef.current = Date.now() + 1500;
    }, []);

    // Auto-Zoom Logic (Ported from OverlayMiniMap)
    useEffect(() => {
        // Only run if map exists, we are connected, and auto-zoom is enabled
        if (!map || !isConnected || !autoZoom || !throttledPos) return;

        // Uses throttledPos directly, so no need for internal throttling

        // 1. Identify "non-blue" POIs (active interest)
        const nonBluePois = displayPois.filter(p => {
            const isPlaying = p.wikidata_id === currentNarratedId;
            const isPreparing = p.wikidata_id === preparingId;
            const isPlayed = p.last_played && p.last_played !== "0001-01-01T00:00:00Z";
            return isPlaying || isPreparing || !isPlayed;
        });

        // 2. Determine optimal zoom level
        let targetZoom = DEFAULT_ZOOM;
        const lat = throttledPos.lat;
        const lon = throttledPos.lon;

        if (nonBluePois.length > 0) {
            const maxLatDiff = Math.max(...nonBluePois.map(p => Math.abs(p.lat - lat)));
            const maxLonDiff = Math.max(...nonBluePois.map(p => Math.abs(p.lon - lon)));

            const latBuffer = Math.max(maxLatDiff, 0.01);
            const lonBuffer = Math.max(maxLonDiff, 0.01);

            const symmetricBounds = L.latLngBounds(
                [lat - latBuffer, lon - lonBuffer],
                [lat + latBuffer, lon + lonBuffer]
            );

            targetZoom = map.getBoundsZoom(symmetricBounds, false, L.point(60, 60));
            targetZoom = Math.min(targetZoom, MAX_ZOOM);
            targetZoom = Math.max(targetZoom, MIN_ZOOM);
        }

        // 3. Apply heading-based offset
        const mapSize = map.getSize();
        const offsetPx = Math.min(mapSize.x, mapSize.y) * 0.25;
        const hdgRad = throttledPos.heading * (Math.PI / 180);
        const dx = offsetPx * Math.sin(hdgRad);
        const dy = -offsetPx * Math.cos(hdgRad);

        const aircraftPoint = map.project([lat, lon], targetZoom);
        const centerPoint = L.point(aircraftPoint.x + dx, aircraftPoint.y + dy);
        const centerLatLng = map.unproject(centerPoint, targetZoom);

        isAutomatedMoveRef.current = true;
        map.setView(centerLatLng, targetZoom, {
            animate: false
        });
        // We need to keep it true for a bit because events might fire asynchronously or in the next tick
        setTimeout(() => {
            isAutomatedMoveRef.current = false;
        }, 100);

    }, [map, isConnected, autoZoom, throttledPos, displayPois, currentNarratedId, preparingId]);

    // Disable auto-zoom on manual interaction
    const handleMapInteraction = () => {
        if (isAutomatedMoveRef.current) return;
        if (Date.now() < lastInteractionAllowedRef.current) return;

        if (autoZoom) {
            console.log("Map: Manual interaction detected, disabling autozoom");
            setAutoZoom(false);
        }
        onMapClick(); // Propagate click
    };

    return (
        <div style={{ position: 'relative', height: '100%', width: '100%' }}>
            <MapContainer
                center={center}
                zoom={isConnected ? DEFAULT_ZOOM : 3}
                minZoom={isConnected ? MIN_ZOOM : 2}
                maxZoom={isConnected ? MAX_ZOOM : 18}
                style={{ height: '100%', width: '100%' }}
                zoomControl={false}
                dragging={false}
                scrollWheelZoom={true}
                doubleClickZoom={false}
                touchZoom={false}
                ref={setMap}
            >
                <MapStateController isConnected={isConnected} />
                <MapEvents onInteraction={handleMapInteraction} />
                <AircraftPaneSetup />
                <TileLayer
                    attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'
                    url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
                    noWrap={!isConnected}
                />
                {showFallbackMap && <MapBranding />}
                {showFallbackMap && <CoverageLayer />}
                {showCacheLayer && isConnected && <CacheLayer />}
                <VisibilityLayer enabled={showVisibilityLayer} />
                {isConnected && telemetry && throttledPos && (
                    <>
                        {/* We use TELEMETRY (real-time) for Ring positions? No, rings should probably move with the plane. 
                            Actually, stick to throttled for everything visual related to the plane position to avoid jitter. */}
                        <RangeRings lat={throttledPos.lat} lon={throttledPos.lon} units={units} />

                        {/* Only use Recenter if AutoZoom is OFF */}
                        {!autoZoom && <Recenter mapCenter={[throttledPos.lat, throttledPos.lon]} heading={throttledPos.heading} />}
                        <div style={{ display: 'none' }}>Reference for Pane Creation</div>
                        <AircraftMarker
                            lat={throttledPos.lat}
                            lon={throttledPos.lon}
                            heading={throttledPos.heading}
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

            {/* Auto Zoom Selector Control - Only when Connected */}
            {isConnected && (
                <div style={{ position: 'absolute', bottom: '16px', left: '16px', zIndex: 1000 }}>
                    <div className="length-selector" style={{ background: 'var(--panel-bg)', boxShadow: '0 4px 10px rgba(0,0,0,0.5)', padding: '4px' }}>
                        <span className="role-btn" style={{ fontWeight: 700, color: 'var(--muted)', marginRight: '8px', marginLeft: '4px' }}>AUTOZOOM</span>
                        <div style={{ display: 'flex', background: 'rgba(0, 0, 0, 0.3)', borderRadius: '4px', padding: '2px', border: '1px solid rgba(255, 255, 255, 0.1)' }}>
                            <button
                                className={`length-btn role-btn ${autoZoom ? 'active' : ''}`}
                                onClick={() => setAutoZoom(true)}
                            >
                                ON
                            </button>
                            <button
                                className={`length-btn role-btn ${!autoZoom ? 'active' : ''}`}
                                onClick={() => setAutoZoom(false)}
                            >
                                OFF
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};
