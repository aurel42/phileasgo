import { useEffect, useState } from 'react';
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

    // Set body class for transparent background
    useEffect(() => {
        document.body.classList.add('overlay-mode');
        document.documentElement.classList.add('overlay-mode');
        return () => {
            document.body.classList.remove('overlay-mode');
            document.documentElement.classList.remove('overlay-mode');
        };
    }, []);

    // Fetch min_poi_score to sync with main app filtering
    const [minPoiScore, setMinPoiScore] = useState<number | undefined>(undefined);
    useEffect(() => {
        const fetchConfig = () => {
            fetch('/api/config')
                .then(r => r.json())
                .then(data => {
                    if (data && typeof data.min_poi_score === 'number') {
                        setMinPoiScore(data.min_poi_score);
                    }
                })
                .catch(() => { });
        };
        fetchConfig();
        const interval = setInterval(fetchConfig, 5000);
        return () => clearInterval(interval);
    }, []);

    const isConnected = telemetry?.SimState === 'active';

    // Current narrated POI
    const currentPoi = narratorStatus?.playback_status !== 'idle' ? narratorStatus?.current_poi : null;
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
                {isConnected && telemetry && (
                    <OverlayMiniMap
                        lat={telemetry.Latitude}
                        lon={telemetry.Longitude}
                        heading={telemetry.Heading}
                        pois={displayPois}
                        minPoiScore={minPoiScore}
                        currentNarratedId={currentPoi?.wikidata_id}
                        preparingId={narratorStatus?.preparing_poi?.wikidata_id}
                        units="nm"
                    />
                )}

                {/* POI panel in top-right */}
                <OverlayPOIPanel
                    poi={currentPoi || null}
                    playbackProgress={playbackProgress}
                    isPlaying={isPlaying}
                />

                {/* Telemetry bar at bottom */}
                <OverlayTelemetryBar
                    telemetry={telemetry}
                />
            </div>
        </div>
    );
};

export default OverlayPage;
