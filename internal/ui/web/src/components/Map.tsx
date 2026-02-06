import { useEffect, Fragment, useMemo, useState, useRef } from 'react';
import { MapContainer, TileLayer, Marker, useMap, Circle } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import { useQueryClient } from '@tanstack/react-query';
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
import { VictorianToggle } from './VictorianToggle';
import { useTripEvents } from '../hooks/useTripEvents';
import { TripReplayOverlay } from './TripReplayOverlay';

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
        click: () => onInteraction(),
    });
    return null;
};

// Range rings component
const RangeRings = ({ lat, lon, heading, units }: { lat: number; lon: number; heading: number; units: 'km' | 'nm' }) => {
    const map = useMap();
    // Conversion to meters
    const kmToM = 1000;
    const nmToM = 1852;
    const unitToM = units === 'nm' ? nmToM : kmToM;

    // Ring distances are the same nice values in user's selected unit
    const RING_DISTANCES = [5, 10, 20, 50, 100];

    // Degrees latitude per km (approx)
    const degPerKm = 1 / 111.11;
    const toRad = Math.PI / 180;

    // Calculate visible label
    // We do this in render because lat/lon/heading change often, and so does map bounds
    // Optimization: memoize if needed, but for now simple calc is fine.

    // Bounds check
    const bounds = map.getBounds();
    const zoom = map.getZoom();

    // We want the label to be inside the ring by some pixels
    const inwardOffsetPx = 20;

    let bestLabel: { lat: number, lon: number, dist: number } | null = null;

    // Iterate backwards (largest first) to find the first visible one
    for (let i = RING_DISTANCES.length - 1; i >= 0; i--) {
        const dist = RING_DISTANCES[i];
        const radiusM = dist * unitToM;
        const radiusKm = radiusM / kmToM;

        // 1. Calculate point on ring at heading
        const dLat = radiusKm * Math.cos(heading * toRad) * degPerKm;
        const dLon = (radiusKm * Math.sin(heading * toRad) * degPerKm) / Math.cos(lat * toRad);
        const ringLat = lat + dLat;
        const ringLon = lon + dLon;

        // 2. Project to pixels to apply offset
        const ringPoint = map.project([ringLat, ringLon], zoom);

        // 3. Move inwards towards center
        // Vector from ring point to center point (aircraft)
        // Actually simpler: we know the heading is the direction FROM center TO ring point.
        // So to move inwards, we move backwards along that heading.
        // dx = -sin(heading), dy = +cos(heading) in screen space? 
        // Leaflet pixel coords: x increases right, y increases down.
        // Heading 0 (North): x=0, y=-radius. Inward: y increases (down).
        // Heading 90 (East): x=radius, y=0. Inward: x decreases (left).

        // Let's use the vector approach to be safe against projection distortions
        const centerPoint = map.project([lat, lon], zoom);
        const vecX = centerPoint.x - ringPoint.x;
        const vecY = centerPoint.y - ringPoint.y;
        const len = Math.sqrt(vecX * vecX + vecY * vecY);

        if (len === 0) continue;

        const ratio = inwardOffsetPx / len;
        const labelPoint = L.point(
            ringPoint.x + vecX * ratio,
            ringPoint.y + vecY * ratio
        );

        const labelLatLng = map.unproject(labelPoint, zoom);

        // 4. Check visibility (with small buffer)
        if (bounds.contains(labelLatLng)) {
            bestLabel = { lat: labelLatLng.lat, lon: labelLatLng.lng, dist };
            break; // Found the largest visible one
        }
    }

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
                                weight: 1,
                                opacity: 0.4,
                                fillOpacity: 0,
                                dashArray: '5, 5',
                            }}
                        />
                    </Fragment>
                );
            })}

            {bestLabel && (
                <Marker
                    position={[bestLabel.lat, bestLabel.lon]}
                    icon={L.divIcon({
                        className: 'range-label',
                        html: `<span class="role-label-overlay" style="color: rgba(74, 158, 255, 0.9); text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9); font-size: 13px; font-style: italic;">${bestLabel.dist}${units}</span>`,
                        iconSize: [50, 20],
                        iconAnchor: [25, 10], // Center the icon
                    })}
                />
            )}
        </>
    );
};

