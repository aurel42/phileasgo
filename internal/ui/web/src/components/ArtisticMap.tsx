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
    paperOpacityFog: number;
    paperOpacityClear: number;
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
    paperOpacityFog = 0.7,
    paperOpacityClear = 0.1
}) => {
    const mapContainer = useRef<HTMLDivElement>(null);
    const map = useRef<maplibregl.Map | null>(null);
    const [styleLoaded, setStyleLoaded] = useState(false);

    // -- Placement Engine (Persistent across ticks) --
    const engine = useMemo(() => new PlacementEngine(), []);

    // -- Data Refs for Heartbeat --
    const telemetryRef = useRef(telemetry);
    const poisRef = useRef(pois);
    const zoomRef = useRef(zoom);
    const limitRef = useRef(settlementLabelLimit);

    useEffect(() => { telemetryRef.current = telemetry; }, [telemetry]);
    useEffect(() => { poisRef.current = pois; }, [pois]);
    useEffect(() => { zoomRef.current = zoom; }, [zoom]);
    useEffect(() => { limitRef.current = settlementLabelLimit; }, [settlementLabelLimit]);

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
            minZoom: 8,
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
    }, [frame.center, frame.zoom, frame.offset]);

    // --- THE HEARTBEAT (Strict 0.5Hz / 2000ms) ---
    useEffect(() => {
        if (!styleLoaded || !map.current) return;

        let isRunning = false;
        let lastMaskData: any = null;
        let lastSettlements: POI[] = [];
        let lastTierIndex: number = -1;
        let prevZoomInt = -1;

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
                fetchExtras();

                // 3. ZOOM ADAPTATION & PERSISTENCE
                const currentZoomInt = Math.floor(z);
                if (prevZoomInt === -1) prevZoomInt = currentZoomInt;

                if (currentZoomInt !== prevZoomInt) {
                    accumulatedSettlements.current.clear();
                    engine.resetCache();
                    prevZoomInt = currentZoomInt;
                }

                let computedTargetZoom = targetZoomBase;
                if (lastMaskData?.geometry) {
                    const bbox = turf.bbox(lastMaskData.geometry);
                    if (bbox && !bbox.some(isNaN)) {
                        const camera = m.cameraForBounds(bbox as [number, number, number, number], { padding: 50, maxZoom: 13 });
                        if (camera?.zoom !== undefined && !isNaN(camera.zoom)) {
                            computedTargetZoom = Math.min(Math.max(camera.zoom, 8), 13);
                        }
                    }
                }

                let newSettlements = Array.isArray(lastSettlements) ? (limitRef.current !== -1 ? lastSettlements.slice(0, limitRef.current) : [...lastSettlements]) : [];
                newSettlements.forEach(s => {
                    const id = s.wikidata_id || `${s.lat}-${s.lon}`;
                    accumulatedSettlements.current.set(id, s);
                });

                // 5. COMPUTE GHOST TRANSFORM (View we are about to enter)
                const mapWidth = m.getCanvas().clientWidth;
                const mapHeight = m.getCanvas().clientHeight;
                const offsetPx = Math.min(mapWidth, mapHeight) * 0.25;
                const hdgRad = t.Heading * (Math.PI / 180);
                const dx = offsetPx * Math.sin(hdgRad);
                const dy = -offsetPx * Math.cos(hdgRad);

                // Target Screen Position for the aircraft (always centered + negative offset)
                const aircraftX = (mapWidth / 2) - dx;
                const aircraftY = (mapHeight / 2) - dy;

                const targetCenter: [number, number] = [t.Longitude, t.Latitude];
                const targetZoom = computedTargetZoom;
                const targetOffset: [number, number] = [-dx, -dy];

                // 6. COMPUTE LAYOUT with Ghost Projection
                // This projects LngLat relative to the FUTURE view center.
                const projector = (lat: number, lon: number) => {
                    // Use m.project with the FUTURE zoom. 
                    // To handle the offset, we project relative to the aircraft coordinate.
                    const pPoint = m.project([lon, lat]);
                    const aPoint = m.project(targetCenter);
                    return {
                        x: aircraftX + (pPoint.x - aPoint.x),
                        y: aircraftY + (pPoint.y - aPoint.y)
                    };
                };

                engine.clear();
                const vw = container?.clientWidth || window.innerWidth;
                const vh = container?.clientHeight || window.innerHeight;

                // Register Settlements
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

                // Register POIs
                currentPois.forEach(p => {
                    if (!p.lat || !p.lon) return;
                    let sizePx = 20;
                    if (p.size === 'S') sizePx = 16;
                    else if (p.size === 'L') sizePx = 24;
                    else if (p.size === 'XL') sizePx = 28;
                    const isHistorical = !!(p.last_played && p.last_played !== "0001-01-01T00:00:00Z");
                    engine.register({
                        id: p.wikidata_id, lat: p.lat, lon: p.lon, text: "", tier: 'village', score: p.score || 0,
                        width: sizePx, height: sizePx, type: 'poi', isHistorical, size: p.size as any, icon: p.icon || 'attraction'
                    });
                });

                const snapshotLabels = engine.compute(projector, vw, vh);

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
                    center: targetCenter,
                    zoom: targetZoom,
                    offset: targetOffset,
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
                    const p = map.current?.project([l.lon, l.lat]);
                    if (!p) return null;
                    const px = p.x; const py = p.y;
                    const dx = (l.finalX || 0) - px; const dy = (l.finalY || 0) - py;
                    const dist = Math.ceil(Math.sqrt(dx * dx + dy * dy));
                    const isDisplaced = dist > 30;

                    if (l.type === 'poi') {
                        // ... POI Icon Rendering (Already present) ...
                        const iconUrl = `/icons/${l.icon || 'attraction'}.svg`;
                        let iconColor = ARTISTIC_MAP_STYLES.colors.icon.bronze;
                        if (l.score > 0.8) iconColor = ARTISTIC_MAP_STYLES.colors.icon.gold;
                        else if (l.score > 0.4) iconColor = ARTISTIC_MAP_STYLES.colors.icon.silver;
                        const visibility = 0.5 + (l.score * 0.5);
                        const finalOpacity = l.isHistorical ? visibility * 0.4 : visibility;

                        return (
                            <React.Fragment key={l.id}>
                                {isDisplaced && (
                                    <svg style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 15 }}>
                                        <path d={`M ${l.trueX || 0},${l.trueY || 0} C ${l.trueX || 0},${(l.trueY || 0) - ((l.trueY || 0) - (l.finalY || 0)) / 2} ${(l.finalX || 0)},${(l.finalY || 0) + ((l.trueY || 0) - (l.finalY || 0)) / 2} ${(l.finalX || 0)},${l.finalY}`}
                                            fill="none" stroke={ARTISTIC_MAP_STYLES.tethers.stroke} strokeWidth={ARTISTIC_MAP_STYLES.tethers.width} opacity={ARTISTIC_MAP_STYLES.tethers.opacity} />
                                        <circle cx={l.trueX || 0} cy={l.trueY || 0} r={ARTISTIC_MAP_STYLES.tethers.dotRadius} fill={ARTISTIC_MAP_STYLES.tethers.stroke} opacity={ARTISTIC_MAP_STYLES.tethers.dotOpacity} />
                                    </svg>
                                )}
                                <div style={{
                                    position: 'absolute', left: l.finalX ?? 0, top: l.finalY ?? 0, width: l.width, height: l.height,
                                    transform: `translate(-50%, -50%)`, opacity: finalOpacity,
                                    filter: 'url(#ink-bleed)',
                                    color: iconColor // Apply color here to ensure inheritance
                                }}>
                                    <InlineSVG src={iconUrl} style={{ width: '100%', height: '100%' }} className="stamped-icon" />
                                </div>
                            </React.Fragment>
                        );
                    }

                    // Settlement Rendering
                    return (
                        <React.Fragment key={l.id}>

                            <div style={{
                                position: 'absolute', left: l.finalX ?? 0, top: l.finalY ?? 0, transform: `translate(-50%, -50%) rotate(${l.rotation}deg)`,
                                fontFamily: ARTISTIC_MAP_STYLES.fonts.city.family, fontSize: l.tier === 'city' ? ARTISTIC_MAP_STYLES.fonts.city.size : (l.tier === 'town' ? ARTISTIC_MAP_STYLES.fonts.town.size : ARTISTIC_MAP_STYLES.fonts.village.size),
                                color: l.isHistorical ? ARTISTIC_MAP_STYLES.colors.text.historical : ARTISTIC_MAP_STYLES.colors.text.active, textShadow: ARTISTIC_MAP_STYLES.colors.shadows.atmosphere, whiteSpace: 'nowrap',
                                filter: 'url(#ink-bleed)'
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
                    <filter id="ink-bleed">
                        <feTurbulence type="fractalNoise" baseFrequency="0.04" numOctaves="3" result="noise" />
                        <feDisplacementMap in="SourceGraphic" in2="noise" scale="3.5" xChannelSelector="R" yChannelSelector="G" />
                    </filter>
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
                backgroundSize: 'cover, 20px 20px', mixBlendMode: 'multiply', zIndex: 10, mask: 'url(#paper-mask)', WebkitMask: 'url(#paper-mask)'
            }} />
            <style>{`
                .stamped-icon svg { width: 100%; height: 100%; overflow: visible; } 
                .stamped-icon path, .stamped-icon circle, .stamped-icon rect, .stamped-icon polygon, .stamped-icon ellipse, .stamped-icon line { 
                    fill: currentColor !important; 
                    stroke: ${ARTISTIC_MAP_STYLES.colors.icon.stroke} !important; 
                    stroke-width: 2.5px !important;
                    stroke-linejoin: round !important;
                    vector-effect: non-scaling-stroke;
                }
            `}</style>
        </div>
    );
};
