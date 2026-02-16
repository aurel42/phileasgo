import { useEffect, useRef, useState } from 'react';
import maplibregl from 'maplibre-gl';
import type { Telemetry } from '../types/telemetry';
import type { POI } from './usePOIs';
import type { LabelDTO } from '../types/mapLabels';
import type { MapFrame } from '../types/artisticMap';
import { MapCameraSystem } from '../systems/MapCameraSystem';
import { MapLabelSystem } from '../systems/MapLabelSystem';
import type { MapContext, SystemState, MapMode } from '../systems/types';

interface UseMapOrchestratorProps {
    mapRef: React.MutableRefObject<maplibregl.Map | null>;
    telemetry: Telemetry | null;
    pois: POI[];
    narratorStatus: any;
    tripEvents: any[];
    validEvents: any[];
    poiDiscoveryTimes: Map<string, number>;
    progress: number;
    effectiveReplayMode: boolean;
    settlementCategories: string[];
    beaconMaxTargets: number;
    styleLoaded: boolean;
    fontsLoaded: boolean;
    accumulatedSettlements: React.MutableRefObject<Map<string, LabelDTO>>;
    accumulatedPois: React.MutableRefObject<Map<string, POI>>;
    initialZoom?: number;
    initialCenter?: [number, number];
}

export const useMapOrchestrator = (props: UseMapOrchestratorProps) => {
    // 1. Initial State
    const [frame, setFrame] = useState<MapFrame>({
        center: props.initialCenter || [0, 0],
        zoom: props.initialZoom || 2,
        offset: [0, 0],
        maskPath: "",
        aircraftX: 0,
        aircraftY: 0,
        agl: 0,
        heading: 0,
        bearingLine: null,
        labels: []
    });

    // 2. Systems Instantiation (Stable across renders)
    const systemsRef = useRef<{ camera: MapCameraSystem; label: MapLabelSystem } | null>(null);
    if (!systemsRef.current) {
        systemsRef.current = {
            camera: new MapCameraSystem(props.mapRef),
            label: new MapLabelSystem(props.mapRef)
        };
    }
    const systems = systemsRef.current;

    // 3. Props Sync (Ref Pattern) to avoid stale closures in the loop
    const propsRef = useRef(props);
    useEffect(() => { propsRef.current = props; }, [props]);

    // 4. Time Tracking
    const lastTickTime = useRef<number>(performance.now());

    // 5. Frame Ref Logic
    const frameRef = useRef(frame);
    useEffect(() => { frameRef.current = frame; }, [frame]);

    // 6. Random Start Location Logic
    const hasFetchedRandomStart = useRef(false);
    const randomLocationRef = useRef<[number, number] | undefined>(undefined);

    useEffect(() => {
        // Only run if map acts as World Map (IDLE state initially) and we haven't fetched yet
        if (hasFetchedRandomStart.current) return;

        const fetchRandomStart = async () => {
            try {
                const response = await fetch('/api/geography/random-start');
                if (!response.ok) throw new Error('Network response was not ok');
                const data = await response.json();
                if (data.lat != null && data.lon != null) {
                    randomLocationRef.current = [data.lon, data.lat];
                    hasFetchedRandomStart.current = true;
                }
            } catch (error) {
                console.warn("Failed to fetch random start location:", error);
                setTimeout(fetchRandomStart, 2000);
            }
        };

        if (!props.telemetry?.SimState || props.telemetry.SimState === 'disconnected') {
            fetchRandomStart();
        } else {
            hasFetchedRandomStart.current = true;
        }

    }, []);

    // 7. The Orchestrator Loop
    useEffect(() => {
        if (!props.styleLoaded) return;

        const tick = () => {
            const now = performance.now();
            const dt = (now - lastTickTime.current) / 1000;
            lastTickTime.current = now;

            const currentProps = propsRef.current;
            const currentFrame = frameRef.current;
            const m = currentProps.mapRef.current;

            if (!m) return; // Map not ready

            // ... Mode calc ...
            const t = currentProps.telemetry;
            let mode: MapMode = 'IDLE';
            if (currentProps.effectiveReplayMode) mode = 'REPLAY';
            else if (t?.SimState === 'active') mode = 'FLIGHT';
            else if (t?.SimState === 'inactive') mode = 'TRANSITION';

            // Replay Helpers
            let totalTripTime = 0;
            if (currentProps.validEvents.length > 0) {
                const start = new Date(currentProps.validEvents[0].timestamp).getTime();
                const end = new Date(currentProps.validEvents[currentProps.validEvents.length - 1].timestamp).getTime();
                totalTripTime = end - start;
            }

            // Context from Ref
            const ctx: MapContext = {
                telemetry: t,
                pois: currentProps.pois,
                zoom: currentFrame.zoom,
                center: currentFrame.center,
                mode,
                progress: currentProps.progress,
                validEvents: currentProps.validEvents,
                poiDiscoveryTimes: currentProps.poiDiscoveryTimes,
                totalTripTime,
                firstEventTime: 0,
                narratorStatus: currentProps.narratorStatus,
                settlementCategories: currentProps.settlementCategories,
                beaconMaxTargets: currentProps.beaconMaxTargets,
                stabilizedReplay: false,
                mapWidth: m.getCanvas().clientWidth,
                mapHeight: m.getCanvas().clientHeight,
                styleLoaded: currentProps.styleLoaded,
                fontsLoaded: currentProps.fontsLoaded,
                randomLocation: randomLocationRef.current
            };

            const nextFrame = { ...currentFrame };
            const state: SystemState = {
                frame: nextFrame,
                accumulatedSettlements: currentProps.accumulatedSettlements.current,
                accumulatedPois: currentProps.accumulatedPois.current
            };

            systems.camera.update(dt, ctx, state);
            systems.label.update(dt, ctx, state);

            setFrame(nextFrame);
        };

        const intervalId = setInterval(tick, 16); // ~60fps
        return () => clearInterval(intervalId);
    }, [props.styleLoaded]);

    return { frame };
};
