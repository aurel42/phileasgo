import { useState, useEffect, useRef, useMemo } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useTripEvents } from '../hooks/useTripEvents';
import { useNarrator } from '../hooks/useNarrator';
import { getSignificantTripEvents } from '../utils/replay';
import { PlacementEngine } from '../metrics/PlacementEngine';

export const useArtisticMapReplay = (
    telemetry: any,
    engine: PlacementEngine
) => {
    const queryClient = useQueryClient();
    const { status: narratorStatus } = useNarrator();
    const { data: tripEvents } = useTripEvents();

    const isDisconnected = telemetry?.SimState === 'disconnected';
    const isDebriefing = narratorStatus?.current_type === 'debriefing';
    const isIdleReplay = isDisconnected && tripEvents && tripEvents.length > 1;
    const isReplayMode = isIdleReplay || isDebriefing;

    const [stickyReplay, setStickyReplay] = useState(false);
    useEffect(() => {
        if (isReplayMode) setStickyReplay(true);
        if (telemetry?.SimState === 'active') setStickyReplay(false);
    }, [isReplayMode, telemetry?.SimState]);

    const effectiveReplayMode = isReplayMode || stickyReplay;
    const prevReplayModeRef = useRef(false);

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

    const [progress, setProgress] = useState(0);
    const startTimeRef = useRef<number | null>(null);
    const animationRef = useRef<number | null>(null);

    useEffect(() => {
        if (effectiveReplayMode && !prevReplayModeRef.current) {
            queryClient.invalidateQueries({ queryKey: ['tripEvents'] });
            engine.resetCache();
        } else if (!effectiveReplayMode && prevReplayModeRef.current) {
            engine.resetCache();
        }
        prevReplayModeRef.current = effectiveReplayMode;
    }, [effectiveReplayMode, queryClient, engine]);

    const validEvents = useMemo(() => {
        const sorted = [...(tripEvents || [])].sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime());
        return getSignificantTripEvents(sorted);
    }, [tripEvents]);

    const pathPoints = useMemo((): [number, number][] => {
        return validEvents.map(e => [e.lat, e.lon] as [number, number]);
    }, [validEvents]);

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

    const { replayFirstTime, replayTotalTime } = useMemo(() => {
        if (validEvents.length < 2) return { replayFirstTime: firstEventTime, replayTotalTime: totalTripTime };
        const start = new Date(validEvents[0].timestamp).getTime();
        const end = new Date(validEvents[validEvents.length - 1].timestamp).getTime();
        return { replayFirstTime: start, replayTotalTime: end - start };
    }, [validEvents, firstEventTime, totalTripTime]);

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

    return {
        effectiveReplayMode,
        progress,
        validEvents,
        pathPoints,
        poiDiscoveryTimes,
        replayFirstTime,
        replayTotalTime,
        firstEventTime,
        totalTripTime
    };
};
