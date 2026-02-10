import React, { useEffect, useRef, useState, useMemo } from 'react';
import maplibregl from 'maplibre-gl';
import * as turf from '@turf/turf';
import type { Feature, Polygon, MultiPolygon } from 'geojson';
import 'maplibre-gl/dist/maplibre-gl.css';
import type { Telemetry } from '../types/telemetry';
import type { POI } from '../hooks/usePOIs';
import { PlacementEngine, type LabelCandidate } from '../metrics/PlacementEngine';
import { measureText } from '../metrics/text';
import { ARTISTIC_MAP_STYLES } from '../styles/artisticMapStyles';
import { InlineSVG } from './InlineSVG';
import { useNarrator } from '../hooks/useNarrator';

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
    settlementLabelLimit: number;
    settlementTier: number;
    paperOpacityFog: number;
    paperOpacityClear: number;
    parchmentSaturation: number;
    onPOISelect?: (poi: POI) => void;
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

export const ArtisticMap: React.FC<ArtisticMapProps> = ({
    className,
    center,
    zoom,
    telemetry,
    pois,
    settlementLabelLimit,
    settlementTier,
    paperOpacityFog = 0.7,
    paperOpacityClear = 0.1,
    parchmentSaturation = 1.0,
    onPOISelect
}) => {
    const mapContainer = useRef<HTMLDivElement>(null);
    const map = useRef<maplibregl.Map | null>(null);
    const [styleLoaded, setStyleLoaded] = useState(false);

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
    const limitRef = useRef(settlementLabelLimit);

    useEffect(() => { telemetryRef.current = telemetry; }, [telemetry]);
    useEffect(() => { poisRef.current = pois; }, [pois]);
    useEffect(() => { zoomRef.current = zoom; }, [zoom]);
    useEffect(() => { limitRef.current = settlementLabelLimit; }, [settlementLabelLimit]);
    // settlementTier is read directly from props in render loop or ref if needed
    const tierRef = useRef(settlementTier);
    useEffect(() => { tierRef.current = settlementTier; }, [settlementTier]);

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

    const accumulatedSettlements = useRef<Map<string, POI>>(new Map());
    const accumulatedPois = useRef<Map<string, POI>>(new Map());

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
                    'stamen-watercolor': {
                        type: 'raster',
                        tiles: ['https://watercolormaps.collection.cooperhewitt.org/tile/watercolor/{z}/{x}/{y}.jpg'],
                        tileSize: 256
                    }
                },
                layers: [
                    { id: 'background', type: 'background', paint: { 'background-color': '#f4ecd8' } },
                    { id: 'watercolor', type: 'raster', source: 'stamen-watercolor', paint: { 'raster-saturation': -0.2, 'raster-contrast': 0.1 } }
                ]
            },
            center: [center[1], center[0]],
            zoom: zoom,
            minZoom: 10,
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
        let lastSettlements: POI[] = [];
        let lastTierIndex: number = -1;
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
                const container = mapContainer.current;

                if (!m || !t || (t.Latitude === 0 && t.Longitude === 0)) return;

                // 1. Snapshot State
                const bounds = m.getBounds();
                const z = m.getZoom();
                const currentPois = poisRef.current;
                const targetZoomBase = zoomRef.current;

                if (!bounds) return;

                // 2. BACKGROUND FETCH (Non-blocking)
                const fetchExtras = async () => {
                    try {
                        const [maskRes, settRes] = await Promise.all([
                            fetch(`/api/map/visibility-mask?bounds=${bounds.getNorth()},${bounds.getEast()},${bounds.getSouth()},${bounds.getWest()}&resolution=20`),
                            fetch(`/api/map/settlements?minLat=${bounds.getSouth()}&maxLat=${bounds.getNorth()}&minLon=${bounds.getWest()}&maxLon=${bounds.getEast()}&zoom=${z}`)
                        ]);
                        if (maskRes.ok) lastMaskData = await maskRes.json();
                        if (settRes.ok) {
                            const data = await settRes.json();
                            // Handle SettlementResponse wrapper { tier_index: X, items: [...] }
                            lastSettlements = (data && Array.isArray(data.items)) ? data.items : [];
                            lastTierIndex = data?.tier_index ?? -1;
                        }
                    } catch (e) {
                        console.error("Background Data Fetch Failed:", e);
                    }
                };
                if (firstTick) {
                    await fetchExtras();
                    firstTick = false;
                } else {
                    fetchExtras();
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
                        if (coneBbox && !coneBbox.some(isNaN)) {
                            const camera = m.cameraForBounds(coneBbox as [number, number, number, number], { padding: 120, maxZoom: 13 });
                            if (camera?.zoom !== undefined && !isNaN(camera.zoom)) {
                                newZoom = Math.min(Math.max(camera.zoom, 10), 13);
                            }
                        }
                    }
                    lockedZoom = newZoom;

                    // Move map to locked position BEFORE projecting
                    m.easeTo({ center: lockedCenter, zoom: lockedZoom, offset: lockedOffset, duration: 0 });
                }

                const targetZoom = lockedZoom;

                // Tileset level change detection
                const currentZoomInt = Math.floor(targetZoom);
                if (prevZoomInt === -1) prevZoomInt = currentZoomInt;

                if (currentZoomInt !== prevZoomInt) {
                    accumulatedSettlements.current.clear();
                    accumulatedPois.current.clear();
                    engine.resetCache();
                    prevZoomInt = currentZoomInt;
                }

                let newSettlements = Array.isArray(lastSettlements) ? (limitRef.current !== -1 ? lastSettlements.slice(0, limitRef.current) : [...lastSettlements]) : [];
                newSettlements.forEach(s => {
                    const id = s.wikidata_id || `${s.lat}-${s.lon}`;
                    accumulatedSettlements.current.set(id, s);
                });

                // 5. PROJECT using the STABLE map state (m.project is now deterministic)
                const aircraftPos = m.project([t.Longitude, t.Latitude]);
                const aircraftX = Math.round(aircraftPos.x);
                const aircraftY = Math.round(aircraftPos.y);

                const projector = (lat: number, lon: number) => {
                    const p = m.project([lon, lat]);
                    return { x: p.x, y: p.y };
                };

                const vw = container?.clientWidth || window.innerWidth;
                const vh = container?.clientHeight || window.innerHeight;

                // No viewport pruning during smooth zoom — POIs are only cleared on tileset level change (above)

                engine.clear();

                // Register Settlements
                // 1. Inject POIs that are settlements into the accumulation map
                //    This ensures any tracked settlement gets a label, even if the dedicated API missed it.
                currentPois.forEach(p => {
                    const cat = (p.category || '').toLowerCase();
                    let tierMatch = -1;
                    if (cat === 'city') tierMatch = 1;
                    else if (cat === 'town') tierMatch = 2;
                    else if (cat === 'village') tierMatch = 3;

                    // If it is a settlement AND fits within current tier setting
                    // (tierRef.current: 0=None, 1=City, 2=City+Town, 3=All)
                    if (tierMatch > 0 && tierMatch <= tierRef.current) {
                        const id = p.wikidata_id;
                        if (!accumulatedSettlements.current.has(id)) {
                            // Convert POI to simplified sync format for label engine
                            accumulatedSettlements.current.set(id, p);
                        }
                    }
                });

                Array.from(accumulatedSettlements.current.values()).forEach(f => {
                    let font = ARTISTIC_MAP_STYLES.fonts.village.cssFont;
                    let tierName: 'city' | 'town' | 'village' = 'village';

                    // Priority from backend index (root of response)
                    if (lastTierIndex === 0) {
                        font = ARTISTIC_MAP_STYLES.fonts.city.cssFont;
                        tierName = 'city';
                    } else if (lastTierIndex === 1) {
                        font = ARTISTIC_MAP_STYLES.fonts.town.cssFont;
                        tierName = 'town';
                    } else if (lastTierIndex === 2) {
                        font = ARTISTIC_MAP_STYLES.fonts.village.cssFont;
                        tierName = 'village';
                    } else {
                        // Fallback logic
                        const cat = (f.category as string || '').toLowerCase();
                        if (cat === 'city') { font = ARTISTIC_MAP_STYLES.fonts.city.cssFont; tierName = 'city'; }
                        else if (cat === 'town') { font = ARTISTIC_MAP_STYLES.fonts.town.cssFont; tierName = 'town'; }
                    }

                    const text = (f.name_user || f.name_en || "").split(',')[0].split('/')[0].trim();
                    // Use exact cssFont for measurement to match rendered style
                    const dims = measureText(text, font);
                    const itemIsHistorical = !!(f.last_played && f.last_played !== "0001-01-01T00:00:00Z");
                    engine.register({
                        id: f.wikidata_id || `${f.lat}-${f.lon}`, lat: f.lat, lon: f.lon, text, tier: tierName,
                        width: dims.width, height: dims.height, type: 'settlement', score: f.score || 0, isHistorical: itemIsHistorical, size: 'L'
                    });
                });

                // Register POIs (Persistent Accumulator)
                currentPois.forEach(p => {
                    accumulatedPois.current.set(p.wikidata_id, p);
                });

                Array.from(accumulatedPois.current.values()).forEach(p => {
                    if (!p.lat || !p.lon) return;
                    const sizePx = 26; // Increased from 20px by ~30% per user request

                    const iconName = p.icon || 'attraction';
                    const isHistorical = !!(p.last_played && p.last_played !== "0001-01-01T00:00:00Z");
                    engine.register({
                        id: p.wikidata_id, lat: p.lat, lon: p.lon, text: "", tier: 'village', score: p.score || 0,
                        width: sizePx, height: sizePx, type: 'poi', isHistorical, size: p.size as any, icon: iconName,
                        visibility: p.visibility
                    });
                });

                const snapshotLabels = engine.compute(projector, vw, vh, targetZoom);

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

                // 8. COMMIT
                setFrame({
                    labels: snapshotLabels,
                    maskPath: lastMaskData ? maskToPath(lastMaskData, m) : '',
                    center: lockedCenter!,
                    zoom: targetZoom,
                    offset: lockedOffset,
                    heading: t.Heading,
                    bearingLine: bLine,
                    aircraftX,
                    aircraftY,
                    agl: t.AltitudeAGL
                });
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

    const maskToPath = (geojson: Feature<Polygon | MultiPolygon>, mapInstance: maplibregl.Map): string => {
        if (!geojson.geometry) return '';
        const coords = geojson.geometry.type === 'Polygon' ? [geojson.geometry.coordinates] : geojson.geometry.coordinates;
        return coords.map(poly => poly.map(ring => ring.map(coord => {
            const p = mapInstance.project([coord[0], coord[1]]);
            return `${p.x},${p.y}`;
        }).join(' L ')).map(ringStr => `M ${ringStr} Z`).join(' ')).join(' ');
    };

    return (
        <div className={className} style={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden' }}>
            <div ref={mapContainer} style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', color: 'black' }} />

            {/* Labels Overlay (Atomic from Frame) */}
            <div style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 20 }}>
                {frame.labels.map(l => {
                    // Use tick-computed trueX/trueY (not live projection) to stay in sync with finalX/finalY
                    const tx = l.trueX ?? 0;
                    const ty = l.trueY ?? 0;
                    const dx = (l.finalX || 0) - tx;
                    const dy = (l.finalY || 0) - ty;
                    const dist = Math.ceil(Math.sqrt(dx * dx + dy * dy));
                    const isDisplaced = dist > 30;

                    // Zoom-relative scale: markers shrink/grow to stay the same size on the map surface
                    const zoomScale = l.placedZoom != null ? Math.pow(2, frame.zoom - l.placedZoom) : 1;

                    if (l.type === 'poi') {
                        // ... POI Icon Rendering ...
                        const iconUrl = `/icons/${l.icon || 'attraction'}.svg`;
                        // Icon color: Selected > Next > Score-based metallic
                        let iconColor = ARTISTIC_MAP_STYLES.colors.icon.copper;
                        if (l.id === currentNarratedId) {
                            iconColor = ARTISTIC_MAP_STYLES.colors.icon.selected;
                        } else if (l.id === preparingId) {
                            iconColor = ARTISTIC_MAP_STYLES.colors.icon.next;
                        } else if (l.isHistorical) {
                            iconColor = ARTISTIC_MAP_STYLES.colors.icon.historical;
                        } else if (l.score > 20) {
                            iconColor = ARTISTIC_MAP_STYLES.colors.icon.gold;
                        } else if (l.score > 10) {
                            iconColor = ARTISTIC_MAP_STYLES.colors.icon.silver;
                        }
                        const activeBoost = l.id === currentNarratedId ? 1.5 : l.id === preparingId ? 1.25 : 1;

                        const sway = 15;
                        // Deterministic sway direction based on ID to keep it stable but organic
                        const swayDir = (l.id.charCodeAt(0) % 2 === 0 ? 1 : -1);
                        const startX = l.trueX || 0;
                        const startY = l.trueY || 0;
                        const endX = l.finalX || 0;
                        const endY = l.finalY || 0;
                        const dy = endY - startY;

                        // Cubic Bezier with 2 control points pulling in opposite directions
                        // CP1 pulls Right (relative to swayDir) at 1/3 distance
                        // CP2 pulls Left (relative to swayDir) at 2/3 distance
                        const cp1X = startX + (sway * swayDir);
                        const cp1Y = startY + (dy * 0.33);
                        const cp2X = endX - (sway * swayDir);
                        const cp2Y = startY + (dy * 0.66);

                        return (
                            <React.Fragment key={l.id}>
                                {isDisplaced && (
                                    <svg style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 15 }}>
                                        <path d={`M ${startX},${startY} C ${cp1X},${cp1Y} ${cp2X},${cp2Y} ${endX},${endY}`}
                                            fill="none" stroke={ARTISTIC_MAP_STYLES.tethers.stroke} strokeWidth={ARTISTIC_MAP_STYLES.tethers.width} opacity={ARTISTIC_MAP_STYLES.tethers.opacity} />
                                        <circle cx={startX} cy={startY} r={ARTISTIC_MAP_STYLES.tethers.dotRadius} fill={ARTISTIC_MAP_STYLES.tethers.stroke} opacity={ARTISTIC_MAP_STYLES.tethers.dotOpacity} />
                                    </svg>
                                )}
                                <div
                                    onClick={() => {
                                        const poi = accumulatedPois.current.get(l.id);
                                        if (poi && onPOISelect) onPOISelect(poi);
                                    }}
                                    style={{
                                        position: 'absolute', left: l.finalX ?? 0, top: l.finalY ?? 0, width: l.width, height: l.height,
                                        transform: `translate(-50%, -50%) scale(${zoomScale * activeBoost})`,
                                        opacity: l.isHistorical ? 0.8 : 1, // Fade historic POIs slightly per user request
                                        color: iconColor, cursor: 'pointer', pointerEvents: 'auto',
                                        // Use drop-shadow filter for true shape contour ("Halo")
                                        // Selected/Next get glowing/colored halos, Normal gets paper-colored cutout halo
                                        filter: l.id === currentNarratedId
                                            ? `drop-shadow(0 0 3px ${ARTISTIC_MAP_STYLES.colors.icon.selectedHalo}) drop-shadow(0 0 5px ${ARTISTIC_MAP_STYLES.colors.icon.selectedHalo})`
                                            : l.id === preparingId
                                                ? `drop-shadow(0 0 3px ${ARTISTIC_MAP_STYLES.colors.icon.nextHalo})`
                                                : `drop-shadow(0 0 2px ${ARTISTIC_MAP_STYLES.colors.icon.normalHalo}) drop-shadow(0 0 1px ${ARTISTIC_MAP_STYLES.colors.icon.normalHalo})`
                                    }}
                                >
                                    <InlineSVG src={iconUrl} style={{ width: '100%', height: '100%' }} className="stamped-icon" />
                                </div>
                            </React.Fragment>
                        );
                    }

                    // Settlement Rendering
                    return (
                        <React.Fragment key={l.id}>

                            <div style={{
                                position: 'absolute', left: l.finalX ?? 0, top: l.finalY ?? 0, transform: `translate(-50%, -50%) scale(${zoomScale})`,
                                fontFamily: ARTISTIC_MAP_STYLES.fonts.city.family,
                                fontSize: l.tier === 'city' ? ARTISTIC_MAP_STYLES.fonts.city.size : (l.tier === 'town' ? ARTISTIC_MAP_STYLES.fonts.town.size : ARTISTIC_MAP_STYLES.fonts.village.size),
                                fontWeight: l.tier === 'city' ? ARTISTIC_MAP_STYLES.fonts.city.weight : (l.tier === 'town' ? ARTISTIC_MAP_STYLES.fonts.town.weight : ARTISTIC_MAP_STYLES.fonts.village.weight),
                                color: l.isHistorical ? ARTISTIC_MAP_STYLES.colors.text.historical : ARTISTIC_MAP_STYLES.colors.text.active,
                                textShadow: ARTISTIC_MAP_STYLES.colors.shadows.atmosphere,
                                whiteSpace: 'nowrap',
                                pointerEvents: 'none', // Ensure clicks pass through to map/icons
                                zIndex: 5, // Above icons? or below? Usually text labeling an icon should be clear.
                            }}>{l.text}</div>
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
                backgroundSize: 'cover, 20px 20px', mixBlendMode: 'multiply', zIndex: 10, mask: 'url(#paper-mask)', WebkitMask: 'url(#paper-mask)',
                filter: `saturate(${parchmentSaturation})`
            }} />
            <style>{`
                .stamped-icon { display: flex; justify-content: center; align-items: center; }
                .stamped-icon svg { width: 100%; height: 100%; overflow: visible; }
                .stamped-icon path, .stamped-icon circle, .stamped-icon rect, .stamped-icon polygon, .stamped-icon ellipse, .stamped-icon line {
                    fill: currentColor !important;
                    stroke: ${ARTISTIC_MAP_STYLES.colors.icon.stroke} !important;
                    stroke-width: 0.8px !important;
                    stroke-linejoin: round !important;
                    vector-effect: non-scaling-stroke;
                }
            `}</style>
        </div>
    );
};