// Helper to control map interaction based on connection state
const MapStateController = ({ isConnected, isPaused }: { isConnected: boolean; isPaused: boolean }) => {
    const map = useMap();
    useEffect(() => {
        if (!isConnected || isPaused) {
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
    }, [isConnected, isPaused, map]);
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
    const queryClient = useQueryClient();

    const { data: telemetry, isLoading: isConnecting } = useTelemetry();
    const isConnected = telemetry?.SimState === 'active';
    const isDisconnected = telemetry?.SimState === 'disconnected';
    const isPaused = telemetry?.SimState === 'inactive';

    // Trip events for replay mode
    const { data: tripEvents } = useTripEvents();

    const { status: narratorStatus } = useNarrator();

    // Determine if we are in debriefing replay mode
    const isDebriefing = narratorStatus?.current_type === 'debriefing';
    const isIdleReplay = isDisconnected && tripEvents && tripEvents.length > 1;
    const isReplayMode = isIdleReplay || isDebriefing;

    // Track replay mode transitions to force TripReplayOverlay remount
    const [replayKey, setReplayKey] = useState(0);
    const prevReplayModeRef = useRef(false);

    // When entering replay mode, invalidate trip events to get fresh data and force remount
    useEffect(() => {
        if (isReplayMode && !prevReplayModeRef.current) {
            // Transitioning into replay mode - refetch trip events and bump key
            queryClient.invalidateQueries({ queryKey: ['tripEvents'] });
            setReplayKey(k => k + 1);
        }
        prevReplayModeRef.current = isReplayMode;
    }, [isReplayMode, queryClient]);

    // Default duration for idle replay is 2 mins, otherwise use actual audio duration
    const replayDuration = isDebriefing ? (narratorStatus?.current_duration_ms || 120000) : 120000;

    // Prevent rendering fallback map until we are sure we are disconnected or paused
    const showFallbackMap = !isConnecting && (!isConnected || isPaused) && !isReplayMode;

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

    // Centralized Throttling (2s) - Skip during replay mode to prevent map jumping
    useEffect(() => {
        if (!telemetry || isReplayMode) return; // Don't update during replay
        const now = Date.now();
        if (now - lastThrottleRef.current > 2000) {
            setThrottledPos({ lat: telemetry.Latitude, lon: telemetry.Longitude, heading: telemetry.Heading });
            lastThrottleRef.current = now;
        }
    }, [telemetry, isReplayMode]);

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
        // Skip during replay mode to avoid fighting with TripReplayOverlay's flyToBounds
        if (!map || !isConnected || !autoZoom || !throttledPos || isReplayMode) return;

        // Uses throttledPos directly, so no need for internal throttling

        // 1. Identify "non-blue" POIs (active interest)
        const nonBluePois = displayPois.filter(p => {
            const isPlaying = p.wikidata_id === currentNarratedId;
            const isPreparing = p.wikidata_id === preparingId;
            const isPlayed = p.last_played && p.last_played !== "0001-01-01T00:00:00Z";
            return isPlaying || isPreparing || !isPlayed;
        });

        // 2. Determine optimal zoom level
        const lat = throttledPos.lat;
        const lon = throttledPos.lon;

        // Default to a 20km radius (40x40km) bounding box
        const degPerKm = 1 / 111.11;
        let latBuffer = 20 * degPerKm;
        let lonBuffer = 20 * degPerKm / Math.cos(lat * Math.PI / 180);
        let targetZoom = DEFAULT_ZOOM;

        if (nonBluePois.length > 0) {
            const maxLatDiff = Math.max(...nonBluePois.map(p => Math.abs(p.lat - lat)));
            const maxLonDiff = Math.max(...nonBluePois.map(p => Math.abs(p.lon - lon)));

            // Subtract 10% for a tighter fit
            latBuffer = Math.max(maxLatDiff * 0.9, 0.01);
            lonBuffer = Math.max(maxLonDiff * 0.9, 0.01);
        }

        const symmetricBounds = L.latLngBounds(
            [lat - latBuffer, lon - lonBuffer],
            [lat + latBuffer, lon + lonBuffer]
        );

        targetZoom = map.getBoundsZoom(symmetricBounds, false, L.point(60, 60));
        targetZoom = Math.min(targetZoom, MAX_ZOOM);
        targetZoom = Math.max(targetZoom, MIN_ZOOM);

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

    }, [map, isConnected, autoZoom, throttledPos, displayPois, currentNarratedId, preparingId, isReplayMode]);

    // Zoom out to world view when sim is paused
    useEffect(() => {
        if (!map || !isPaused) return;
        map.setView([30, 0], 2, { animate: true });
    }, [map, isPaused]);

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
                <MapStateController isConnected={isConnected} isPaused={isPaused} />
                <MapEvents onInteraction={handleMapInteraction} />
                <AircraftPaneSetup />
                <TileLayer
                    attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'
                    url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
                    noWrap={!isConnected}
                />
                {showFallbackMap && <MapBranding />}
                {showFallbackMap && <CoverageLayer />}
                {isReplayMode && tripEvents && <TripReplayOverlay key={replayKey} events={tripEvents} durationMs={replayDuration} isPlaying={isReplayMode} />}
                {showCacheLayer && isConnected && <CacheLayer />}
                {isConnected && <VisibilityLayer enabled={showVisibilityLayer} />}
                {/* Hide aircraft elements during replay mode to prevent telemetry-based map updates */}
                {isConnected && telemetry && throttledPos && !isReplayMode && (
                    <>
                        {/* We use TELEMETRY (real-time) for Ring positions? No, rings should probably move with the plane. 
                            Actually, stick to throttled for everything visual related to the plane position to avoid jitter. */}
                        <RangeRings lat={throttledPos.lat} lon={throttledPos.lon} heading={throttledPos.heading} units={units} />

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
                {/* Hide during replay mode - TripReplayOverlay has its own animated POI markers */}
                {isConnected && !isReplayMode && (
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
                    <div style={{ background: 'var(--panel-bg)', boxShadow: '0 4px 10px rgba(0,0,0,0.5)', padding: '6px 8px', borderRadius: '4px', border: '1px solid rgba(255,255,255,0.1)' }}>
                        <VictorianToggle
                            checked={autoZoom}
                            onChange={setAutoZoom}
                        />
                    </div>
                </div>
            )}
        </div>
    );
};
