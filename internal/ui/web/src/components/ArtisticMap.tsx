import React, { useEffect, useRef, useState, useMemo } from 'react';
import maplibregl from 'maplibre-gl';
import * as turf from '@turf/turf';
import type { Feature, Polygon, MultiPolygon } from 'geojson';
import 'maplibre-gl/dist/maplibre-gl.css';
import type { Telemetry } from '../types/telemetry';
import type { POI } from '../hooks/usePOIs';
import { PlacementEngine, type LabelCandidate } from '../metrics/PlacementEngine';
import { measureText, getFontFromClass } from '../metrics/text';
import { ARTISTIC_MAP_STYLES } from '../styles/artisticMapStyles';
import { InlineSVG } from './InlineSVG';
import { labelService } from '../services/labelService';
import type { LabelDTO } from '../types/mapLabels';
import { useNarrator } from '../hooks/useNarrator';
import { CompassRose } from './CompassRose';
import { WaxSeal } from './WaxSeal';
import { ScaleBar } from './ScaleBar';

const DEBUG_FLOURISHES = false;

const HotAirBalloon: React.FC<{
    x: number;
    y: number;
    agl: number;
}> = ({ x, y, agl }) => {
    // Interpolation (0 -> 10,000 ft)
    const ratio = Math.min(Math.max(agl / 10000, 0), 1);
    const shadowOffset = ratio * 20;
    const shadowScale = 1 - (ratio * 0.5);

    return (
        <div style={{ position: 'absolute', left: 0, top: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 100 }}>
            {/* Shadow: Soft grey, offset down and left */}
            <svg
                viewBox="0 0 40 50"
                style={{
                    position: 'absolute',
                    left: x - shadowOffset,
                    top: y + shadowOffset,
                    width: 32 * shadowScale,
                    height: 40 * shadowScale,
                    transform: 'translate(-50%, -50%)',
                    filter: 'blur(2px)',
                    opacity: 0.3
                }}
            >
                <path d="M20,5 C12,5 5,12 5,22 C5,28 10,35 20,42 C30,35 35,28 35,22 C35,12 28,5 20,5" fill="black" />
                <rect x="16" y="42" width="8" height="6" fill="black" />
            </svg>

            {/* Balloon: Red body, black outline (1.5px), black basket */}
            <svg
                viewBox="0 0 40 50"
                style={{
                    position: 'absolute',
                    left: x,
                    top: y,
                    width: 32,
                    height: 40,
                    transform: 'translate(-50%, -50%)'
                }}
            >
                {/* Envelope */}
                <path
                    d="M20,5 C12,5 5,12 5,22 C5,28 10,35 20,42 C30,35 35,28 35,22 C35,12 28,5 20,5"
                    fill="#e63946"
                    stroke="black"
                    strokeWidth="1.5"
                />
                {/* Strings */}
                <line x1="12" y1="36" x2="16" y2="42" stroke="black" strokeWidth="1" />
                <line x1="28" y1="36" x2="24" y2="42" stroke="black" strokeWidth="1" />
                {/* Gondola/Basket */}
                <rect x="16" y="42" width="8" height="6" rx="1" fill="#1a1a1a" stroke="black" strokeWidth="1" />
            </svg>
        </div>
    );
};

interface ArtisticMapProps {
    className?: string;
    center: [number, number];
    zoom: number;
    telemetry: Telemetry | null;
    pois: POI[];
    settlementTier: number;
    settlementCategories: string[];
    paperOpacityFog: number;
    paperOpacityClear: number;
    parchmentSaturation: number;
    selectedPOI?: POI | null;
    isAutoOpened?: boolean;
    onPOISelect?: (poi: POI) => void;
    onMapClick?: () => void;
}

// Single Atomic Frame state for strict synchronization
interface MapFrame {
    labels: LabelCandidate[];
    maskPath: string;
    center: [number, number];
    zoom: number;
    offset: [number, number]; // [dx, dy] pixels
    heading: number;
    bearingLine: Feature<any> | null;
    aircraftX: number;
    aircraftY: number;
    agl: number;
}

// Helper for color interpolation (Hex -> Hex/RGB)
const lerpColor = (c1: string, c2: string, t: number): string => {
    const hex = (c: string) => parseInt(c.slice(1), 16);
    const r1 = (hex(c1) >> 16) & 255;
    const g1 = (hex(c1) >> 8) & 255;
    const b1 = (hex(c1)) & 255;
    const r2 = (hex(c2) >> 16) & 255;
    const g2 = (hex(c2) >> 8) & 255;
    const b2 = (hex(c2)) & 255;
    const r = Math.round(r1 + (r2 - r1) * t);
    const g = Math.round(g1 + (g2 - g1) * t);
    const b = Math.round(b1 + (b2 - b1) * t);
    return `rgb(${r}, ${g}, ${b})`;
};

// Helper to apply design-spec font adjustments for Artistic Map only
const adjustFont = (f: { font: string, uppercase: boolean, letterSpacing: number }, offset: number) => ({
    ...f,
    font: f.font.replace(/(\d+)px/, (_, s) => `${Math.max(1, parseInt(s) + offset)}px`)
});

/**
 * ArtisticMap Component
 * 
 * DESIGN PRINCIPLES:
 * 1. DISCRETE INTEGER SCALING:
 *    The map is treated as a physical parchment. It only renders at discrete integer zoom levels 
 *    (Real Zoom) where raster tiles are 1:1 with pixels. Hypothetical Zoom (telemetry-based float) 
 *    is only used to trigger snaps between these layers.
 * 
 * 2. PERMANENCE & LAYERED DETAIL:
 *    When a POI is discovered, it is "stamped" on the current map layer (Z_placed).
 *    Its geographic size matches the parchment at that moment (Scale 1.0).
 *    As the map zooms out (Z_current decreases), the icon shrinks: Scale = 2^(Z_current - Z_placed).
 *    This ensures that icons never overlap as the map grows, and creates a rich texture 
 *    of varied marker sizes representing the aircraft's history across different altitudes.
 * 
 * 3. NO CONTINUOUS BLOAT:
 *    Scaling and collision calculations MUST use the integer Real Zoom. If hypothetical (float) 
 *    zoom is used, icons will "bloat" or "shrink" continuously with altitude, violating
 *    the static parchment aesthetic.
 */
