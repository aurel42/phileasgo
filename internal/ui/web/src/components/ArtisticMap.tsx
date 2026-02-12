import React, { useEffect, useRef, useState, useMemo } from 'react';
import maplibregl from 'maplibre-gl';
import * as turf from '@turf/turf';
import type { Feature, Polygon, MultiPolygon } from 'geojson';
import { useQueryClient } from '@tanstack/react-query';
import { useTripEvents } from '../hooks/useTripEvents';
import { useNarrator } from '../hooks/useNarrator';
import type { POI } from '../hooks/usePOIs';
import type { Telemetry } from '../types/telemetry';
import { PlacementEngine, type LabelCandidate } from '../metrics/PlacementEngine';
import { measureText, getFontFromClass } from '../metrics/text';
import { ARTISTIC_MAP_STYLES } from '../styles/artisticMapStyles';
import { InlineSVG } from './InlineSVG';
import { labelService } from '../services/labelService';
import type { LabelDTO } from '../types/mapLabels';
import { CompassRose } from './CompassRose';
import { WaxSeal } from './WaxSeal';
import { ScaleBar } from './ScaleBar';
import { InkTrail } from './InkTrail';
import { interpolatePositionFromEvents, isTransitionEvent, getSignificantTripEvents } from '../utils/replay';

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
    onPOISelect: (poi: POI) => void;
    onMapClick: () => void;
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
    font: f.font.replace(/(\d+)px/, (_, s) => `${Math.max(1, parseInt(s) + offset)} px`)
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
    const memoizedCategories = useMemo(() => settlementCategories, [JSON.stringify(settlementCategories)]);
    const queryClient = useQueryClient();
    const { status: narratorStatus } = useNarrator();
    const { data: tripEvents } = useTripEvents();

    const mapContainer = useRef<HTMLDivElement>(null);
    const map = useRef<maplibregl.Map | null>(null);
    const [styleLoaded, setStyleLoaded] = useState(false);

    const isDisconnected = telemetry?.SimState === 'disconnected';

    // Determine if we are in debriefing replay mode
    const isDebriefing = narratorStatus?.current_type === 'debriefing';
    const isIdleReplay = isDisconnected && tripEvents && tripEvents.length > 1;
    const isReplayMode = isIdleReplay || isDebriefing;

    // Sticky persistence: stay in replay view until SimState becomes active
    const [stickyReplay, setStickyReplay] = useState(false);
    useEffect(() => {
        if (isReplayMode) setStickyReplay(true);
        if (telemetry?.SimState === 'active') setStickyReplay(false);
    }, [isReplayMode, telemetry?.SimState]);

    const effectiveReplayMode = isReplayMode || stickyReplay;


    // Track replay mode transitions to force state resets/refetches
    const prevReplayModeRef = useRef(false);

    // Ref to expose effectiveReplayMode to the heartbeat closure (avoids stale capture)
    const isReplayModeRef = useRef(effectiveReplayMode);
    isReplayModeRef.current = effectiveReplayMode;

    // Default duration for idle replay is 2 mins, otherwise use actual audio duration
    const replayDuration = isDebriefing ? (narratorStatus?.current_duration_ms || 120000) : 120000;

    const firstEventTime = useMemo(() => {
        if (!tripEvents || tripEvents.length === 0) return 0;
        return new Date(tripEvents[0].timestamp).getTime();
    }, [tripEvents]);

    const totalTripTime = useMemo(() => {
        if (!tripEvents || tripEvents.length < 2) return 0;
        const last = tripEvents[tripEvents.length - 1];
        return new Date(last.timestamp).getTime() - firstEventTime;
    }, [tripEvents, firstEventTime]);

    // When entering replay mode, invalidate trip events to get fresh data and force remount of animations
    useEffect(() => {
        if (effectiveReplayMode && !prevReplayModeRef.current) {
            queryClient.invalidateQueries({ queryKey: ['tripEvents'] });
        }
        prevReplayModeRef.current = effectiveReplayMode;
    }, [effectiveReplayMode, queryClient]);

    // -- REPLAY ANIMATION STATE --
    const [progress, setProgress] = useState(0);
    const startTimeRef = useRef<number | null>(null);
    const progressRef = useRef(progress);
    useEffect(() => { progressRef.current = progress; }, [progress]);
    const animationRef = useRef<number | null>(null);
    const [replayLabels, setReplayLabels] = useState<LabelCandidate[]>([]);
    const registeredIds = useRef<Set<string>>(new Set());

    // Filter and sort events chronologically for stable replay
    const validEvents = useMemo(() => {
        const sorted = [...(tripEvents || [])].sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime());
        return getSignificantTripEvents(sorted);
    }, [tripEvents]);

    const pathPoints = useMemo((): [number, number][] => {
        return validEvents.map(e => [e.lat, e.lon] as [number, number]);
    }, [validEvents]);

    // Departure / destination airports for replay X-marks
    const { departure, destination } = useMemo(() => {
        const dep = validEvents.find(e => isTransitionEvent(e.type) && e.title?.toLowerCase().includes('take-off'));
        const dest = validEvents.slice().reverse().find(e => isTransitionEvent(e.type) && e.title?.toLowerCase().includes('landed'));
        return {
            departure: dep ? [dep.lat, dep.lon] as [number, number] : (pathPoints.length > 0 ? pathPoints[0] : null),
            destination: dest ? [dest.lat, dest.lon] as [number, number] : (pathPoints.length > 1 ? pathPoints[pathPoints.length - 1] : null),
        };
    }, [validEvents, pathPoints]);

    // Optimize POI discovery lookup (O(1) instead of O(N) in render loop)
    const poiDiscoveryTimes = useMemo(() => {
        const map = new Map<string, number>();
        validEvents.forEach(e => {
            const poiId = e.metadata?.poi_id || e.metadata?.qid;
            if (poiId) {
                map.set(poiId, new Date(e.timestamp).getTime());
            }
        });
        return map;
    }, [validEvents]);

    // Fit map to route on replay start
    useEffect(() => {
        if (effectiveReplayMode && pathPoints.length >= 2 && map.current) {
            // Lower minZoom for replay so the full route can fit
            map.current.setMinZoom(5);
            const bbox = turf.bbox({
                type: 'Feature',
                properties: {},
                geometry: { type: 'LineString', coordinates: pathPoints.map(p => [p[1], p[0]]) }
            });
            const camera = map.current.cameraForBounds(bbox as [number, number, number, number], { padding: 100 });
            console.log('[Replay] fitBounds:', { bbox, camera: camera ? { center: camera.center, zoom: camera.zoom } : null });
            if (camera) {
                map.current.easeTo({ center: camera.center, zoom: Math.min(camera.zoom || 12, 12), duration: 2000 });
            }
        }
    }, [isReplayMode, pathPoints, styleLoaded]);

    // Animation loop (mirrors TripReplayOverlay)
    useEffect(() => {
        if (pathPoints.length < 2 || !isReplayMode) {
            setProgress(0);
            startTimeRef.current = null;
            return;
        }

        startTimeRef.current = Date.now();
        const animate = () => {
            if (!startTimeRef.current) return;
            const now = Date.now();
            const elapsed = now - startTimeRef.current;
            const p = Math.min(1, elapsed / replayDuration);
            setProgress(p);

            // Cooldown for lifecycle (as in TripReplayOverlay)
            const cooldownEnd = replayDuration + 2000;
            if (elapsed < cooldownEnd) {
                animationRef.current = requestAnimationFrame(animate);
            }
        };

        animationRef.current = requestAnimationFrame(animate);
        return () => {
            if (animationRef.current) cancelAnimationFrame(animationRef.current);
        };
    }, [pathPoints.length, isReplayMode, replayDuration]);

    // Design: Sync with font loading to avoid optimistic (narrow) bounding boxes
    useEffect(() => {
        if (document.fonts) {
            document.fonts.ready.then(() => setFontsLoaded(true));
        } else {
            // Fallback for browsers without FontFaceSet
            setFontsLoaded(true);
        }
    }, []);

    const [fontsLoaded, setFontsLoaded] = useState(false);

    // Replay: Pre-calculate ALL item placements ONCE at the start
    useEffect(() => {
        if (!effectiveReplayMode || !map.current || !fontsLoaded || validEvents.length === 0) {
            if (!effectiveReplayMode) setReplayLabels([]);
            return;
        }

        const m = map.current;
        const eng = new PlacementEngine(); // Use a fresh engine instance for the one-time layout

        // Extract Fonts
        const cityFont = adjustFont(getFontFromClass('role-title'), -4);
        const townFont = adjustFont(getFontFromClass('role-header'), -4);
        const villageFont = adjustFont(getFontFromClass('role-text-lg'), -4);
        const secondaryFont = adjustFont(getFontFromClass('role-label'), 2);

        // 1. Register Settlements
        Array.from(accumulatedSettlements.current.values()).forEach(l => {
            let role = villageFont;
            let tierName: 'city' | 'town' | 'village' = 'village';
            if (l.category === 'city') { tierName = 'city'; role = cityFont; }
            else if (l.category === 'town') { tierName = 'town'; role = townFont; }

            let text = l.name.split('(')[0].split(',')[0].split('/')[0].trim();
            if (role.uppercase) text = text.toUpperCase();
            const dims = measureText(text, role.font, role.letterSpacing);

            eng.register({
                id: l.id, lat: l.lat, lon: l.lon, text, tier: tierName,
                width: dims.width, height: dims.height, type: 'settlement', score: l.pop || 0,
                isHistorical: false, size: 'L'
            });
        });

        // 2. Register ALL Trip POIs
        const processedIds = new Set<string>();
        validEvents.forEach(e => {
            if (!e.metadata) return;
            const eid = e.metadata.poi_id || e.metadata.qid;
            if (!eid || processedIds.has(eid)) return;
            processedIds.add(eid);

            const lat = e.metadata.poi_lat ? parseFloat(e.metadata.poi_lat) : e.lat;
            const lon = e.metadata.poi_lon ? parseFloat(e.metadata.poi_lon) : e.lon;
            const icon = e.metadata.icon_artistic || e.metadata.icon || 'attraction';
            const name = e.title || e.metadata.poi_name || 'Point of Interest';
            const score = e.metadata.poi_score ? parseFloat(e.metadata.poi_score) : 30;

            let secondaryLabel = undefined;
            if (score >= 10) {
                let text = name.split('(')[0].split(',')[0].split('/')[0].trim();
                if (secondaryFont.uppercase) text = text.toUpperCase();
                const dims = measureText(text, secondaryFont.font, secondaryFont.letterSpacing);
                secondaryLabel = { text, width: dims.width, height: dims.height };
            }

            eng.register({
                id: eid, lat, lon, text: "", tier: 'village', score,
                width: 26, height: 26, type: 'poi', isHistorical: false,
                size: (e.metadata.poi_size || 'M') as any, icon,
                secondaryLabel
            });
        });

        // Use a small delay to ensure the map scale/bounds are stable after the initial fitBounds
        const timeout = setTimeout(() => {
            const w = m.getCanvas().clientWidth;
            const h = m.getCanvas().clientHeight;
            const placed = eng.compute(
                (lat, lon) => {
                    const pos = m.project([lon, lat]);
                    return { x: pos.x, y: pos.y };
                },
                w, h, m.getZoom()
            );

            console.log('[Replay] Static Placement Complete:', placed.length, 'items');
            setReplayLabels(placed);
        }, 500);

        return () => clearTimeout(timeout);
    }, [effectiveReplayMode, fontsLoaded, validEvents.length]);

    // -- Placement Engine (Persistent across ticks) --
    const engine = useMemo(() => new PlacementEngine(), []);

    const currentNarratedId = (narratorStatus?.playback_status === 'playing' || narratorStatus?.playback_status === 'paused')
        ? narratorStatus?.current_poi?.wikidata_id : undefined;
    const preparingId = narratorStatus?.preparing_poi?.wikidata_id;

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

    // Tracking for damped solver execution
    const lastPoiCount = useRef(0);
    const lastLabelsJson = useRef("");
    const lastPlacementView = useRef<{ lng: number, lat: number, zoom: number } | null>(null);

    const replayBalloonPos = useMemo(() => {
        if (!effectiveReplayMode || pathPoints.length < 2 || !map.current) return null;
        const { position } = interpolatePositionFromEvents(validEvents, progress);
        const pt = map.current.project([position[1], position[0]]);
        return { x: pt.x, y: pt.y };
    }, [effectiveReplayMode, pathPoints, progress, styleLoaded, frame.zoom, frame.center, frame.offset, validEvents]);
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

    // -- Data Refs for Heartbeat (Reactive Sync) --
    const telemetryRef = useRef(telemetry);
    const poisRef = useRef(pois);
    const zoomRef = useRef(zoom);
    const lastSyncLabelsRef = useRef(lastSyncLabels);
    const replayLabelsRef = useRef(replayLabels);
    const validEventsRef = useRef(validEvents);
    const poiDiscoveryTimesRef = useRef(poiDiscoveryTimes);
    const totalTripTimeRef = useRef(totalTripTime);

    useEffect(() => { telemetryRef.current = telemetry; }, [telemetry]);
    useEffect(() => { poisRef.current = pois; }, [pois]);
    useEffect(() => { zoomRef.current = zoom; }, [zoom]);
    useEffect(() => { lastSyncLabelsRef.current = lastSyncLabels; }, [lastSyncLabels]);
    useEffect(() => { replayLabelsRef.current = replayLabels; }, [replayLabels]);
    useEffect(() => { validEventsRef.current = validEvents; }, [validEvents]);
    useEffect(() => { poiDiscoveryTimesRef.current = poiDiscoveryTimes; }, [poiDiscoveryTimes]);
    useEffect(() => { totalTripTimeRef.current = totalTripTime; }, [totalTripTime]);

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
        let lastLabels: LabelCandidate[] = [];
        let lastMask: string = '';

        const tick = async () => {
            if (isRunning) return;
            isRunning = true;

            const effectiveReplayMode = isReplayModeRef.current;
            const currentReplayLabels = replayLabelsRef.current;

            // Stability Bypass: If we are in replay mode and already have our static layout,
            // we stop the heartbeat to preserve CPU and ensure absolute position freezing.
            if (effectiveReplayMode && currentReplayLabels.length > 0) {
                isRunning = false;
                return;
            }

            try {
                // Design Section 6: Ensure fonts are loaded before measurement
                if (document.fonts) {
                    await document.fonts.ready;
                }
                const m = map.current;
                const t = telemetryRef.current;
                const currentValidEvents = validEventsRef.current;

                if (!m || !t || (!effectiveReplayMode && t.SimState === 'disconnected')) {
                    isRunning = false;
                    return;
                }

                // 1. Unified Aircraft State (Live vs Replay)
                const acState = effectiveReplayMode && currentValidEvents.length >= 2
                    ? (() => {
                        const interp = interpolatePositionFromEvents(currentValidEvents, progressRef.current);
                        return { lat: interp.position[0], lon: interp.position[1], heading: interp.heading };
                    })()
                    : { lat: t.Latitude, lon: t.Longitude, heading: t.Heading };

                // 1. Snapshot State
                const bounds = m.getBounds();
                const targetZoomBase = zoomRef.current;

                if (!bounds) {
                    isRunning = false;
                    return;
                }

                // 2. BACKGROUND FETCH (Visibility Mask)
                const fetchMask = async () => {
                    try {
                        const maskRes = await fetch(`/api/map/visibility-mask?bounds=${bounds.getNorth()},${bounds.getEast()},${bounds.getSouth()},${bounds.getWest()}&resolution=20`);
                        if (maskRes.ok) lastMaskData = await maskRes.json();
                    } catch (e) {
                        console.error("Background Mask Fetch Failed:", e);
                    }
                };
                const simActive = t.SimState === 'active';
                if (simActive) {
                    if (firstTick) {
                        await fetchMask();
                    } else {
                        fetchMask();
                    }
                }

                const mapWidth = m.getCanvas().clientWidth;
                const mapHeight = m.getCanvas().clientHeight;

                // 3. DEAD-ZONE PANNING — re-center when more map is behind aircraft than ahead
                let needsRecenter = !lockedCenter; // First tick always centers

                if (lockedCenter) {
                    const currentPos: [number, number] = [acState.lon, acState.lat];
                    const aircraftOnMap = m.project(currentPos);
                    const hdgRad = acState.heading * (Math.PI / 180);

                    const adx = Math.sin(hdgRad);
                    const ady = -Math.cos(hdgRad);

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
                    const hdgRad = acState.heading * (Math.PI / 180);
                    const dx = offsetPx * Math.sin(hdgRad);
                    const dy = -offsetPx * Math.cos(hdgRad);

                    lockedCenter = [acState.lon, acState.lat];
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
                    lockedZoom = Math.round(newZoom * 2) / 2;
                }

                if (effectiveReplayMode) {
                    // PASSIVE REPLAY: Read camera from map instance (controlled by fitBounds/animations)
                    const c = m.getCenter();
                    lockedCenter = [c.lng, c.lat];
                    lockedZoom = m.getZoom();
                    lockedOffset = [0, 0];
                    console.log('[Replay] tick: passive camera', { center: lockedCenter, zoom: lockedZoom });
                } else if (needsRecenter) {
                    // ACTIVE FLIGHT: Force-snap the map to follow the aircraft
                    m.easeTo({ center: lockedCenter as maplibregl.LngLatLike, zoom: lockedZoom, offset: lockedOffset, duration: 0 });
                }

                // -- SMART SYNC: Fetch labels on move/snap (backend handles density limit) --
                const b = m.getBounds();
                labelService.fetchLabels({
                    bbox: [b.getSouth(), b.getWest(), b.getNorth(), b.getEast()],
                    ac_lat: acState.lat,
                    ac_lon: acState.lon,
                    heading: acState.heading,
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
                } else if (!results.wasPlaced && lastLabels.length > 0 && results.id === nextCompass.id) {
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
                    if (!effectiveReplayMode) {
                        pruneOffscreen(m.getBounds());
                    }
                    prevZoomInt = currentZoomSnap;
                }

                // 5. PROJECT using the STABLE map state
                const aircraftPos = m.project([acState.lon, acState.lat]);
                const aircraftX = Math.round(aircraftPos.x);
                const aircraftY = Math.round(aircraftPos.y);
                if (effectiveReplayMode) {
                    console.log('[Replay] tick: aircraft projected', { currentPos: [acState.lon, acState.lat], aircraftX, aircraftY, mapSize: [mapWidth, mapHeight] });
                }

                // 7. BEARING LINE
                const lat1 = acState.lat * Math.PI / 180;
                const lon1 = acState.lon * Math.PI / 180;
                const brng = acState.heading * Math.PI / 180;
                const R = 3440.065; const d = 50.0;
                const lat2 = Math.asin(Math.sin(lat1) * Math.cos(d / R) + Math.cos(lat1) * Math.sin(d / R) * Math.cos(brng));
                const lon2 = lon1 + Math.atan2(Math.sin(brng) * Math.sin(d / R) * Math.cos(lat1), Math.cos(d / R) - Math.sin(lat1) * Math.sin(lat2));

                const bLine: Feature<any> = {
                    type: 'Feature', properties: {},
                    geometry: { type: 'LineString', coordinates: [[acState.lon, acState.lat], [lon2 * 180 / Math.PI, lat2 * 180 / Math.PI]] }
                };
                const lineSource = m.getSource('bearing-line') as maplibregl.GeoJSONSource;
                if (lineSource) lineSource.setData(bLine);

                // 9. DAMPED COLLISION SOLVER
                // Only execute if geography (snap/pan) or data (discovery) changes.
                const currentSyncLabels = lastSyncLabelsRef.current;
                const currentPois = poisRef.current;
                const labelsJson = JSON.stringify(currentSyncLabels.map(l => l.id));
                const dataChanged = currentPois.length !== lastPoiCount.current || labelsJson !== lastLabelsJson.current;
                const viewChanged = !lastPlacementView.current || !lockedCenter ||
                    Math.abs(lastPlacementView.current.zoom - lockedZoom) > 0.05 ||
                    Math.abs(lastPlacementView.current.lng - lockedCenter![0]) > 0.0001 ||
                    Math.abs(lastPlacementView.current.lat - lockedCenter![1]) > 0.0001;

                let labels = lastLabels;
                let finalMaskPath = lastMask;

                const isSnap = needsRecenter || viewChanged || firstTick;
                if (isSnap) {
                    engine.clear();
                    registeredIds.current.clear();
                }

                if (isSnap || dataChanged) {
                    // Extract Fonts from CSS Roles
                    const cityFont = adjustFont(getFontFromClass('role-title'), -4);
                    const townFont = adjustFont(getFontFromClass('role-header'), -4);
                    const villageFont = adjustFont(getFontFromClass('role-text-lg'), -4);
                    const secondaryFont = adjustFont(getFontFromClass('role-label'), 2);

                    // 1. Process Sync Labels (Settlements from DB)
                    Array.from(accumulatedSettlements.current.values()).forEach(l => {
                        if (registeredIds.current.has(l.id)) return;

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
                            id: l.id, lat: l.lat, lon: l.lon, text, tier: tierName,
                            width: dims.width, height: dims.height, type: 'settlement', score: l.pop || 0,
                            isHistorical: false, size: 'L'
                        });
                        registeredIds.current.add(l.id);
                    });

                    // 1.5. Process Compass Rose
                    if (nextCompass && !registeredIds.current.has(nextCompass.id)) {
                        const size = 58;
                        engine.register({
                            id: nextCompass.id, lat: nextCompass.lat, lon: nextCompass.lon,
                            text: "", tier: 'village', score: 200,
                            width: size, height: size, type: 'compass', isHistorical: false
                        });
                        registeredIds.current.add(nextCompass.id);
                    }

                    // 2. Identify the "Champion POI" for labeling
                    const settlementCatSet = new Set(settlementCategories.map(c => c.toLowerCase()));
                    let champion: POI | null = null;

                    currentPois.forEach(p => {
                        if (settlementCatSet.has(p.category?.toLowerCase())) return;

                        const normalizedName = p.name_en.split('(')[0].split(',')[0].split('/')[0].trim();
                        if (normalizedName.length > 24) return;

                        if (p.score >= 10 && !failedPoiLabelIds.current.has(p.wikidata_id) && !labeledPoiIds.current.has(p.wikidata_id)) {
                            if (!champion || (p.score > champion.score)) {
                                champion = p;
                            }
                        }
                    });

                    let registeredNewPoi = false;
                    if (effectiveReplayMode) {
                        const totalTime = totalTripTimeRef.current;
                        const discoveryTimes = poiDiscoveryTimesRef.current;
                        const simulatedElapsed = progressRef.current * totalTime;
                        const sizePx = 26;
                        const processedIds = new Set<string>();

                        currentValidEvents.forEach(e => {
                            if (!e.metadata) return;
                            const eid = e.metadata.poi_id || e.metadata.qid;
                            if (!eid || processedIds.has(eid)) return;
                            processedIds.add(eid);

                            // DISCOVERY-AWARE REGISTRATION:
                            const discoveryTime = discoveryTimes.get(eid);
                            if (discoveryTime != null) {
                                const eventElapsed = discoveryTime - firstEventTime;
                                if (eventElapsed > simulatedElapsed) return;
                            }

                            // If already registered AND not a candidate for secondary label, skip.
                            if (registeredIds.current.has(eid)) return;

                            const lat = e.metadata.poi_lat ? parseFloat(e.metadata.poi_lat) : e.lat;
                            const lon = e.metadata.poi_lon ? parseFloat(e.metadata.poi_lon) : e.lon;
                            if (!lat || !lon) return;
                            const icon = e.metadata.icon_artistic || e.metadata.icon || 'attraction';
                            const name = e.title || e.metadata.poi_name || 'Point of Interest';
                            const score = e.metadata.poi_score ? parseFloat(e.metadata.poi_score) : 30;

                            let secondaryLabel = undefined;
                            if (score >= 10) {
                                let text = name.split('(')[0].split(',')[0].split('/')[0].trim();
                                if (secondaryFont.uppercase) text = text.toUpperCase();
                                const dims = measureText(text, secondaryFont.font, secondaryFont.letterSpacing);
                                secondaryLabel = { text, width: dims.width, height: dims.height };
                            }

                            engine.register({
                                id: eid, lat, lon, text: "", tier: 'village', score,
                                width: sizePx, height: sizePx, type: 'poi', isHistorical: false,
                                size: (e.metadata.poi_size || 'M') as any, icon,
                                secondaryLabel
                            });
                            registeredIds.current.add(eid);
                            registeredNewPoi = true;
                        });
                    } else {
                        // Live flight: use accumulated POIs from /api/pois/tracked
                        currentPois.forEach(p => {
                            if (!p.lat || !p.lon) return;
                            accumulatedPois.current.set(p.wikidata_id, p);
                        });

                        Array.from(accumulatedPois.current.values()).forEach(p => {
                            const isChampion = champion && p.wikidata_id === champion.wikidata_id;
                            const isLabeled = labeledPoiIds.current.has(p.wikidata_id);
                            const needsSecondary = isChampion || isLabeled;

                            // Skip if already registered AND doesn't need a label update
                            if (registeredIds.current.has(p.wikidata_id) && !needsSecondary) return;

                            const sizePx = 26;
                            const isHistorical = !!(p.last_played && p.last_played !== "0001-01-01T00:00:00Z");

                            let secondaryLabel = undefined;
                            if (needsSecondary) {
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
                            registeredIds.current.add(p.wikidata_id);
                            registeredNewPoi = true;
                        });
                    }

                    if (isSnap || registeredNewPoi) {
                        labels = engine.compute(
                            (lat: number, lon: number) => {
                                // PROJECT using the TARGET map state (the one we're about to set)
                                // This ensures the labels are placed relative to where the map WILL be.
                                const pos = m.project([lon, lat]);

                                // Adjust for the offset we're about to apply
                                return {
                                    x: pos.x - (lockedOffset[0] || 0),
                                    y: pos.y - (lockedOffset[1] || 0)
                                };
                            },
                            mapWidth,
                            mapHeight,
                            lockedZoom
                        );

                        // Update fail/success memory for Champion
                        if (champion) {
                            const placedChamp = labels.find(l => l.id === (champion as POI).wikidata_id);
                            if (placedChamp && placedChamp.secondaryLabel) {
                                if (placedChamp.secondaryLabelPos) {
                                    labeledPoiIds.current.add((champion as POI).wikidata_id);
                                } else {
                                    failedPoiLabelIds.current.add((champion as POI).wikidata_id);
                                }
                            }
                        }

                        finalMaskPath = lastMaskData ? maskToPath(lastMaskData, m) : '';

                        // Update tracking refs
                        lastPoiCount.current = currentPois.length;
                        lastLabelsJson.current = labelsJson;
                        lastPlacementView.current = { lng: lockedCenter![0], lat: lockedCenter![1], zoom: lockedZoom };
                    }

                    // 10. COMMIT
                    lastLabels = labels;
                    lastMask = finalMaskPath;

                    if (effectiveReplayMode) {
                        console.log('[Replay] tick: committing frame', { center: lockedCenter, zoom: targetZoom, aircraftX, aircraftY, labelCount: labels.length });
                    }
                    setFrame(prev => ({
                        ...prev,
                        maskPath: finalMaskPath,
                        center: lockedCenter!,
                        zoom: targetZoom,
                        offset: lockedOffset,
                        heading: t.Heading,
                        bearingLine: bLine,
                        aircraftX,
                        aircraftY,
                        agl: t.AltitudeAGL,
                        labels: labels
                    }));
                }
            } catch (err) {
                console.error("Heartbeat Loop Crash:", err);
            } finally {
                isRunning = false;
            }
        };

        tick();
        const interval = setInterval(tick, 2000);
        return () => clearInterval(interval);
    }, [styleLoaded, memoizedCategories]);

    // Discovery filter is now in the render loop. Placement is in the heartbeat.

    const maskToPath = (geojson: Feature<Polygon | MultiPolygon>, mapInstance: maplibregl.Map): string => {
        if (!geojson.geometry) return '';
        const coords = geojson.geometry.type === 'Polygon' ? [geojson.geometry.coordinates] : (geojson.geometry as MultiPolygon).coordinates;
        return coords.map((poly: any) => poly.map((ring: any) => ring.map((coord: any) => {
            const p = mapInstance.project([coord[0], coord[1]]);
            return `${p.x},${p.y} `;
        }).join(' L ')).map((ringStr: string) => `M ${ringStr} Z`).join(' ')).join(' ');
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
                {effectiveReplayMode && pathPoints.length >= 2 && map.current && (
                    <div style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 10 }}>
                        <InkTrail
                            pathPoints={pathPoints}
                            validEvents={validEvents}
                            progress={progress}
                            departure={departure}
                            destination={destination}
                            project={(lnglat) => {
                                const pt = map.current!.project(lnglat);
                                return { x: pt.x, y: pt.y };
                            }}
                        />
                    </div>
                )}
                {/* Labels Overlay */}
                {(effectiveReplayMode ? replayLabels : frame.labels).map(l => {
                    // Replay Discovery Filter: Skip rendering if not yet discovered
                    if (effectiveReplayMode && l.type === 'poi') {
                        const discoveryTime = poiDiscoveryTimes.get(l.id);
                        if (discoveryTime != null) {
                            const eventElapsed = discoveryTime - firstEventTime;
                            const simulatedElapsed = progress * totalTripTime;
                            // Only apply filter if we've discovered it and we're currently animating (progress < 1)
                            if (progress < 0.999 && eventElapsed > simulatedElapsed) return null;
                        }
                    }

                    // RESOLUTION: Project coordinates in the render loop to ensure labels stay locked to the map
                    // background during pans, avoiding the "lag" associated with the 2s heartbeat.
                    const m = map.current;
                    if (!m) return null;

                    const geoPos = m.project([l.lon, l.lat]);
                    // Zoom-relative scale: markers shrink/grow to stay the same size on the map surface
                    let zoomScale = l.placedZoom != null ? Math.pow(2, frame.zoom - l.placedZoom) : 1;
                    if (l.type === 'settlement') zoomScale = Math.min(zoomScale, 1.0);

                    // APPLY relative offsets from the placement engine to maintain collision avoidance
                    // while still following the map during pans.
                    const offsetX = ((l.finalX ?? 0) - (l.trueX ?? 0)) * zoomScale;
                    const offsetY = ((l.finalY ?? 0) - (l.trueY ?? 0)) * zoomScale;
                    const finalX = geoPos.x + offsetX;
                    const finalY = geoPos.y + offsetY;

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
                        const tTx = geoPos.x;
                        const tTy = geoPos.y;
                        const tDx = finalX - tTx;
                        const tDy = finalY - tTy;
                        const tDist = Math.ceil(Math.sqrt(tDx * tDx + tDy * tDy));
                        const isDisplaced = !effectiveIsHistorical && tDist > 68;

                        const score = l.score || 0;
                        const isHero = score >= 20 && !effectiveIsHistorical;
                        const isReplayItem = isReplayMode && isReplayModeRef.current;

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
                        const isDeferred = !isReplayItem && (poi?.is_deferred || poi?.badges?.includes('deferred'));

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

                        if (silhouette) {
                            iconColor = '#000000';
                            outlineColor = '#ffffff';
                            hColor = '#ffffff';
                            hSize = 3;
                            hLayers = 1;
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
                        const startX = geoPos.x;
                        const startY = geoPos.y;
                        const endX = finalX;
                        const endY = finalY;
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
                                        position: 'absolute', left: finalX, top: finalY,
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
                                        position: 'absolute', left: finalX, top: finalY, width: l.width, height: l.height,
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
                                            // @ts-ignore - custom CSS variables for the stamped-icon class
                                            '--stamped-stroke': outlineColor,
                                            '--stamped-width': `${outlineWeight}px`
                                        }}
                                        className="stamped-icon"
                                    />
                                </div>

                                {l.secondaryLabel && l.secondaryLabelPos && (
                                    <div
                                        className="role-label"
                                        style={{
                                            position: 'absolute',
                                            // Project the secondary label anchor displacement relative to the dynamic final coordinate
                                            left: finalX + ((l.secondaryLabelPos.x - (l.finalX ?? 0)) * zoomScale),
                                            top: finalY + ((l.secondaryLabelPos.y - (l.finalY ?? 0)) * zoomScale),
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
                                    position: 'absolute', left: finalX, top: finalY, width: l.width, height: l.height,
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
                                    position: 'absolute', left: finalX, top: finalY, transform: `translate(-50%, -50%) scale(${zoomScale})`,
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
                    x={(isReplayMode || effectiveReplayMode) && replayBalloonPos ? replayBalloonPos.x : frame.aircraftX}
                    y={(isReplayMode || effectiveReplayMode) && replayBalloonPos ? replayBalloonPos.y : frame.aircraftY}
                    agl={(isReplayMode || effectiveReplayMode) ? 5000 : frame.agl}
                />
            </div>

            {/* SVG Filter Definitions */}
            <svg style={{ position: 'absolute', width: 0, height: 0 }}>
                <defs>
                    <mask id="paper-mask" maskContentUnits="userSpaceOnUse">
                        <rect x="0" y="0" width="10000" height="10000" fill={getMaskColor(isReplayMode ? paperOpacityClear : paperOpacityFog)} />
                        {!isReplayMode && <path d={frame.maskPath} fill={getMaskColor(paperOpacityClear)} />}
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
    fill: currentColor!important;
    stroke: var(--stamped-stroke, ${ARTISTIC_MAP_STYLES.colors.icon.stroke}) !important;
    stroke-width: var(--stamped-width, 0.8px) !important;
    stroke-linejoin: round!important;
    vector-effect: non-scaling-stroke;
}
`}</style>
        </div>
    );
};
