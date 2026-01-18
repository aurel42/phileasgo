import { useEffect, useState, useRef } from 'react';
import type { Telemetry } from '../types/telemetry';

interface OverlayTelemetryBarProps {
    telemetry?: Telemetry;
}

interface Stats {
    providers?: {
        wikidata?: { api_success: number; api_zero: number; api_errors: number; hit_rate: number };
        wikipedia?: { api_success: number; api_errors: number; hit_rate: number };
        gemini?: { api_success: number; api_errors: number };
        'edge-tts'?: { api_success: number; api_zero: number; api_errors: number };
        'azure-speech'?: { api_success: number; api_zero: number; api_errors: number };
        [key: string]: { api_success: number; api_zero?: number; api_errors: number; hit_rate?: number } | undefined;
    };
    tracking?: { active_pois?: number };
}

interface Config {
    filter_mode?: string;
    target_poi_count?: number;
    min_poi_score?: number;
    narration_frequency?: number;
    text_length?: number;
    llm_provider?: string;
    tts_engine?: string;
}

interface Geography {
    city: string;
    region?: string;
    country: string;
}

export const OverlayTelemetryBar = ({ telemetry }: OverlayTelemetryBarProps) => {
    const [stats, setStats] = useState<Stats | null>(null);
    const [version, setVersion] = useState<string>('...');
    const [config, setConfig] = useState<Config>({});
    const [location, setLocation] = useState<Geography | null>(null);
    const [logLine, setLogLine] = useState<string>('');

    // Use ref to access latest telemetry in interval without resetting it
    const telemetryRef = useRef(telemetry);
    useEffect(() => { telemetryRef.current = telemetry; }, [telemetry]);

    useEffect(() => {
        const fetchStats = () => {
            fetch('/api/stats')
                .then(r => r.json())
                .then(data => setStats(data))
                .catch(() => { });
        };

        const fetchVersion = () => {
            fetch('/api/version')
                .then(r => r.json())
                .then(data => setVersion(data.version || '?'))
                .catch(() => { });
        };

        const fetchConfig = () => {
            fetch('/api/config')
                .then(r => r.json())
                .then(data => setConfig(data))
                .catch(() => { });
        };

        const fetchLocation = () => {
            const t = telemetryRef.current;
            if (!t) return;
            fetch(`/api/geography?lat=${t.Latitude}&lon=${t.Longitude}`)
                .then(r => r.json())
                .then(data => setLocation(data))
                .catch(() => { });
        };

        const fetchLog = () => {
            fetch('/api/log/latest')
                .then(r => r.json())
                .then(data => setLogLine(data.log || ''))
                .catch(() => { });
        };

        fetchStats();
        fetchVersion();
        fetchConfig();
        fetchLocation();
        fetchLog();

        const statsInterval = setInterval(fetchStats, 5000);
        const configInterval = setInterval(fetchConfig, 5000);
        const locInterval = setInterval(fetchLocation, 10000);
        const logInterval = setInterval(fetchLog, 1000);

        return () => {
            clearInterval(statsInterval);
            clearInterval(configInterval);
            clearInterval(locInterval);
            clearInterval(logInterval);
        };
    }, []);

    if (!telemetry) {
        return (
            <div className="overlay-telemetry-bar">
                <div className="stats-row">
                    <div className="stat-box">
                        <div className="stat-value">
                            <span className="status-dot error"></span>
                            Disconnected
                        </div>
                    </div>
                </div>
            </div>
        );
    }

    const wdStats = stats?.providers?.wikidata || { api_success: 0 };
    const wpStats = stats?.providers?.wikipedia || { api_success: 0 };
    const geminiStats = stats?.providers?.gemini || { api_success: 0 };
    const ttsStats = stats?.providers?.['edge-tts'] || stats?.providers?.['azure-speech'] || { api_success: 0 };

    return (
        <div className="overlay-telemetry-bar">
            {/* Wrapper for boxes to control width independent of log line */}
            <div className="stats-row">

                {/* Tracking (Vertical) */}
                <div className="stat-box" style={{ alignItems: 'flex-start', minWidth: '120px' }}>
                    <div className="stat-value" style={{ fontFamily: 'monospace', fontSize: '14px', display: 'flex', flexDirection: 'column', gap: '2px' }}>
                        <div style={{ whiteSpace: 'nowrap' }}><span style={{ color: '#ccc', width: '32px', display: 'inline-block' }}>HDG</span> {Math.round(telemetry.Heading)}°</div>
                        <div style={{ whiteSpace: 'nowrap' }}><span style={{ color: '#ccc', width: '32px', display: 'inline-block' }}>GS</span> {Math.round(telemetry.GroundSpeed)} <span className="unit" style={{ fontSize: '14px', color: '#ccc' }}>kts</span></div>
                        <div style={{ whiteSpace: 'nowrap' }}><span style={{ color: '#ccc', width: '32px', display: 'inline-block' }}>AGL</span> {Math.round(telemetry.AltitudeAGL)} <span className="unit" style={{ fontSize: '14px', color: '#ccc' }}>ft</span></div>
                        <div style={{ whiteSpace: 'nowrap' }}><span style={{ color: '#ccc', width: '32px', display: 'inline-block' }}>MSL</span> {Math.round(telemetry.AltitudeMSL)} <span className="unit" style={{ fontSize: '14px', color: '#ccc' }}>ft</span></div>
                    </div>
                </div>

                {/* Position */}
                <div className="stat-box" style={{ minWidth: location?.city ? '220px' : '180px' }}>
                    {location?.city ? (
                        <>
                            <div className="stat-value" style={{ fontSize: '16px', color: '#fff', textAlign: 'center', fontFamily: 'Inter, sans-serif', fontWeight: 600 }}>
                                <span style={{ color: '#ddd', fontWeight: 400, marginRight: '6px', fontSize: '14px' }}>near</span>
                                {location.city}
                            </div>
                            <div style={{ color: '#eee', fontSize: '14px', marginTop: '4px', textAlign: 'center', fontFamily: 'Inter, sans-serif' }}>
                                {location.region ? `${location.region}, ` : ''}{location.country}
                            </div>
                            <div style={{ fontSize: '13px', fontFamily: 'monospace', color: '#ccc', marginTop: '8px', textAlign: 'center', letterSpacing: '0.5px' }}>
                                {telemetry.Latitude.toFixed(4)}, {telemetry.Longitude.toFixed(4)}
                            </div>
                        </>
                    ) : (
                        <div className="stat-value" style={{ fontSize: '14px', fontFamily: 'monospace', textAlign: 'center' }}>
                            <span className="unit" style={{ color: '#ccc' }}>LAT </span>{telemetry.Latitude.toFixed(4)} <br />
                            <span className="unit" style={{ color: '#ccc' }}>LON </span>{telemetry.Longitude.toFixed(4)}
                        </div>
                    )}
                </div>

                {/* APIs (Vertical matching Tracking) */}
                <div className="stat-box" style={{ minWidth: '160px', alignItems: 'flex-start' }}>
                    <div className="stat-value" style={{
                        fontFamily: 'monospace',
                        fontSize: '14px',
                        display: 'grid',
                        gridTemplateColumns: 'max-content 1fr',
                        columnGap: '16px',
                        rowGap: '2px',
                        textAlign: 'left'
                    }}>
                        <div style={{ color: '#ccc' }}>Wikidata API</div>
                        <div style={{ textAlign: 'right' }}>{wdStats.api_success}</div>

                        <div style={{ color: '#ccc' }}>Wikipedia</div>
                        <div style={{ textAlign: 'right' }}>{wpStats.api_success}</div>

                        <div style={{ color: '#ccc' }}>LLM <span style={{ fontSize: '0.85em', opacity: 0.8 }}>({config.llm_provider || '?'})</span></div>
                        <div style={{ textAlign: 'right' }}>{geminiStats.api_success}</div>

                        <div style={{ color: '#ccc' }}>TTS <span style={{ fontSize: '0.85em', opacity: 0.8 }}>({config.tts_engine || '?'})</span></div>
                        <div style={{ textAlign: 'right' }}>{ttsStats.api_success}</div>
                    </div>
                </div>

                {/* Config (Prose) */}
                <div className="stat-box" style={{ minWidth: '350px', alignItems: 'flex-start', textAlign: 'left', padding: '12px 16px' }}>
                    <div className="stat-value" style={{ fontSize: '14px', lineHeight: '1.5', fontFamily: 'Inter, sans-serif', fontWeight: 400, color: '#eee' }}>
                        <div style={{ marginBottom: '4px' }}>
                            {config.filter_mode === 'adaptive'
                                ? `Map Marker Mode: Adaptive (~${config.target_poi_count ?? 20} new POIs)`
                                : `Map Marker Mode: Fixed Score (>= ${(config.min_poi_score ?? 0.5).toFixed(1)})`}
                        </div>
                        <div>
                            {(() => {
                                const freq = config.narration_frequency ?? 3;
                                const pacing = ['Rarely', 'Normal', 'Active', 'Busy', 'Constant'][freq - 1] || 'Active';

                                const len = config.text_length ?? 3;
                                const detail = ['Shortest', 'Shorter', 'Normal', 'Longer', 'Longest'][len - 1] || 'Normal';

                                return `Narration frequency: ${pacing}. Text length: ${detail}.`;
                            })()}
                        </div>
                    </div>
                </div>

                {/* Branding - Restored Original */}
                <div className="stat-box branding-box" style={{ minWidth: '120px' }}>
                    <div className="stat-value" style={{ fontSize: '11px', lineHeight: '1.3', textAlign: 'center', color: '#4a9eff', fontWeight: 700, fontFamily: 'sans-serif' }}>
                        PHILEAS<br />
                        TOUR GUIDE<br />
                        FOR MSFS
                    </div>
                    <div className="stat-value" style={{ textAlign: 'center', marginTop: '6px', fontSize: '12px', color: '#bbb' }}>
                        {version}
                    </div>
                </div>

            </div> {/* End of stats-row */}

            {/* Log Line (Outside of flow, absolute positioned in CSS) */}
            <div className="log-line">
                {logLine}
            </div>
        </div>
    );
};