export const ArtisticMap: React.FC<ArtisticMapProps> = ({
    className,
    center,
    zoom,
    telemetry,
    pois,
    // settlementTier is handled by the backend labels Manager
    settlementTier: _settlementTier,
    settlementCategories,
    paperOpacityFog = 0.7,
    paperOpacityClear = 0.1,
    parchmentSaturation = 1.0,
    selectedPOI,
    isAutoOpened = false,
    onPOISelect,
    onMapClick
}) => {
    const mapContainer = useRef<HTMLDivElement>(null);
    const map = useRef<maplibregl.Map | null>(null);
    const [styleLoaded, setStyleLoaded] = useState(false);
    const [fontsLoaded, setFontsLoaded] = useState(false);

    // -- Placement Engine (Persistent across ticks) --
    const engine = useMemo(() => new PlacementEngine(), []);

    // -- Narrator State (for Selected / Next in Queue icon colors) --
    const { status: narratorStatus } = useNarrator();
    const currentNarratedId = (narratorStatus?.playback_status === 'playing' || narratorStatus?.playback_status === 'paused')
        ? narratorStatus?.current_poi?.wikidata_id : undefined;
    const preparingId = narratorStatus?.preparing_poi?.wikidata_id;

    // -- Data Refs for Heartbeat --
    const telemetryRef = useRef(telemetry);
    const poisRef = useRef(pois);
    const zoomRef = useRef(zoom);

    useEffect(() => { telemetryRef.current = telemetry; }, [telemetry]);
    useEffect(() => { poisRef.current = pois; }, [pois]);
    useEffect(() => { zoomRef.current = zoom; }, [zoom]);

    // Design: Sync with font loading to avoid optimistic (narrow) bounding boxes
    useEffect(() => {
        if (document.fonts) {
            document.fonts.ready.then(() => setFontsLoaded(true));
        } else {
            // Fallback for browsers without FontFaceSet
            setFontsLoaded(true);
        }
    }, []);
    // -- THE SINGLE ATOMIC STATE --
    const [frame, setFrame] = useState<MapFrame>({
        labels: [],
        maskPath: '',
        center: [center[1], center[0]],
        zoom: zoom,
        offset: [0, 0],
        heading: 0,
        bearingLine: null,
        aircraftX: 0,
        aircraftY: 0,
        agl: 0
    });

    const accumulatedSettlements = useRef<Map<string, LabelDTO>>(new Map());
    const accumulatedPois = useRef<Map<string, POI>>(new Map());
    const labelAppearanceRef = useRef<Map<string, number>>(new Map());
    const failedPoiLabelIds = useRef<Set<string>>(new Set());
    const labeledPoiIds = useRef<Set<string>>(new Set());
    const lastPlacementResults = useRef<{ wasPlaced: boolean, id: string }>({ wasPlaced: true, id: '' });
    const zoomReady = useRef(false); // Gate placement until heartbeat establishes real zoom
    const [lastSyncLabels, setLastSyncLabels] = useState<LabelDTO[]>([]);
    const [compassRose, setCompassRose] = useState<{ id: string, lat: number, lon: number } | null>(null);
    const compassRoseRef = useRef(compassRose);
    useEffect(() => { compassRoseRef.current = compassRose; }, [compassRose]);

    // Helper to calculate Mask Color
    const getMaskColor = (opacity: number) => {
        const val = Math.floor(opacity * 255);
        return `rgb(${val}, ${val}, ${val})`;
    };

    // Initialize Map (Static Viewport Only)
    useEffect(() => {
        if (map.current || !mapContainer.current) return;

        map.current = new maplibregl.Map({
            container: mapContainer.current,
            style: {
                version: 8,
                sources: {
                    'stamen-watercolor-hd': {
                        type: 'raster',
                        tiles: ['https://watercolormaps.collection.cooperhewitt.org/tile/watercolor/{z}/{x}/{y}.jpg'],
                        tileSize: 128 // HD Source: 128px tiles (displays Z+1 content at Z)
                    },
                    'stamen-watercolor-std': {
                        type: 'raster',
                        tiles: ['https://watercolormaps.collection.cooperhewitt.org/tile/watercolor/{z}/{x}/{y}.jpg'],
                        tileSize: 256 // Standard Source: 256px tiles (displays Z content at Z)
                    }
                },
                layers: [
                    { id: 'background', type: 'background', paint: { 'background-color': '#f4ecd8' } },
                    // HD Layer: Active for Zoom 0-10 (e.g. at Z9, uses Z10 tiles)
                    {
                        id: 'watercolor-hd', type: 'raster', source: 'stamen-watercolor-hd',
                        maxzoom: 10,
                        paint: { 'raster-saturation': -0.2, 'raster-contrast': 0.1 }
                    },
                    // Std Layer: Active for Zoom 10+ (Normal behavior)
                    {
                        id: 'watercolor-std', type: 'raster', source: 'stamen-watercolor-std',
                        minzoom: 10,
                        paint: { 'raster-saturation': -0.2, 'raster-contrast': 0.1 }
                    }
                ]
            },
            center: [center[1], center[0]],
            zoom: zoom,
            minZoom: 9, // Allowed now that we have higher-res tiles (Z10 tiles at Z9 view)
            maxZoom: 13,
            attributionControl: false,
            interactive: false
        });

        map.current.on('load', () => {
            map.current?.addSource('bearing-line', {
                type: 'geojson',
                data: { type: 'Feature', properties: {}, geometry: { type: 'LineString', coordinates: [] } }
            });
            map.current?.addLayer({
                id: 'bearing-line-layer', type: 'line', source: 'bearing-line',
                paint: { 'line-color': '#5c4033', 'line-width': 2, 'line-dasharray': [2, 2], 'line-opacity': 0.7 }
            });
            setStyleLoaded(true);
        });

        return () => { map.current?.remove(); map.current = null; };
    }, []);

    const spawnCompassRose = (mapInstance: maplibregl.Map, acHeading: number) => {
        const canvas = mapInstance.getCanvas();
        const w = canvas.clientWidth;
        const h = canvas.clientHeight;
        const padding = 80;

        const candidates = [
            { id: 'tl', x: padding, y: padding, angle: 315 },
            { id: 'tr', x: w - padding, y: padding, angle: 45 },
            { id: 'bl', x: padding, y: h - padding, angle: 225 },
            { id: 'br', x: w - padding, y: h - padding, angle: 135 }
        ];

        const normHdg = (acHeading % 360 + 360) % 360;
        candidates.sort((a, b) => {
            const da = Math.abs((a.angle - normHdg + 180 + 360) % 360 - 180);
            const db = Math.abs((b.angle - normHdg + 180 + 360) % 360 - 180);
            return da - db;
        });

        return candidates;
    };

    // useLayoutEffect ensures the map snapped in the SAME paint cycle as the labels
    React.useLayoutEffect(() => {
        const m = map.current;
        if (!m || !styleLoaded) return;
        m.easeTo({
            center: frame.center,
            zoom: frame.zoom,
            offset: [frame.offset[0], frame.offset[1]],
            duration: 0
        });
    }, [frame.center, frame.zoom, frame.offset, styleLoaded]);

    // --- THE HEARTBEAT (Strict 0.5Hz / 2000ms) ---
    useEffect(() => {
        if (!styleLoaded || !map.current) return;

        let isRunning = false;
        let lastMaskData: any = null;
        let prevZoomInt = -1;
        let firstTick = true;

        // Dead-zone panning: map stays locked until aircraft exits this circle
        let lockedCenter: [number, number] | null = null; // [lng, lat]
        let lockedOffset: [number, number] = [0, 0];
        let lockedZoom = -1;

        const tick = async () => {
            if (isRunning) return;
            isRunning = true;

            try {
                // Design Section 6: Ensure fonts are loaded before measurement
                if (document.fonts) {
                    await document.fonts.ready;
                }
                const m = map.current;
                const t = telemetryRef.current;

                if (!m || !t || (t.Latitude === 0 && t.Longitude === 0)) return;

                // 1. Snapshot State
                const bounds = m.getBounds();
                const targetZoomBase = zoomRef.current;

                if (!bounds) return;

                // 2. BACKGROUND FETCH (Visibility Mask)
                const fetchMask = async () => {
                    try {
                        const maskRes = await fetch(`/api/map/visibility-mask?bounds=${bounds.getNorth()},${bounds.getEast()},${bounds.getSouth()},${bounds.getWest()}&resolution=20`);
                        if (maskRes.ok) lastMaskData = await maskRes.json();
                    } catch (e) {
                        console.error("Background Mask Fetch Failed:", e);
                    }
                };
                if (firstTick) {
                    await fetchMask();
                    // firstTick logic for labels moved to needsRecenter
                } else {
                    fetchMask();
                }

                const mapWidth = m.getCanvas().clientWidth;
                const mapHeight = m.getCanvas().clientHeight;

                // 3. DEAD-ZONE PANNING — re-center when more map is behind aircraft than ahead
                let needsRecenter = !lockedCenter; // First tick always centers

                if (lockedCenter) {
                    const aircraftOnMap = m.project([t.Longitude, t.Latitude]);
                    const hdgRad = t.Heading * (Math.PI / 180);
                    // Ray direction in screen coords (Y-down)
                    const adx = Math.sin(hdgRad);
                    const ady = -Math.cos(hdgRad);

                    // Distance from point to viewport edge along a ray direction
                    const rayToEdge = (px: number, py: number, dx: number, dy: number): number => {
                        let tMin = Infinity;
                        if (dx > 0) tMin = Math.min(tMin, (mapWidth - px) / dx);
                        else if (dx < 0) tMin = Math.min(tMin, -px / dx);
                        if (dy > 0) tMin = Math.min(tMin, (mapHeight - py) / dy);
                        else if (dy < 0) tMin = Math.min(tMin, -py / dy);
                        return tMin === Infinity ? 0 : Math.max(0, tMin);
                    };

                    const distAhead = rayToEdge(aircraftOnMap.x, aircraftOnMap.y, adx, ady);
                    const distBehind = rayToEdge(aircraftOnMap.x, aircraftOnMap.y, -adx, -ady);
                    needsRecenter = distBehind > distAhead;
                }

                if (needsRecenter) {
                    // Re-center: place aircraft with heading offset (35% pushes it well behind center)
                    const offsetPx = Math.min(mapWidth, mapHeight) * 0.35;
                    const hdgRad = t.Heading * (Math.PI / 180);
                    const dx = offsetPx * Math.sin(hdgRad);
                    const dy = -offsetPx * Math.cos(hdgRad);

                    lockedCenter = [t.Longitude, t.Latitude];
                    lockedOffset = [-dx, -dy];

                    // Compute zoom: fit visibility cone if available, otherwise use base
                    let newZoom = lockedZoom === -1 ? targetZoomBase : lockedZoom;
                    if (lastMaskData?.geometry) {
                        const coneBbox = turf.bbox(lastMaskData.geometry);
                        const camera = m.cameraForBounds(coneBbox as [number, number, number, number], { padding: 0, maxZoom: 13 });
                        if (camera?.zoom !== undefined && !isNaN(camera.zoom)) {
                            newZoom = Math.min(Math.max(camera.zoom, 9), 13);
                        }
                    }
                    // COMPUTE REAL ZOOM (Discrete 0.5 Step Snap)
                    // We render at 0.5 increments to soften the jump while keeping HD tiles.
                    lockedZoom = Math.round(newZoom * 2) / 2;

                    // Move map to locked position BEFORE projecting
                    m.easeTo({ center: lockedCenter as maplibregl.LngLatLike, zoom: lockedZoom, offset: lockedOffset, duration: 0 });
                }

                // -- SMART SYNC: Fetch labels on move/snap (backend handles density limit) --
                const b = m.getBounds();
                labelService.fetchLabels({
                    bbox: [b.getSouth(), b.getWest(), b.getNorth(), b.getEast()],
                    ac_lat: t.Latitude,
                    ac_lon: t.Longitude,
                    heading: t.Heading,
                    zoom: lockedZoom
                }).then(newLabels => {
                    newLabels.forEach(l => accumulatedSettlements.current.set(l.id, l));
                    setLastSyncLabels(newLabels);
                }).catch(e => console.error("Label Sync Failed:", e));

                if (firstTick) {
                    firstTick = false;
                    zoomReady.current = true;
                }

                const targetZoom = lockedZoom;

                // Tileset level change detection (0.5 granularity)
                const currentZoomSnap = Math.round(targetZoom * 2) / 2;
                if (prevZoomInt === -1) prevZoomInt = currentZoomSnap;

                // Pruning Helper
                const pruneOffscreen = (bounds: maplibregl.LngLatBounds) => {
                    const checkPrune = (map: Map<string, any>) => {
                        for (const [id, item] of map.entries()) {
                            // Basic LngLat check: if any of the coordinates is outside bounds
                            if (!bounds.contains([item.lon, item.lat])) {
                                map.delete(id);
                                engine.forget(id);
                            }
                        }
                    };
                    checkPrune(accumulatedSettlements.current);
                    checkPrune(accumulatedPois.current);
                    // Clear failures so we retry labeling in the newly visible/scaled areas
                    failedPoiLabelIds.current.clear();
                    labeledPoiIds.current.clear();
                };

                // 8. COMPASS ROSE PERSISTENCE & FALLBACKS
                // Placed on zoom change; persists at geo coords (stamped on map) between zooms.
                // Repositioned only if its center scrolls outside the viewport after a pan.
                let nextCompass = compassRoseRef.current;
                const results = lastPlacementResults.current;
                const corners = spawnCompassRose(m, t.Heading);

                if (!nextCompass || currentZoomSnap !== prevZoomInt) {
                    // First placement or zoom changed: stamp in best corner
                    const choice = corners[0];
                    const lngLat = m.unproject([choice.x, choice.y]);
                    nextCompass = { id: choice.id, lat: lngLat.lat, lon: lngLat.lng };
                    setCompassRose(nextCompass);
                    lastPlacementResults.current = { wasPlaced: true, id: nextCompass.id };
                } else if (!results.wasPlaced && frame.labels.length > 0 && results.id === nextCompass.id) {
                    // Blocked by placement engine: cycle to next best corner
                    const idx = corners.findIndex(c => c.id === nextCompass?.id);
                    const nextIdx = (idx + 1) % corners.length;
                    const choice = corners[nextIdx];
                    const lngLat = m.unproject([choice.x, choice.y]);
                    nextCompass = { id: choice.id, lat: lngLat.lat, lon: lngLat.lng };
                    setCompassRose(nextCompass);
                    lastPlacementResults.current = { wasPlaced: true, id: nextCompass.id };
                }

                // Between zooms: compass persists at its geo coords.
                // Reposition only if center scrolled outside the viewport after a pan.
                if (nextCompass) {
                    const compassPx = m.project([nextCompass.lon, nextCompass.lat]);
                    if (compassPx.x < 0 || compassPx.x > mapWidth || compassPx.y < 0 || compassPx.y > mapHeight) {
                        const choice = corners[0];
                        const lngLat = m.unproject([choice.x, choice.y]);
                        nextCompass = { id: choice.id, lat: lngLat.lat, lon: lngLat.lng };
                        setCompassRose(nextCompass);
                    }
                }

                if (currentZoomSnap !== prevZoomInt) {
                    pruneOffscreen(m.getBounds());
                    prevZoomInt = currentZoomSnap;
                }

                // 5. PROJECT using the STABLE map state
                const aircraftPos = m.project([t.Longitude, t.Latitude]);
                const aircraftX = Math.round(aircraftPos.x);
                const aircraftY = Math.round(aircraftPos.y);

                // 7. BEARING LINE
                const lat1 = t.Latitude * Math.PI / 180;
                const lon1 = t.Longitude * Math.PI / 180;
                const brng = t.Heading * Math.PI / 180;
                const R = 3440.065; const d = 50.0;
                const lat2 = Math.asin(Math.sin(lat1) * Math.cos(d / R) + Math.cos(lat1) * Math.sin(d / R) * Math.cos(brng));
                const lon2 = lon1 + Math.atan2(Math.sin(brng) * Math.sin(d / R) * Math.cos(lat1), Math.cos(d / R) - Math.sin(lat1) * Math.sin(lat2));

                const bLine: Feature<any> = {
                    type: 'Feature', properties: {},
                    geometry: { type: 'LineString', coordinates: [[t.Longitude, t.Latitude], [lon2 * 180 / Math.PI, lat2 * 180 / Math.PI]] }
                };
                const lineSource = m.getSource('bearing-line') as maplibregl.GeoJSONSource;
                if (lineSource) lineSource.setData(bLine);

                // 9. COMMIT (Labels now computed via useMemo outside this loop)
                setFrame(prev => ({
                    ...prev,
                    maskPath: lastMaskData ? maskToPath(lastMaskData, m) : '',
                    center: lockedCenter!,
                    zoom: targetZoom,
                    offset: lockedOffset,
                    heading: t.Heading,
                    bearingLine: bLine,
                    aircraftX,
                    aircraftY,
                    agl: t.AltitudeAGL
                }));
            } catch (err) {
                console.error("Heartbeat Loop Crash:", err);
            } finally {
                isRunning = false;
            }
        };

        tick();
        const interval = setInterval(tick, 2000);
        return () => clearInterval(interval);
    }, [styleLoaded]);

    // --- COLLISION SOLVER (Memoized) ---
    // Runs only when sync labels, local POIs, or viewport changes.
    const computedLabels = useMemo<LabelCandidate[]>(() => {
        const m = map.current;
        if (!m || !styleLoaded || !zoomReady.current || !fontsLoaded) return [];

        engine.clear();

        // 0. Extract Fonts from CSS Roles (Apply adjustments per design spec)
        const cityFont = adjustFont(getFontFromClass('role-title'), -4);
        const townFont = adjustFont(getFontFromClass('role-header'), -4);
        const villageFont = adjustFont(getFontFromClass('role-text-lg'), -4);
        const secondaryFont = adjustFont(getFontFromClass('role-label'), 2);

        // 1. Process Sync Labels (Settlements from DB)
        Array.from(accumulatedSettlements.current.values()).forEach(l => {
            const id = l.id;
            let tierName: 'city' | 'town' | 'village' = 'village';
            let role = villageFont;

            if (l.category === 'city') {
                tierName = 'city';
                role = cityFont;
            } else if (l.category === 'town') {
                tierName = 'town';
                role = townFont;
            }

            let text = l.name.split('(')[0].split(',')[0].split('/')[0].trim();
            if (role.uppercase) text = text.toUpperCase();

            const dims = measureText(text, role.font, role.letterSpacing);

            engine.register({
                id, lat: l.lat, lon: l.lon, text, tier: tierName,
                width: dims.width, height: dims.height, type: 'settlement', score: l.pop || 0,
                isHistorical: false, size: 'L'
            });
        });

        // 1.5. Process Compass Rose
        if (compassRose) {
            const size = 52; // Approximately 2x POI marker
            engine.register({
                id: compassRose.id, lat: compassRose.lat, lon: compassRose.lon,
                text: "", tier: 'village', score: 200,
                width: size, height: size, type: 'compass', isHistorical: false
            });
        }

        // 2. Identify the "Champion POI" for labeling (Singleton Strategy)
        // Find the single highest scoring POI currently in view that hasn't failed and isn't already labeled
        const settlementCatSet = new Set(settlementCategories.map(c => c.toLowerCase()));
        let champion: POI | null = null;
        pois.forEach(p => {
            if (settlementCatSet.has(p.category?.toLowerCase())) return;

            // Name Length Constraint: Normalized name must not exceed 24 characters
            const normalizedName = p.name_en.split('(')[0].split(',')[0].split('/')[0].trim();
            if (normalizedName.length > 24) return;

            if (p.score >= 10 && !failedPoiLabelIds.current.has(p.wikidata_id) && !labeledPoiIds.current.has(p.wikidata_id)) {
                if (!champion || (p.score > champion.score)) {
                    champion = p;
                }
            }
        });

        pois.forEach(p => {
            if (!p.lat || !p.lon) return;
            accumulatedPois.current.set(p.wikidata_id, p);
        });

        Array.from(accumulatedPois.current.values()).forEach(p => {
            const sizePx = 26;
            const isHistorical = !!(p.last_played && p.last_played !== "0001-01-01T00:00:00Z");

            // Secondary Label for Champion OR already Labeled POIs
            let secondaryLabel = undefined;
            if (labeledPoiIds.current.has(p.wikidata_id) || (champion && p.wikidata_id === champion.wikidata_id)) {
                let text = p.name_en.split('(')[0].split(',')[0].split('/')[0].trim();
                if (secondaryFont.uppercase) text = text.toUpperCase();
                const dims = measureText(text, secondaryFont.font, secondaryFont.letterSpacing);
                secondaryLabel = { text, width: dims.width, height: dims.height };
            }

            engine.register({
                id: p.wikidata_id, lat: p.lat, lon: p.lon, text: "", tier: 'village', score: p.score || 0,
                width: sizePx, height: sizePx, type: 'poi', isHistorical, size: p.size as any, icon: p.icon,
                visibility: p.visibility,
                secondaryLabel
            });
        });

        const vw = mapContainer.current?.clientWidth || window.innerWidth;
        const vh = mapContainer.current?.clientHeight || window.innerHeight;

        // 3. Optional: Debug Sampler Grid
        if (DEBUG_FLOURISHES) {
            const samplers = [
                { id: 'dbg-1', name: 'Normal Halo', halo: 'normal', icon: 'castle' },
                { id: 'dbg-2', name: 'Dark Smudge', halo: 'organic', icon: 'mountain' },
                { id: 'dbg-3', name: 'Neon Cyan', halo: 'neon-cyan', icon: 'rocket' },
                { id: 'dbg-4', name: 'Neon Pink', halo: 'neon-pink', icon: 'amusement-park' },
                { id: 'dbg-5', name: 'Silhouette', silhouette: true, icon: 'communications-tower' },
                { id: 'dbg-6', name: 'Heavy Ink', weight: 1.45, icon: 'landmark' }, // Softened from 1.8 (-20%)
                { id: 'dbg-7', name: 'Red Sketch', weight: 2, color: 'red', icon: 'stadium' },
            ];

            samplers.forEach((s, i) => {
                // Perfect viewport column anchoring: unproject unique screen points
                const screenY = 120 + (i * 45); // Constant pixel spacing
                const lngLat = m.unproject([60, screenY]);

                engine.register({
                    id: s.id, lat: lngLat.lat, lon: lngLat.lng, text: "",
                    tier: 'landmark', // Locked Phase 1 - zero movement, no tethers
                    score: 100,
                    width: 26, height: 26, type: 'poi', isHistorical: false,
                    icon: s.icon,
                    secondaryLabel: { text: s.name, width: 100, height: 16 },
                    custom: {
                        halo: s.halo,
                        silhouette: s.silhouette,
                        weight: s.weight,
                        color: s.color
                    }
                });
            });
        }

        const result = engine.compute(
            (lat: number, lon: number) => {
                const p = m.project([lon, lat]);
                return { x: p.x, y: p.y };
            },
            vw,
            vh,
            frame.zoom
        );

        // Update successful placement tracking for Heartbeat
        if (compassRose) {
            const placedCompass = result.some(l => l.type === 'compass' && l.id === compassRose.id);
            lastPlacementResults.current = { wasPlaced: placedCompass, id: compassRose.id };
        }

        // Fail/Success Memory Logic for the Champion
        if (champion) {
            const placedChamp = result.find(l => l.id === (champion as POI).wikidata_id);
            if (placedChamp && placedChamp.secondaryLabel) {
                if (placedChamp.secondaryLabelPos) {
                    labeledPoiIds.current.add((champion as POI).wikidata_id);
                } else {
                    failedPoiLabelIds.current.add((champion as POI).wikidata_id);
                }
            }
        }

        return result;

    }, [lastSyncLabels, pois, frame.zoom, frame.center, frame.offset, styleLoaded, settlementCategories, fontsLoaded, compassRose]);

    // Update frame with computed labels
    useEffect(() => {
        setFrame(prev => ({ ...prev, labels: computedLabels }));
    }, [computedLabels]);

    const maskToPath = (geojson: Feature<Polygon | MultiPolygon>, mapInstance: maplibregl.Map): string => {
        if (!geojson.geometry) return '';
        const coords = geojson.geometry.type === 'Polygon' ? [geojson.geometry.coordinates] : geojson.geometry.coordinates;
        return coords.map(poly => poly.map(ring => ring.map(coord => {
            const p = mapInstance.project([coord[0], coord[1]]);
            return `${p.x},${p.y}`;
        }).join(' L ')).map(ringStr => `M ${ringStr} Z`).join(' ')).join(' ');
    };

    return (
        <div className={className}
            onClick={() => onMapClick?.()}
            style={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden' }}>
            <div ref={mapContainer} style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', color: 'black' }} />

            {/* Dual-Scale Bar (above paper, below labels) */}
            <ScaleBar zoom={frame.zoom} latitude={frame.center[1]} />

            {/* Labels Overlay (Atomic from Frame) */}
            <div style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 20 }}>
                {frame.labels.map(l => {
                    // Zoom-relative scale: markers shrink/grow to stay the same size on the map surface
                    // Both frame.zoom (Real Zoom) and l.placedZoom (Placement Real Zoom) are integers.
                    let zoomScale = l.placedZoom != null ? Math.pow(2, frame.zoom - l.placedZoom) : 1;

                    // Settlement Scale Cap: Avoid "HUGE TEXT" when zooming in (stay <= 1.0x native)
                    if (l.type === 'settlement') {
                        zoomScale = Math.min(zoomScale, 1.0);
                    }

                    // Fade-In Logic
                    const now = Date.now();
                    if (!labelAppearanceRef.current.has(l.id)) {
                        labelAppearanceRef.current.set(l.id, now);
                    }
                    const start = labelAppearanceRef.current.get(l.id)!;
                    const fadeOpacity = Math.min(1, (now - start) / 2000);

                    if (l.type === 'poi') {
                        // ... POI Icon Rendering ...
                        const poi = accumulatedPois.current.get(l.id);
                        const iconName = (poi?.icon_artistic || l.icon || 'attraction');
                        const iconUrl = `/icons/${iconName}.svg`;

                        // Semantic Logic Simplification: Playing or Preparing items are NEVER treated as historic
                        const isLive = l.id === currentNarratedId || l.id === preparingId;
                        const effectiveIsHistorical = l.isHistorical && !isLive;

                        // Tether logic: Historic items never get tethers, regardless of distance.
                        const tTx = l.trueX ?? 0;
                        const tTy = l.trueY ?? 0;
                        const tDx = (l.finalX || 0) - tTx;
                        const tDy = (l.finalY || 0) - tTy;
                        const tDist = Math.ceil(Math.sqrt(tDx * tDx + tDy * tDy));
                        const isDisplaced = !effectiveIsHistorical && tDist > 68;

                        const score = l.score || 0;
                        const isHero = score >= 20 && !effectiveIsHistorical;

                        // 1. Icon Fill Color: Strictly Score-based (Silver -> Gold) unless Historical
                        let iconColor = ARTISTIC_MAP_STYLES.colors.icon.copper;
                        if (effectiveIsHistorical) {
                            iconColor = ARTISTIC_MAP_STYLES.colors.icon.historical;
                        } else {
                            if (score <= 0) {
                                iconColor = ARTISTIC_MAP_STYLES.colors.icon.silver;
                            } else if (score >= 20) {
                                iconColor = ARTISTIC_MAP_STYLES.colors.icon.gold;
                            } else {
                                const t = score / 20.0;
                                iconColor = lerpColor(ARTISTIC_MAP_STYLES.colors.icon.silver, ARTISTIC_MAP_STYLES.colors.icon.gold, t);
                            }
                        }

                        // 2. Halo Properties
                        const isActive = l.id === currentNarratedId;
                        const isPreparing = l.id === preparingId;
                        const isSelected = selectedPOI && l.id === selectedPOI.wikidata_id;
                        const isDeferred = poi?.is_deferred || poi?.badges?.includes('deferred');

                        let hColor = ARTISTIC_MAP_STYLES.colors.icon.normalHalo; // Paper White
                        let hSize = 2;
                        let hLayers = isActive ? 3 : (isPreparing ? 2 : 1);

                        if (isHero) {
                            hColor = ARTISTIC_MAP_STYLES.colors.icon.gold;
                        }

                        if (isSelected && !isAutoOpened) {
                            hColor = ARTISTIC_MAP_STYLES.colors.icon.neonCyan;
                            hLayers = 3;
                        }

                        if (score > 30 && !effectiveIsHistorical) {
                            // Non-linear scaling: 2px at 30, ~5px at 150 (X=3.5)
                            hSize = 2 + (Math.sqrt(score / 10 - 2) - 1) * 3.5;
                        }

                        let silhouette = false;
                        if (isDeferred) {
                            silhouette = true;
                        }

                        let outlineWeight = 1.2; // Optimized from 0.8 (+50%) for better ink clarity

                        const isDeepDive = poi?.badges?.includes('deep_dive');
                        if (isDeepDive) {
                            outlineWeight = 1.45; // "Heavy Ink" style for deep dive content (-20% from 1.8)
                        }

                        let outlineColor = effectiveIsHistorical ? iconColor : ARTISTIC_MAP_STYLES.colors.icon.stroke;

                        if (l.custom) {
                            if (l.custom.silhouette) silhouette = true;
                            if (l.custom.weight) outlineWeight = l.custom.weight;
                            if (l.custom.color) outlineColor = l.custom.color;
                        }

                        // Filter mapping for Halos (applying dynamic size and layers)
                        let dropShadowFilter = '';
                        if (effectiveIsHistorical || (l.custom?.halo === 'none')) {
                            dropShadowFilter = 'none';
                        } else if (l.custom?.halo === 'organic' || (l.id.startsWith('dbg-2') && DEBUG_FLOURISHES)) {
                            dropShadowFilter = `drop-shadow(1px 1px 2px ${ARTISTIC_MAP_STYLES.colors.icon.organicSmudge}) drop-shadow(-1px -1px 2px ${ARTISTIC_MAP_STYLES.colors.icon.organicSmudge})`;
                        } else {
                            // Standard or Special (Neon, Gold, Selected)
                            if (hLayers === 3) {
                                dropShadowFilter = `drop-shadow(0 0 ${hSize / 2}px ${hColor}) drop-shadow(0 0 ${hSize}px ${hColor}) drop-shadow(0 0 ${hSize * 2}px ${hColor})`;
                            } else if (hLayers === 2) {
                                dropShadowFilter = `drop-shadow(0 0 ${hSize / 2}px ${hColor}) drop-shadow(0 0 ${hSize}px ${hColor})`;
                            } else {
                                dropShadowFilter = `drop-shadow(0 0 ${hSize}px ${hColor})`;
                            }
                        }

                        // Silhouette Logic
                        if (l.custom?.silhouette) silhouette = true;

                        if (silhouette) {
                            // Empty: moved to InlineSVG filter for better halo preservation
                        }

                        const activeBoost = l.id === currentNarratedId ? 1.5 : l.id === preparingId ? 1.25 : 1;

                        const swayOut = 36;
                        const swayIn = 24;
                        // Deterministic sway direction based on ID to keep it stable but organic
                        const swayDir = (l.id.charCodeAt(0) % 2 === 0 ? 1 : -1);
                        const startX = l.trueX || 0;
                        const startY = l.trueY || 0;
                        const endX = l.finalX || 0;
                        const endY = l.finalY || 0;
                        const dy = endY - startY;

                        // Calligraphic stroke: Two Bezier curves (Outer/Inner) forming a filled shape that's bulky in the center
                        const cp1OutX = startX + (swayOut * swayDir);
                        const cp1OutY = startY + (dy * 0.1);
                        const cp2OutX = endX - (swayOut * swayDir);
                        const cp2OutY = startY + (dy * 0.9);

                        const cp1InX = startX + (swayIn * swayDir);
                        const cp1InY = startY + (dy * 0.1);
                        const cp2InX = endX - (swayIn * swayDir);
                        const cp2InY = startY + (dy * 0.9);

                        return (
                            <React.Fragment key={l.id}>
                                {isDisplaced && (
                                    <svg style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 15, opacity: fadeOpacity * ARTISTIC_MAP_STYLES.tethers.opacity }}>
                                        <path d={`M ${startX},${startY} C ${cp1OutX},${cp1OutY} ${cp2OutX},${cp2OutY} ${endX},${endY} C ${cp2InX},${cp2InY} ${cp1InX},${cp1InY} ${startX},${startY} Z`}
                                            fill={ARTISTIC_MAP_STYLES.tethers.stroke}
                                            stroke={ARTISTIC_MAP_STYLES.tethers.stroke}
                                            strokeWidth={ARTISTIC_MAP_STYLES.tethers.width}
                                            strokeLinejoin="round" />
                                        <circle cx={startX} cy={startY} r={ARTISTIC_MAP_STYLES.tethers.dotRadius} fill={ARTISTIC_MAP_STYLES.tethers.stroke} opacity={ARTISTIC_MAP_STYLES.tethers.dotOpacity} />
                                    </svg>
                                )}
                                {(l.id === currentNarratedId || l.id === preparingId) && (
                                    <div style={{
                                        position: 'absolute', left: l.finalX ?? 0, top: l.finalY ?? 0,
                                        // Stable random rotation based on ID bytes
                                        transform: `translate(-50%, -50%) scale(${zoomScale * activeBoost}) rotate(${(l.id.split('').reduce((acc, char) => acc + char.charCodeAt(0), 0) % 360)}deg)`,
                                        opacity: l.id === currentNarratedId ? 1 : 0.5,
                                        pointerEvents: 'none',
                                        zIndex: 14
                                    }}>
                                        <WaxSeal size={l.width} />
                                    </div>
                                )}
                                <div
                                    onClick={(e) => {
                                        e.stopPropagation();
                                        const poi = accumulatedPois.current.get(l.id);
                                        if (poi && onPOISelect) onPOISelect(poi);
                                    }}
                                    style={{
                                        position: 'absolute', left: l.finalX ?? 0, top: l.finalY ?? 0, width: l.width, height: l.height,
                                        transform: `translate(-50%, -50%) scale(${zoomScale * activeBoost})`,
                                        opacity: (effectiveIsHistorical ? 0.5 : 1) * fadeOpacity,
                                        color: iconColor, cursor: 'pointer', pointerEvents: 'auto',
                                        // Use drop-shadow filter for true shape contour ("Halo")
                                        filter: dropShadowFilter,
                                        zIndex: 15
                                    }}
                                >
                                    <InlineSVG
                                        src={iconUrl}
                                        style={{
                                            width: '100%', height: '100%',
                                            // @ts-ignore - custom CSS variables for the stamped-icon class
                                            '--stamped-stroke': outlineColor,
                                            '--stamped-width': `${outlineWeight}px`,
                                            filter: silhouette ? 'contrast(0) brightness(0)' : undefined
                                        }}
                                        className="stamped-icon"
                                    />
                                </div>

                                {l.secondaryLabel && l.secondaryLabelPos && (
                                    <div
                                        className="role-label"
                                        style={{
                                            position: 'absolute',
                                            left: l.secondaryLabelPos.x,
                                            top: l.secondaryLabelPos.y,
                                            transform: `translate(-50%, -50%) scale(${zoomScale})`,
                                            fontSize: '17px', // Match secondaryFont adjustment (+2)
                                            opacity: fadeOpacity,
                                            pointerEvents: 'none',
                                            zIndex: 25,
                                            textShadow: ARTISTIC_MAP_STYLES.colors.shadows.atmosphere,
                                            whiteSpace: 'nowrap'
                                        }}
                                    >
                                        {l.secondaryLabel.text}
                                    </div>
                                )}
                            </React.Fragment>
                        );
                    }

                    if (l.type === 'compass') {
                        return (
                            <div
                                key={l.id}
                                style={{
                                    position: 'absolute', left: l.finalX ?? 0, top: l.finalY ?? 0, width: l.width, height: l.height,
                                    transform: `translate(-50%, -50%)`,
                                    opacity: 0.8 * fadeOpacity,
                                    color: ARTISTIC_MAP_STYLES.colors.icon.compass,
                                    pointerEvents: 'none',
                                    zIndex: 5
                                }}
                            >
                                <CompassRose size={l.width} />
                            </div>
                        );
                    }

                    // Settlement Rendering
                    return (
                        <React.Fragment key={l.id}>
                            <div
                                className={l.tier === 'city' ? 'role-title' : (l.tier === 'town' ? 'role-header' : 'role-text-lg')}
                                style={{
                                    position: 'absolute', left: l.finalX ?? 0, top: l.finalY ?? 0, transform: `translate(-50%, -50%) scale(${zoomScale})`,
                                    fontSize: l.tier === 'city' ? '24px' : (l.tier === 'town' ? '16px' : '14px'), // Match role font adjustments (-4)
                                    color: l.isHistorical ? ARTISTIC_MAP_STYLES.colors.text.historical : ARTISTIC_MAP_STYLES.colors.text.active,
                                    textShadow: ARTISTIC_MAP_STYLES.colors.shadows.atmosphere,
                                    whiteSpace: 'nowrap',
                                    pointerEvents: 'none',
                                    zIndex: 5,
                                    opacity: fadeOpacity
                                }}
                            >
                                {l.text}
                            </div>
                        </React.Fragment>
                    );
                })}

                {/* Hot Air Balloon Aircraft Icon (Atomic from Frame) */}
                <HotAirBalloon
                    x={frame.aircraftX}
                    y={frame.aircraftY}
                    agl={frame.agl}
                />
            </div>

            {/* SVG Filter Definitions */}
            <svg style={{ position: 'absolute', width: 0, height: 0 }}>
                <defs>
                    <mask id="paper-mask" maskContentUnits="userSpaceOnUse">
                        <rect x="0" y="0" width="10000" height="10000" fill={getMaskColor(paperOpacityFog)} />
                        <path d={frame.maskPath} fill={getMaskColor(paperOpacityClear)} />
                    </mask>
                </defs>
            </svg>

            {/* Paper Overlay (Atomic from Frame) */}
            <div style={{
                position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none',
                backgroundColor: '#f4ecd8', backgroundImage: 'url(/assets/textures/paper.jpg), radial-gradient(#d4af37 1px, transparent 1px)',
                backgroundSize: 'cover, 20px 20px', zIndex: 10, mask: 'url(#paper-mask)', WebkitMask: 'url(#paper-mask)',
                filter: `saturate(${parchmentSaturation})`
            }} />
            <style>{`
                .stamped-icon { display: flex; justify-content: center; align-items: center; }
                .stamped-icon svg { width: 100%; height: 100%; overflow: visible; }
                .stamped-icon path, .stamped-icon circle, .stamped-icon rect, .stamped-icon polygon, .stamped-icon ellipse, .stamped-icon line {
                    fill: currentColor !important;
                    stroke: var(--stamped-stroke, ${ARTISTIC_MAP_STYLES.colors.icon.stroke}) !important;
                    stroke-width: var(--stamped-width, 0.8px) !important;
                    stroke-linejoin: round !important;
                    vector-effect: non-scaling-stroke;
                }
            `}</style>
        </div>
    );
};
