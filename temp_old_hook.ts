import { useState, useEffect, useRef } from 'react';
import maplibregl from 'maplibre-gl';
import * as turf from '@turf/turf';
import type { Feature } from 'geojson';
import type { Telemetry } from '../types/telemetry';
import type { POI } from '../hooks/usePOIs';
import type { LabelDTO } from '../types/mapLabels';
import type { MapFrame } from '../types/artisticMap';
import { PlacementEngine } from '../metrics/PlacementEngine';
import type { LabelCandidate } from '../metrics/PlacementEngine';
import { labelService } from '../services/labelService';
import { interpolatePositionFromEvents } from '../utils/replay';
import { adjustFont } from '../utils/mapUtils';
import { measureText, getFontFromClass } from '../metrics/text';
import { rayToEdge, maskToPath } from '../utils/mapGeometry';

export const useArtisticMapHeartbeat = (
    map: React.MutableRefObject<maplibregl.Map | null>,
    telemetry: Telemetry | null,
    pois: POI[],
    zoom: number,
    center: [number, number],
    effectiveReplayMode: boolean,
    progress: number,
    validEvents: any[],
    poiDiscoveryTimes: Map<string, number>,
    totalTripTime: number,
    firstEventTime: number,
    narratorStatus: any,
    engine: PlacementEngine,
    styleLoaded: boolean,
    setStyleLoaded: (loaded: boolean) => void,
    settlementCategories: string[],
    beaconMaxTargets: number,
    stabilizedReplay: boolean
) => {
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

    const [fontsLoaded, setFontsLoaded] = useState(false);
    useEffect(() => {
        if (document.fonts) {
            document.fonts.ready.then(() => setFontsLoaded(true));
        } else {
            setFontsLoaded(true);
        }
    }, []);

    const accumulatedSettlements = useRef<Map<string, LabelDTO>>(new Map());
    const accumulatedPois = useRef<Map<string, POI>>(new Map());
    const failedPoiLabelIds = useRef<Set<string>>(new Set());
    const labeledPoiIds = useRef<Set<string>>(new Set());
    const zoomReady = useRef(false);
    const [lastSyncLabels, setLastSyncLabels] = useState<LabelDTO[]>([]);

    const lastPoiCount = useRef(0);
    const lastLabelsJson = useRef("");
    const lastPlacementView = useRef<{ lng: number, lat: number, zoom: number } | null>(null);

    const prevSimStateRef = useRef<string | undefined>(undefined);
    const lastNarratedId = useRef<string | undefined>(undefined);
    const lastPreparingId = useRef<string | undefined>(undefined);

    // Refs for stable heartbeat closure
    const telemetryRef = useRef(telemetry);
    const poisRef = useRef(pois);
    const zoomRef = useRef(zoom);
    const lastSyncLabelsRef = useRef(lastSyncLabels);
    const progressRef = useRef(progress);
    const validEventsRef = useRef(validEvents);
    const poiDiscoveryTimesRef = useRef(poiDiscoveryTimes);
    const totalTripTimeRef = useRef(totalTripTime);
    const firstEventTimeRef = useRef(firstEventTime);

    const currentNarratedId = (narratorStatus?.playback_status === 'playing' || narratorStatus?.playback_status === 'paused')
        ? narratorStatus?.current_poi?.wikidata_id : undefined;
    const preparingId = narratorStatus?.preparing_poi?.wikidata_id;

    const currentNarratedIdRef = useRef<string | undefined>(currentNarratedId);
    const preparingIdRef = useRef<string | undefined>(preparingId);
    const currentPoiRef = useRef<POI | undefined>(undefined);
    const preparingPoiRef = useRef<POI | undefined>(undefined);

    useEffect(() => { telemetryRef.current = telemetry; }, [telemetry]);
    useEffect(() => { poisRef.current = pois; }, [pois]);
    useEffect(() => { zoomRef.current = zoom; }, [zoom]);
    useEffect(() => { lastSyncLabelsRef.current = lastSyncLabels; }, [lastSyncLabels]);
    useEffect(() => { progressRef.current = progress; }, [progress]);
    useEffect(() => { validEventsRef.current = validEvents; }, [validEvents]);
    useEffect(() => { poiDiscoveryTimesRef.current = poiDiscoveryTimes; }, [poiDiscoveryTimes]);
    useEffect(() => { totalTripTimeRef.current = totalTripTime; }, [totalTripTime]);
    useEffect(() => { firstEventTimeRef.current = firstEventTime; }, [firstEventTime]);
    useEffect(() => { currentNarratedIdRef.current = currentNarratedId; }, [currentNarratedId]);
    useEffect(() => { preparingIdRef.current = preparingId; }, [preparingId]);
    useEffect(() => { currentPoiRef.current = narratorStatus?.current_poi; }, [narratorStatus?.current_poi]);
    useEffect(() => { preparingPoiRef.current = narratorStatus?.preparing_poi; }, [narratorStatus?.preparing_poi]);

    useEffect(() => {
        if (!styleLoaded || !map.current) return;

        let isRunning = false;
        let lastMaskData: any = null;
        let prevZoomInt = -1;
        let firstTick = true;
        let lockedCenter: [number, number] | null = null;
        let lockedOffset: [number, number] = [0, 0];
        let lockedZoom = -1;
        let lastLabels: LabelCandidate[] = [];
        let lastMask: string = '';

        const tick = async () => {
            if (isRunning) return;
            isRunning = true;

            // Stability Bypass: If we are in replay mode and already have our static layout stabilized,
            // we stop the heartbeat to preserve CPU and ensure absolute position freezing.
            if (effectiveReplayMode && stabilizedReplay) {
                isRunning = false;
                return;
            }

            try {
                if (document.fonts) await document.fonts.ready;
                const m = map.current;
                const t = telemetryRef.current;
                const currentValidEvents = validEventsRef.current;

                if (!m || !t) { isRunning = false; return; }

                const acState = effectiveReplayMode && currentValidEvents.length >= 2
                    ? (() => {
                        const interp = interpolatePositionFromEvents(currentValidEvents, progressRef.current);
                        return { lat: interp.position[0], lon: interp.position[1], heading: interp.heading };
                    })()
                    : { lat: t.Latitude, lon: t.Longitude, heading: t.Heading };

                const bounds = m.getBounds();
                const targetZoomBase = zoomRef.current;
                if (!bounds) { isRunning = false; return; }

                const currentSimState = t.SimState;
                const prevSimState = prevSimStateRef.current;
                const stateTransition = prevSimState !== currentSimState;
                prevSimStateRef.current = currentSimState;

                const fetchMask = async () => {
                    try {
                        const maskRes = await fetch(`/api/map/visibility-mask?bounds=${bounds.getNorth()},${bounds.getEast()},${bounds.getSouth()},${bounds.getWest()}&resolution=20`);
                        if (maskRes.ok) lastMaskData = await maskRes.json();
                    } catch (e) {
                        console.error("Background Mask Fetch Failed:", e);
                    }
                };

                if (t.SimState === 'active') {
                    if (firstTick) await fetchMask();
                    else fetchMask();
                }

                const mapWidth = m.getCanvas().clientWidth;
                const mapHeight = m.getCanvas().clientHeight;

                let needsRecenter = !lockedCenter || stateTransition;
                if (lockedCenter && !needsRecenter) {
                    const currentPos: [number, number] = [acState.lon, acState.lat];
                    const aircraftOnMap = m.project(currentPos);
                    const hdgRad = acState.heading * (Math.PI / 180);
                    const adx = Math.sin(hdgRad);
                    const ady = -Math.cos(hdgRad);

                    const distAhead = rayToEdge(aircraftOnMap.x, aircraftOnMap.y, adx, ady, mapWidth, mapHeight);
                    const distBehind = rayToEdge(aircraftOnMap.x, aircraftOnMap.y, -adx, -ady, mapWidth, mapHeight);
                    needsRecenter = distBehind > distAhead;

                    const playingPoi = currentPoiRef.current;
                    if (playingPoi && !bounds.contains([playingPoi.lon, playingPoi.lat])) {
                        needsRecenter = true;
                    }
                }

                if (needsRecenter) {
                    const isIdle = t.SimState === 'disconnected' && !effectiveReplayMode;
                    const offsetPx = isIdle ? 0 : Math.min(mapWidth, mapHeight) * 0.35;
                    const hdgRad = acState.heading * (Math.PI / 180);
                    const dx = offsetPx * Math.sin(hdgRad);
                    const dy = -offsetPx * Math.cos(hdgRad);

                    lockedCenter = isIdle ? [0, 0] : [acState.lon, acState.lat];
                    lockedOffset = isIdle ? [0, 0] : [-dx, -dy];

                    let newZoom = (lockedZoom === -1) ? targetZoomBase : lockedZoom;
                    if (isIdle || (stateTransition && prevSimState === 'disconnected' && currentSimState === 'inactive')) {
                        const longerDim = Math.max(mapWidth, mapHeight);
                        newZoom = Math.log2(longerDim / 256);
                        newZoom = Math.max(newZoom, m.getMinZoom());
                    } else if (stateTransition && currentSimState === 'inactive' && prevSimState === 'active') {
                        const features: any[] = [];
                        if (lastMaskData?.geometry) features.push({ type: 'Feature', geometry: lastMaskData.geometry, properties: {} });
                        const playingPoi = currentPoiRef.current;
                        const preparingPoi = preparingPoiRef.current;
                        if (playingPoi) features.push(turf.point([playingPoi.lon, playingPoi.lat]));
                        if (preparingPoi) features.push(turf.point([preparingPoi.lon, preparingPoi.lat]));

                        if (features.length > 0) {
                            const collection = turf.featureCollection(features);
                            const baseBbox = turf.bbox(collection);
                            const centerLng = (baseBbox[0] + baseBbox[2]) / 2;
                            const centerLat = (baseBbox[1] + baseBbox[3]) / 2;
                            const halfWidth = (baseBbox[2] - baseBbox[0]);
                            const halfHeight = (baseBbox[3] - baseBbox[1]);
                            const doubledBbox = [centerLng - halfWidth, centerLat - halfHeight, centerLng + halfWidth, centerLat + halfHeight];
                            const camera = m.cameraForBounds(doubledBbox as [number, number, number, number], { padding: 40, maxZoom: 12 });
                            if (camera?.zoom !== undefined && !isNaN(camera.zoom)) newZoom = Math.min(Math.max(camera.zoom, m.getMinZoom()), 12);
                        } else {
                            newZoom = Math.max(newZoom - 1, m.getMinZoom());
                        }
                    } else {
                        const features: any[] = [];
                        if (lastMaskData?.geometry) features.push({ type: 'Feature', geometry: lastMaskData.geometry, properties: {} });
                        const playingPoi = currentPoiRef.current;
                        const preparingPoi = preparingPoiRef.current;
                        if (playingPoi) features.push(turf.point([playingPoi.lon, playingPoi.lat]));
                        if (preparingPoi) features.push(turf.point([preparingPoi.lon, preparingPoi.lat]));
                        if (features.length > 0) {
                            const collection = turf.featureCollection(features);
                            const combinedBbox = turf.bbox(collection);
                            const camera = m.cameraForBounds(combinedBbox as [number, number, number, number], { padding: 40, maxZoom: 12 });
                            if (camera?.zoom !== undefined && !isNaN(camera.zoom)) newZoom = Math.min(Math.max(camera.zoom, m.getMinZoom()), 12);
                        }
                    }
                    lockedZoom = Math.round(newZoom * 2) / 2;
                }

                if (effectiveReplayMode) {
                    if (m.isEasing()) { isRunning = false; return; }
                    const c = m.getCenter();
                    lockedCenter = [c.lng, c.lat];
                    lockedZoom = m.getZoom();
                    lockedOffset = [0, 0];
                } else if (needsRecenter) {
                    m.easeTo({ center: lockedCenter as maplibregl.LngLatLike, zoom: lockedZoom, offset: lockedOffset, duration: 0 });
                }

                const b = m.getBounds();
                labelService.fetchLabels({
                    bbox: [b.getSouth(), b.getWest(), b.getNorth(), b.getEast()],
                    ac_lat: acState.lat, ac_lon: acState.lon, heading: acState.heading, zoom: lockedZoom
                }).then(newLabels => {
                    newLabels.forEach(l => accumulatedSettlements.current.set(l.id, l));
                    setLastSyncLabels(newLabels);
                }).catch(e => console.error("Label Sync Failed:", e));

                if (firstTick) { firstTick = false; zoomReady.current = true; }

                const targetZoom = lockedZoom;
                const currentZoomSnap = Math.round(targetZoom * 2) / 2;
                if (prevZoomInt === -1) prevZoomInt = currentZoomSnap;

                const pruneOffscreen = () => {
                    const w = mapWidth; const h = mapHeight;
                    const topLeft = m.unproject([-w / 2, -h / 2]);
                    const bottomRight = m.unproject([w + w / 2, h + h / 2]);
                    const expandedBounds = new maplibregl.LngLatBounds([topLeft.lng, bottomRight.lat], [bottomRight.lng, topLeft.lat]);
                    const checkPrune = (map: Map<string, any>) => {
                        for (const [id, item] of map.entries()) {
                            if (!expandedBounds.contains([item.lon, item.lat])) {
                                map.delete(id);
                                engine.forget(id);
                            }
                        }
                    };
                    checkPrune(accumulatedSettlements.current);
                    checkPrune(accumulatedPois.current);
                    failedPoiLabelIds.current.clear();
                    labeledPoiIds.current.clear();
                };

                if (currentZoomSnap !== prevZoomInt) {
                    if (!effectiveReplayMode) pruneOffscreen();
                    prevZoomInt = currentZoomSnap;
                }

                const aircraftPos = m.project([acState.lon, acState.lat]);
                const aircraftX = Math.round(aircraftPos.x);
                const aircraftY = Math.round(aircraftPos.y);

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
                if (m.getLayer('bearing-line-layer')) {
                    m.setPaintProperty('bearing-line-layer', 'line-opacity', effectiveReplayMode ? 0 : 0.7);
                }

                const currentSyncLabels = lastSyncLabelsRef.current;
                const currentPois = poisRef.current;
                const labelsJson = JSON.stringify(currentSyncLabels.map(l => l.id));
                const dataChanged = currentPois.length !== lastPoiCount.current || labelsJson !== lastLabelsJson.current;
                const viewChanged = !lastPlacementView.current || !lockedCenter ||
                    Math.abs(lastPlacementView.current.zoom - lockedZoom) > 0.05 ||
                    Math.abs(lastPlacementView.current.lng - lockedCenter![0]) > 0.0001 ||
                    Math.abs(lastPlacementView.current.lat - lockedCenter![1]) > 0.0001;

                let labels = lastLabels;
                const statusChanged = currentNarratedIdRef.current !== lastNarratedId.current || preparingIdRef.current !== lastPreparingId.current;

                const isSnap = needsRecenter || viewChanged || firstTick || statusChanged;
                if (isSnap || dataChanged) {
                    if (isSnap) {
                        if (stateTransition) engine.resetCache();
                        engine.clear();
                    }
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
                        engine.register({ id: l.id, lat: l.lat, lon: l.lon, text, tier: tierName, width: dims.width, height: dims.height, type: 'settlement', score: l.pop || 0, isHistorical: false, size: 'L' });
                    });

                    const settlementCatSet = new Set(settlementCategories.map(c => c.toLowerCase()));
                    let champion: POI | null = null;
                    currentPois.forEach(p => {
                        if (settlementCatSet.has(p.category?.toLowerCase())) return;
                        const normalizedName = p.name_en.split('(')[0].split(',')[0].split('/')[0].trim();
                        if (normalizedName.length > 24) return;
                        const isHistorical = !!(p.last_played && p.last_played !== "0001-01-01T00:00:00Z");
                        if (isHistorical) return;
                        if (p.score >= 10 && !failedPoiLabelIds.current.has(p.wikidata_id) && !labeledPoiIds.current.has(p.wikidata_id)) {
                            if (!champion || (p.score > champion.score)) champion = p;
                        }
                    });

                    if (effectiveReplayMode) {
                        const simulatedElapsed = progressRef.current * totalTripTimeRef.current;
                        const processedIds = new Set<string>();
                        currentValidEvents.forEach(e => {
                            const eid = e.metadata?.poi_id || e.metadata?.qid;
                            if (!eid || processedIds.has(eid)) return;
                            processedIds.add(eid);
                            const discoveryTime = poiDiscoveryTimesRef.current.get(eid);
                            if (discoveryTime != null && (discoveryTime - firstEventTimeRef.current) > simulatedElapsed) return;
                            const lat = e.metadata.poi_lat ? parseFloat(e.metadata.poi_lat) : e.lat;
                            const lon = e.metadata.poi_lon ? parseFloat(e.metadata.poi_lon) : e.lon;
                            if (!lat || !lon) return;
                            const name = e.title || e.metadata.poi_name || 'Point of Interest';
                            const score = e.metadata.poi_score ? parseFloat(e.metadata.poi_score) : 30;
                            let markerLabel = undefined;
                            if (score >= 10) {
                                let text = name.split('(')[0].split(',')[0].split('/')[0].trim();
                                if (markerLabelFont.uppercase) text = text.toUpperCase();
                                const dims = measureText(text, markerLabelFont.font, markerLabelFont.letterSpacing);
                                markerLabel = { text, width: dims.width, height: dims.height };
                            }
                            engine.register({ id: eid, lat, lon, text: "", tier: 'village', score, width: 26, height: 26, type: 'poi', isHistorical: false, size: (e.metadata.poi_size || 'M') as any, icon: e.metadata.icon_artistic || e.metadata.icon || 'attraction', markerLabel });
                        });
                    } else {
                        currentPois.forEach(p => { if (p.lat && p.lon) accumulatedPois.current.set(p.wikidata_id, p); });
                        Array.from(accumulatedPois.current.values()).forEach(p => {
                            const isChampion = champion && p.wikidata_id === champion.wikidata_id;
                            const needsMarkerLabel = isChampion || labeledPoiIds.current.has(p.wikidata_id);
                            const isHistorical = !!(p.last_played && p.last_played !== "0001-01-01T00:00:00Z");
                            let markerLabel = undefined;
                            if (needsMarkerLabel) {
                                let text = p.name_en.split('(')[0].split(',')[0].split('/')[0].trim();
                                if (markerLabelFont.uppercase) text = text.toUpperCase();
                                const dims = measureText(text, markerLabelFont.font, markerLabelFont.letterSpacing);
                                markerLabel = { text, width: dims.width, height: dims.height };
                            }
                            engine.register({ id: p.wikidata_id, lat: p.lat, lon: p.lon, text: "", tier: 'village', score: p.score || 0, width: 26, height: 26, type: 'poi', isHistorical, size: p.size as any, icon: p.icon, visibility: p.visibility, markerLabel });
                        });
                    }

                    labels = engine.compute((lat, lon) => { const pos = m.project([lon, lat]); return { x: pos.x, y: pos.y }; }, mapWidth, mapHeight, lockedZoom);
                    if (champion) {
                        const placedChamp = labels.find(l => l.id === (champion as POI).wikidata_id);
                        if (placedChamp?.markerLabel) {
                            if (placedChamp.markerLabelPos) labeledPoiIds.current.add((champion as POI).wikidata_id);
                            else failedPoiLabelIds.current.add((champion as POI).wikidata_id);
                        }
                    }
                    lastPoiCount.current = currentPois.length;
                    lastLabelsJson.current = labelsJson;
                    lastPlacementView.current = { lng: lockedCenter![0], lat: lockedCenter![1], zoom: lockedZoom };
                    lastNarratedId.current = currentNarratedIdRef.current;
                    lastPreparingId.current = preparingIdRef.current;
                    lastLabels = labels;
                }

                if (lastMaskData) lastMask = maskToPath(lastMaskData, m);

                setFrame(prev => {
                    const beaconSource = effectiveReplayMode ? [] : Array.from(accumulatedPois.current.values());
                    if (isSnap && beaconSource.length > 0) {
                        beaconSource.forEach(p => p.has_balloon = false);
                        const priorityIds = new Set<string>();
                        if (currentNarratedIdRef.current) priorityIds.add(currentNarratedIdRef.current);
                        if (preparingIdRef.current) priorityIds.add(preparingIdRef.current);
                        priorityIds.forEach(id => { const p = accumulatedPois.current.get(id); if (p && p.beacon_color) p.has_balloon = true; });
                        const remainingSlots = beaconMaxTargets - priorityIds.size;
                        if (remainingSlots > 0) {
                            beaconSource.filter(p => p.beacon_color && !priorityIds.has(p.wikidata_id)).sort((a, b) => new Date(b.last_played).getTime() - new Date(a.last_played).getTime()).slice(0, remainingSlots).forEach(p => p.has_balloon = true);
                        }
                    }
                    return { ...prev, maskPath: lastMask, center: lockedCenter!, zoom: targetZoom, offset: lockedOffset, heading: acState.heading, bearingLine: bLine, aircraftX, aircraftY, agl: t.AltitudeAGL, labels: lastLabels };
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
    }, [styleLoaded, settlementCategories, beaconMaxTargets, effectiveReplayMode]);

    return { frame, setFrame, fontsLoaded, styleLoaded, setStyleLoaded, accumulatedSettlements, accumulatedPois };
};
