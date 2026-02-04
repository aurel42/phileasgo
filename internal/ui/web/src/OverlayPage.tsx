import { useEffect, useState, useRef } from 'react';
import { useTelemetry } from './hooks/useTelemetry';
import { useNarrator } from './hooks/useNarrator';
import { useAudio } from './hooks/useAudio';
import { useTrackedPOIs } from './hooks/usePOIs';
import { OverlayMiniMap } from './components/OverlayMiniMap';
import { OverlayPOIPanel } from './components/OverlayPOIPanel';
import { OverlayTelemetryBar } from './components/OverlayTelemetryBar';
import './overlay.css';

const OverlayPage = () => {
    const { data: telemetry } = useTelemetry();
    const { status: narratorStatus } = useNarrator();
    const { status: audioStatus } = useAudio();
    const pois = useTrackedPOIs();

    // Throttle telemetry updates for the map to 2s to reduce jitter/load
    const [throttledTelemetry, setThrottledTelemetry] = useState(telemetry);
    const lastThrottleRef = useRef(0);

    useEffect(() => {
        if (!telemetry) return;
        const now = Date.now();
        if (now - lastThrottleRef.current > 2000) {
            setThrottledTelemetry(telemetry);
            lastThrottleRef.current = now;
        }
    }, [telemetry]);

    // Set body class for transparent background
    useEffect(() => {
        document.body.classList.add('overlay-mode');
        document.documentElement.classList.add('overlay-mode');
        return () => {
            document.body.classList.remove('overlay-mode');
            document.documentElement.classList.remove('overlay-mode');
        };
    }, []);

    // Config state
    const [units, setUnits] = useState<'km' | 'nm'>('km');
    const [minPoiScore, setMinPoiScore] = useState<number | undefined>(undefined);
    const [showMapBox, setShowMapBox] = useState(true);
    const [showPOIInfo, setShowPOIInfo] = useState(true);
    const [showInfoBar, setShowInfoBar] = useState(true);
    useEffect(() => {
        const fetchConfig = () => {
            fetch('/api/config')
                .then(r => r.json())
                .then(data => {
                    if (data) {
                        if (data.range_ring_units) setUnits(data.range_ring_units);
                        if (typeof data.min_poi_score === 'number') setMinPoiScore(data.min_poi_score);
                        if (typeof data.show_map_box === 'boolean') setShowMapBox(data.show_map_box);
                        if (typeof data.show_poi_info === 'boolean') setShowPOIInfo(data.show_poi_info);
                        if (typeof data.show_info_bar === 'boolean') setShowInfoBar(data.show_info_bar);
                    }
                })
                .catch(() => { });
        };
        fetchConfig();
        const interval = setInterval(fetchConfig, 5000);
        return () => clearInterval(interval);
    }, []);

    const isConnected = telemetry?.SimState === 'active';

    // Current narrated POI (only use current_poi when actually playing a POI narration)
    const currentPoi = narratorStatus?.playback_status !== 'idle' && narratorStatus?.current_type === 'poi' ? narratorStatus?.current_poi : null;
    const isPlaying = narratorStatus?.playback_status === 'playing';

    // Calculate playback progress
    const playbackProgress = audioStatus?.duration && audioStatus.duration > 0
        ? audioStatus.position / audioStatus.duration
        : 0;

    // Merge active POI into pois if filtered out
    const displayPois = [...pois];
    if (currentPoi && !displayPois.find(p => p.wikidata_id === currentPoi.wikidata_id)) {
        displayPois.push(currentPoi);
    }
    if (narratorStatus?.preparing_poi && !displayPois.find(p => p.wikidata_id === narratorStatus.preparing_poi?.wikidata_id)) {
        displayPois.push(narratorStatus.preparing_poi);
    }

    return (
        <div className="overlay-root">
            <div className="overlay-container">
                {/* Mini-map in top-left */}
                {showMapBox && isConnected && throttledTelemetry && (
                    <OverlayMiniMap
                        lat={throttledTelemetry.Latitude}
                        lon={throttledTelemetry.Longitude}
                        heading={throttledTelemetry.Heading}
                        pois={displayPois}
                        minPoiScore={minPoiScore}
                        currentNarratedId={currentPoi?.wikidata_id}
                        preparingId={narratorStatus?.preparing_poi?.wikidata_id}
                        units={units}
                    />
                )}

                {/* POI panel in top-right */}
                {showPOIInfo && (
                    <OverlayPOIPanel
                        poi={currentPoi || null}
                        title={narratorStatus?.display_title || narratorStatus?.current_title}
                        displayThumbnail={narratorStatus?.display_thumbnail}
                        currentType={narratorStatus?.current_type}
                        playbackProgress={playbackProgress}
                        isPlaying={isPlaying}
                    />
                )}

                {/* Telemetry bar at bottom */}
                {showInfoBar && (
                    <OverlayTelemetryBar
                        telemetry={telemetry}
                    />
                )}
            </div>
        </div>
    );
};

export default OverlayPage;
