import React, { useState, useEffect, useRef, useMemo } from 'react';
import maplibregl from 'maplibre-gl';
import { useNarrator } from '../../hooks/useNarrator';
import { useTripEvents } from '../../hooks/useTripEvents';
import { PlacementEngine } from '../../metrics/PlacementEngine';
import type { LabelCandidate } from '../../metrics/PlacementEngine';
import type { ArtisticMapProps } from '../../types/artisticMap';
import { ARTISTIC_MAP_STYLES } from '../../styles/artisticMapStyles';
import { useArtisticMapReplay } from '../../hooks/useArtisticMapReplay';
import { useArtisticMapHeartbeat } from '../../hooks/useArtisticMapHeartbeat';
import { ArtisticMapOverlays } from './ArtisticMapOverlays';
import { ArtisticMapLabels } from './ArtisticMapLabels';
import { ArtisticMapAircraft } from './ArtisticMapAircraft';
import { adjustFont } from '../../utils/mapUtils';
import { getFontFromClass, measureText } from '../../metrics/text';
import { interpolatePositionFromEvents } from '../../utils/replay';

export const ArtisticMap: React.FC<ArtisticMapProps> = ({
    className, center, zoom, telemetry, pois, settlementCategories,
    paperOpacityFog, paperOpacityClear, parchmentSaturation, selectedPOI, isAutoOpened,
    onPOISelect, onMapClick, beaconMaxTargets = 8, showDebugBoxes = false,
    aircraftIcon = 'balloon', aircraftSize = 32, aircraftColorMain, aircraftColorAccent,
    mapFactory = (opts) => new maplibregl.Map(opts)
}) => {
    const mapContainer = useRef<HTMLDivElement>(null);
    const map = useRef<maplibregl.Map | null>(null);
    const [styleLoaded, setStyleLoaded] = useState(false);
    const engine = useMemo(() => new PlacementEngine(), []);

    const { status: narratorStatus } = useNarrator();
    const { data: tripEvents } = useTripEvents();
    console.log('[ArtisticMap] tripEvents:', tripEvents?.length); // Use it

    const [replayLabels, setReplayLabels] = useState<LabelCandidate[]>([]);
    const labelAppearanceRef = useRef<Map<string, number>>(new Map());

    const {
        effectiveReplayMode, progress, validEvents, pathPoints, poiDiscoveryTimes,
        replayFirstTime, replayTotalTime, firstEventTime
    } = useArtisticMapReplay(telemetry, engine);

    const {
        frame, fontsLoaded, accumulatedPois, accumulatedSettlements
    } = useArtisticMapHeartbeat(
        map, telemetry, pois, zoom, center, effectiveReplayMode, progress,
        validEvents, poiDiscoveryTimes, replayTotalTime, firstEventTime,
        narratorStatus, engine, styleLoaded, setStyleLoaded, settlementCategories, beaconMaxTargets,
        replayLabels.length > 0
    );

    // Replay Static Layout (One-time)
    useEffect(() => {
        if (!effectiveReplayMode || !map.current || !fontsLoaded || validEvents.length === 0) {
            if (!effectiveReplayMode) setReplayLabels([]);
            return;
        }

        const m = map.current;
        const eng = new PlacementEngine();
        const cityFont = adjustFont(getFontFromClass('role-title'), -4);
        const townFont = adjustFont(getFontFromClass('role-header'), -4);
        const villageFont = adjustFont(getFontFromClass('role-text-lg'), -4);
        const markerLabelFont = adjustFont(getFontFromClass('role-label'), 2);

        Array.from(accumulatedSettlements.current.values()).forEach(l => {
            let role = villageFont; let tierName: 'city' | 'town' | 'village' = 'village';
            if (l.category === 'city') { tierName = 'city'; role = cityFont; }
            else if (l.category === 'town') { tierName = 'town'; role = townFont; }
            let text = l.name.split('(')[0].split(',')[0].split('/')[0].trim();
            if (role.uppercase) text = text.toUpperCase();
            const dims = measureText(text, role.font, role.letterSpacing);
            eng.register({ id: l.id, lat: l.lat, lon: l.lon, text, tier: tierName, width: dims.width, height: dims.height, type: 'settlement', score: l.pop || 0, isHistorical: false, size: 'L' });
        });

        validEvents.forEach(e => {
            if (!e.metadata) return;
            const eid = e.metadata.poi_id || e.metadata.qid;
            if (!eid) return;
            const lat = e.metadata.poi_lat ? parseFloat(e.metadata.poi_lat) : e.lat;
            const lon = e.metadata.poi_lon ? parseFloat(e.metadata.poi_lon) : e.lon;
            const icon = e.metadata.icon_artistic || e.metadata.icon || 'attraction';
            const name = e.title || e.metadata.poi_name || 'Point of Interest';
            const score = e.metadata.poi_score ? parseFloat(e.metadata.poi_score) : 30;
            let markerLabel = undefined;
            if (score >= 10) {
                let text = name.split('(')[0].split(',')[0].split('/')[0].trim();
                if (markerLabelFont.uppercase) text = text.toUpperCase();
                const dims = measureText(text, markerLabelFont.font, markerLabelFont.letterSpacing);
                markerLabel = { text, width: dims.width, height: dims.height };
            }
            eng.register({ id: eid, lat, lon, text: "", tier: 'village', score, width: 26, height: 26, type: 'poi', isHistorical: false, size: (e.metadata.poi_size || 'M') as any, icon, markerLabel });
        });

        const targetZoom = frame.zoom;
        const timeout = setTimeout(() => {
            const w = m.getCanvas().clientWidth;
            const h = m.getCanvas().clientHeight;
            const placed = eng.compute((lat, lon) => { const pos = m.project([lon, lat]); return { x: pos.x, y: pos.y }; }, w, h, targetZoom);
            setReplayLabels(placed);
        }, 500);

        return () => clearTimeout(timeout);
    }, [effectiveReplayMode, fontsLoaded, validEvents.length, frame.zoom]);

    // Initialize Map
    useEffect(() => {
        if (map.current || !mapContainer.current) return;
        map.current = mapFactory({
            container: mapContainer.current,
            style: {
                version: 8,
                sources: {
                    'stamen-watercolor-hd': { type: 'raster', tiles: ['https://watercolormaps.collection.cooperhewitt.org/tile/watercolor/{z}/{x}/{y}.jpg'], tileSize: 128 },
                    'hillshade-source': { type: 'raster-dem', tiles: ['https://tiles.stadiamaps.com/data/terrarium/{z}/{x}/{y}.png'], tileSize: 256, encoding: 'terrarium' },
                    'openfreemap': { type: 'vector', url: 'https://tiles.openfreemap.org/planet' }
                },
                layers: [
                    { id: 'background', type: 'background', paint: { 'background-color': '#f4ecd8' } },
                    { id: 'watercolor', type: 'raster', source: 'stamen-watercolor-hd', paint: { 'raster-saturation': -0.2, 'raster-contrast': 0.1 } },
                    { id: 'hillshading', type: 'hillshade', source: 'hillshade-source', maxzoom: 10, paint: { 'hillshade-exaggeration': ['interpolate', ['linear'], ['zoom'], 4, 0.0, 6, 0.45], 'hillshade-shadow-color': 'rgba(0, 0, 0, 0.35)', 'hillshade-accent-color': 'rgba(0, 0, 0, 0.15)' } },
                    { id: 'runways-fill', type: 'fill', source: 'openfreemap', 'source-layer': 'aeroway', minzoom: 8, filter: ['all', ["match", ["geometry-type"], ["MultiPolygon", "Polygon"], true, false], ["==", ["get", "class"], "runway"]], paint: { 'fill-color': ['case', ['match', ['get', 'surface'], ['grass', 'dirt', 'earth', 'ground', 'unpaved'], true, false], '#769b58', '#707070'], 'fill-opacity': 0.8 } },
                    { id: 'runways-line', type: 'line', source: 'openfreemap', 'source-layer': 'aeroway', minzoom: 8, filter: ['all', ["match", ["geometry-type"], ["LineString", "MultiLineString"], true, false], ["==", ["get", "class"], "runway"]], paint: { 'line-color': ['case', ['match', ['get', 'surface'], ['grass', 'dirt', 'earth', 'ground', 'unpaved'], true, false], '#769b58', '#707070'], 'line-width': 6, 'line-opacity': 1.0 } }
                ]
            },
            center: [center[1], center[0]],
            zoom: zoom,
            minZoom: 0,
            maxZoom: 12,
            attributionControl: false,
            interactive: false
        });

        if (!map.current) return;
        map.current.on('load', () => {
            map.current?.addSource('bearing-line', { type: 'geojson', data: { type: 'Feature', properties: {}, geometry: { type: 'LineString', coordinates: [] } } });
            map.current?.addLayer({ id: 'bearing-line-layer', type: 'line', source: 'bearing-line', paint: { 'line-color': '#5c4033', 'line-width': 2, 'line-dasharray': [2, 2], 'line-opacity': 0.7 } });
            setStyleLoaded(true);
        });

        return () => { map.current?.remove(); map.current = null; };
    }, []);

    // Sync Map to Frame
    useEffect(() => {
        const m = map.current;
        if (!m || !styleLoaded) return;
        if (effectiveReplayMode && m.isEasing()) return;
        m.easeTo({ center: frame.center, zoom: frame.zoom, offset: [frame.offset[0], frame.offset[1]], duration: 0 });
    }, [frame.center, frame.zoom, frame.offset, styleLoaded]);

    const replayBalloonPos = useMemo(() => {
        if (!effectiveReplayMode || pathPoints.length < 2 || !map.current) return null;
        const interp = interpolatePositionFromEvents(validEvents, progress);
        const pt = map.current.project([interp.position[1], interp.position[0]]);
        return { x: pt.x, y: pt.y };
    }, [effectiveReplayMode, pathPoints, progress, styleLoaded, frame.zoom, frame.center]);

    const currentNarratedId = (narratorStatus?.playback_status === 'playing' || narratorStatus?.playback_status === 'paused')
        ? narratorStatus?.current_poi?.wikidata_id : undefined;
    const preparingId = narratorStatus?.preparing_poi?.wikidata_id;

    return (
        <div className={className} onClick={() => onMapClick?.()} style={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden' }}>
            <div ref={mapContainer} style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', color: 'black' }} />

            <ArtisticMapOverlays
                zoom={frame.zoom}
                center={frame.center}
                telemetry={telemetry}
                effectiveReplayMode={effectiveReplayMode}
                paperOpacityFog={paperOpacityFog}
                paperOpacityClear={paperOpacityClear}
                parchmentSaturation={parchmentSaturation}
                maskPath={frame.maskPath}
            />

            <ArtisticMapLabels
                map={map}
                labels={effectiveReplayMode ? replayLabels : frame.labels}
                accumulatedPois={accumulatedPois}
                labelAppearanceRef={labelAppearanceRef}
                selectedPOI={selectedPOI}
                currentNarratedId={currentNarratedId}
                preparingId={preparingId}
                isAutoOpened={!!isAutoOpened}
                effectiveReplayMode={effectiveReplayMode}
                progress={progress}
                poiDiscoveryTimes={poiDiscoveryTimes}
                replayFirstTime={replayFirstTime}
                replayTotalTime={replayTotalTime}
                currentZoom={frame.zoom}
                onPOISelect={onPOISelect}
            />

            <ArtisticMapAircraft
                map={map}
                effectiveReplayMode={effectiveReplayMode}
                pathPoints={pathPoints}
                validEvents={validEvents}
                progress={progress}
                departure={null}
                destination={null}
                replayBalloonPos={replayBalloonPos}
                frame={frame}
                aircraftIcon={aircraftIcon}
                aircraftSize={aircraftSize}
                aircraftColorMain={aircraftColorMain || '#ffffff'}
                aircraftColorAccent={aircraftColorAccent || '#000000'}
            />

            {showDebugBoxes && (
                <svg style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 25 }}>
                    {engine.getDebugBoxes().map((box, i) => (
                        <rect key={`dbg-${box.ownerId}-${i}`} x={box.minX} y={box.minY} width={box.maxX - box.minX} height={box.maxY - box.minY} fill="none" stroke={box.type === 'marker' ? 'rgba(255,60,60,0.7)' : 'rgba(60,120,255,0.7)'} strokeWidth={1} />
                    ))}
                </svg>
            )}

            <style>{`.stamped-icon { display: flex; justify-content: center; align-items: center; } .stamped-icon svg { width: 100%; height: 100%; overflow: visible; } .stamped-icon path, .stamped-icon circle, .stamped-icon rect, .stamped-icon polygon, .stamped-icon ellipse, .stamped-icon line { fill: currentColor!important; stroke: var(--stamped-stroke, ${ARTISTIC_MAP_STYLES.colors.icon.stroke}) !important; stroke-width: var(--stamped-width, 0.8px) !important; stroke-linejoin: round!important; vector-effect: non-scaling-stroke; }`}</style>
        </div>
    );
};
