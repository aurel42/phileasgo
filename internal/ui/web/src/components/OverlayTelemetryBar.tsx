import { useEffect, useState, useRef } from 'react';
import type { Telemetry } from '../types/telemetry';

interface OverlayTelemetryBarProps {
    telemetry?: Telemetry;
}

interface Stats {
    providers?: {
        wikidata?: { api_success: number; api_zero: number; api_errors: number; hit_rate: number; free_tier?: boolean };
        wikipedia?: { api_success: number; api_errors: number; hit_rate: number; free_tier?: boolean };
        gemini?: { api_success: number; api_errors: number; free_tier?: boolean };
        'edge-tts'?: { api_success: number; api_zero: number; api_errors: number; free_tier?: boolean };
        'azure-speech'?: { api_success: number; api_zero: number; api_errors: number; free_tier?: boolean };
        [key: string]: { api_success: number; api_zero?: number; api_errors: number; hit_rate?: number; free_tier?: boolean } | undefined;
    };
    system?: { memory_alloc_mb: number; memory_max_mb: number; goroutines: number };
    tracking?: { active_pois: number };
}

interface Config {
    filter_mode?: string;
    target_poi_count?: number;
    min_poi_score?: number;
    narration_frequency?: number;
    text_length?: number;
    llm_provider?: string;
    tts_engine?: string;
    show_log_line?: boolean;
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
                                {location.city === 'Unknown' ? (
                                    <span style={{ color: '#fff' }}>Far from civilization</span>
                                ) : (
                                    <>
                                        <span style={{ color: '#ddd', fontWeight: 400, marginRight: '6px', fontSize: '14px' }}>near</span>
                                        {location.city}
                                    </>
                                )}
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

                {/* APIs (Dynamic List) */}
                <div className="stat-box" style={{ minWidth: '160px', alignItems: 'flex-start' }}>
                    <div className="stat-value" style={{
                        fontFamily: 'monospace',
                        fontSize: '14px',
                        display: 'grid',
                        gridTemplateColumns: 'max-content 1fr 24px',
                        columnGap: '12px',
                        rowGap: '2px',
                        textAlign: 'left'
                    }}>
                        {stats?.providers && Object.entries(stats.providers)
                            .sort(([keyA], [keyB]) => keyA.localeCompare(keyB)) // Optional: Alphabetical sort
                            .map(([key, data]) => {
                                if (!data) return null;
                                // Filter empty stats (0 success AND 0 errors)
                                if (data.api_success === 0 && data.api_errors === 0) return null;

                                const label = key.toUpperCase().replace('-', ' ');
                                return (
                                    <div key={key} style={{ display: 'contents' }}>
                                        <div style={{ color: '#ccc', whiteSpace: 'nowrap' }}>
                                            {label}
                                            {data.free_tier === false && <span style={{ marginLeft: '4px' }}>💵</span>}
                                        </div>
                                        <div style={{ textAlign: 'right', paddingRight: '4px' }}>{data.api_success}</div>
                                        <div style={{ width: '24px' }}></div>
                                    </div>
                                );
                            })}
                    </div>
                </div>

                {/* System Stats (Vertical matching Tracking) */}
                <div className="stat-box" style={{ minWidth: '140px', alignItems: 'flex-start' }}>
                    <div className="stat-value" style={{
                        fontFamily: 'monospace',
                        fontSize: '14px',
                        display: 'grid',
                        gridTemplateColumns: 'max-content 1fr',
                        columnGap: '12px',
                        rowGap: '2px',
                        textAlign: 'left'
                    }}>
                        <div style={{ color: '#ccc' }}>MEM (RSS)</div>
                        <div style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>{stats?.system?.memory_alloc_mb || 0} <span className="unit" style={{ fontSize: '12px', color: '#888' }}>MB</span></div>

                        <div style={{ color: '#ccc' }}>MEM (max)</div>
                        <div style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>{stats?.system?.memory_max_mb || 0} <span className="unit" style={{ fontSize: '12px', color: '#888' }}>MB</span></div>

                        <div style={{ color: '#ccc' }}>Tracked POIs</div>
                        <div style={{ textAlign: 'right' }}>{stats?.tracking?.active_pois || 0}</div>
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

                <div className="stat-box config-box">
                    <div className="overlay-config-status">
                        <div className="config-item">
                            <span className="config-label">SIM</span>
                            <span style={{ fontSize: '10px' }}>
                                {telemetry.SimState === 'active' ? '🟢' : telemetry.SimState === 'inactive' ? '🟠' : '🔴'}
                            </span>
                        </div>
                        <div className="config-item">
                            <span className="config-label">{config.filter_mode === 'adaptive' ? 'ADAPTIVE' : 'FIXED'}</span>
                            <span className="icon">{config.filter_mode === 'adaptive' ? '⚡' : '🎯'}</span>
                        </div>
                        <div className="config-item">
                            <span className="config-label">FRQ</span>
                            <div className="pips">
                                {[1, 2, 3, 4, 5].map(v => (
                                    <div key={v} className={`pip ${v <= (config.narration_frequency || 0) ? 'active' : ''} ${v > 3 && v <= (config.narration_frequency || 0) ? 'high' : ''}`} />
                                ))}
                            </div>
                        </div>
                        <div className="config-item">
                            <span className="config-label">LEN</span>
                            <div className="pips">
                                {[1, 2, 3, 4, 5].map(v => (
                                    <div key={v} className={`pip ${v <= (config.text_length || 0) ? 'active' : ''} ${v > 4 && v <= (config.text_length || 0) ? 'high' : ''}`} />
                                ))}
                            </div>
                        </div>
                    </div>
                </div>

            </div> {/* End of stats-row */}

            {/* Log Line (Outside of flow, absolute positioned in CSS) */}
            {config.show_log_line && (
                <div className="log-line">
                    {logLine}
                </div>
            )}
        </div>
    );
};
