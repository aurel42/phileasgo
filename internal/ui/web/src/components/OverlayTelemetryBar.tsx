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

import { useGeography } from '../hooks/useGeography';

export const OverlayTelemetryBar = ({ telemetry }: OverlayTelemetryBarProps) => {
    const [stats, setStats] = useState<Stats | null>(null);
    const [version, setVersion] = useState<string>('...');
    const [config, setConfig] = useState<Config>({});
    const { location } = useGeography(telemetry);
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

        const fetchLog = () => {
            fetch('/api/log/latest')
                .then(r => r.json())
                .then(data => setLogLine(data.log || ''))
                .catch(() => { });
        };

        fetchStats();
        fetchVersion();
        fetchConfig();
        fetchLog();

        const statsInterval = setInterval(fetchStats, 5000);
        const configInterval = setInterval(fetchConfig, 5000);
        const logInterval = setInterval(fetchLog, 1000);

        return () => {
            clearInterval(statsInterval);
            clearInterval(configInterval);
            clearInterval(logInterval);
        };
    }, []);

    if (!telemetry || telemetry.SimState === 'disconnected' || !telemetry.Valid) {
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

                {/* 1. HDG @ GS (Telemetry Card) - GRID LAYOUT */}
                <div className="stat-box" style={{ alignItems: 'flex-start', minWidth: '140px' }}>
                    <div className="stat-value" style={{
                        display: 'grid',
                        gridTemplateColumns: '30px 1fr 34px', // Increased unit width for 'deg.' and 'kts'
                        columnGap: '8px',
                        rowGap: '2px',
                        textAlign: 'right', // Align numbers to right
                        alignItems: 'baseline'
                    }}>
                        {/* HDG */}
                        <div className="role-label-overlay" style={{ textAlign: 'left' }}>HDG</div>
                        <div className="role-num-lg" style={{ fontSize: '20px' }}>{Math.round(telemetry.Heading)}</div>
                        <div className="role-label-overlay" style={{ fontSize: '14px', textAlign: 'left' }}>deg.</div>

                        {/* GS */}
                        <div className="role-label-overlay" style={{ textAlign: 'left' }}>GS</div>
                        <div className="role-num-lg" style={{ fontSize: '20px' }}>{Math.round(telemetry.GroundSpeed)}</div>
                        <div className="role-label-overlay" style={{ fontSize: '14px', textAlign: 'left' }}>kts</div>

                        {/* AGL */}
                        <div className="role-label-overlay" style={{ textAlign: 'left' }}>AGL</div>
                        <div className="role-num-lg" style={{ fontSize: '20px' }}>{Math.round(telemetry.AltitudeAGL)}</div>
                        <div className="role-label-overlay" style={{ fontSize: '14px', textAlign: 'left' }}>ft</div>

                        {/* MSL */}
                        <div className="role-label-overlay" style={{ textAlign: 'left' }}>MSL</div>
                        <div className="role-num-lg" style={{ fontSize: '20px' }}>{Math.round(telemetry.AltitudeMSL)}</div>
                        <div className="role-label-overlay" style={{ fontSize: '14px', textAlign: 'left' }}>ft</div>
                    </div>
                </div>

                {/* 2. Position Card */}
                <div className="stat-box" style={{ minWidth: (location?.city || location?.country) ? '220px' : '180px' }}>
                    {(location?.city || location?.country) ? (
                        <>
                            <div className="role-text-lg" style={{ textAlign: 'center' }}>
                                {location.city ? (
                                    location.city === 'Unknown' ? (
                                        <span>Far from civilization</span>
                                    ) : (
                                        <>
                                            <span className="role-label-overlay" style={{ marginRight: '6px' }}>near</span>
                                            {location.city}
                                        </>
                                    )
                                ) : (
                                    <span>{location.country}</span>
                                )}
                            </div>
                            <div className="role-text-sm" style={{ textAlign: 'center', marginTop: '4px' }}>
                                {location.city_country_code && location.country_code && location.city_country_code !== location.country_code ? (
                                    <>
                                        <div>{location.city_region ? `${location.city_region}, ` : ''}{location.city_country}</div>
                                        <div style={{ color: 'var(--accent)', marginTop: '2px' }}>in {location.country}</div>
                                    </>
                                ) : (
                                    <>{location.region ? `${location.region}, ` : ''}{location.city ? location.country : (location.region ? '' : '')}</>
                                )}
                            </div>
                            <div className="role-num-sm" style={{ color: 'var(--muted)', marginTop: '8px', textAlign: 'center' }}>
                                {telemetry.Latitude.toFixed(4)}, {telemetry.Longitude.toFixed(4)}
                            </div>
                        </>
                    ) : (
                        <div className="stat-value" style={{ textAlign: 'center' }}>
                            <span className="role-label-overlay">LAT </span><span className="role-num-sm">{telemetry.Latitude.toFixed(4)}</span> <br />
                            <span className="role-label-overlay">LON </span><span className="role-num-sm">{telemetry.Longitude.toFixed(4)}</span>
                        </div>
                    )}
                </div>

                {/* 3. LLM Pipeline Card */}
                <div className="stat-box" style={{ minWidth: '180px', alignItems: 'flex-start' }}>
                    <div className="role-label-overlay" style={{ marginBottom: '6px', color: 'var(--accent)', fontSize: '12px', borderBottom: '1px solid rgba(212,175,55,0.2)', width: '100%' }}>LLM PIPELINE</div>
                    <div className="stat-value" style={{
                        display: 'grid',
                        gridTemplateColumns: 'max-content 1fr max-content',
                        columnGap: '8px',
                        rowGap: '2px',
                        textAlign: 'left',
                        alignItems: 'baseline',
                        width: '100%'
                    }}>
                        {(() => {
                            if (!stats?.providers) return null;
                            const fallbackOrder = (stats as any).llm_fallback || [];
                            const toRoman = (num: number) => ["I", "II", "III", "IV", "V"][num] || (num + 1).toString();

                            return Object.entries(stats.providers)
                                .filter(([key]) => fallbackOrder.includes(key))
                                .sort(([keyA], [keyB]) => fallbackOrder.indexOf(keyA) - fallbackOrder.indexOf(keyB))
                                .map(([key, data], idx) => {
                                    if (!data) return null;
                                    if (data.api_success === 0 && data.api_errors === 0) return null;
                                    const label = key.toUpperCase().replace('-', ' ');
                                    return (
                                        <div key={key} style={{ display: 'contents' }}>
                                            <div style={{ display: 'flex', alignItems: 'baseline' }}>
                                                <span className="roman-numeral">{toRoman(idx)}</span>
                                                <div className="role-header" style={{ fontSize: '14px', whiteSpace: 'nowrap' }}>
                                                    {label}
                                                </div>
                                            </div>
                                            <div className="role-num-sm" style={{ textAlign: 'right', paddingRight: '4px' }}>
                                                <span style={{ color: 'var(--success)' }}>{data.api_success}</span>
                                                <span style={{ color: 'var(--muted)', margin: '0 4px', fontSize: '10px' }}>◆</span>
                                                <span style={{ color: 'var(--error)' }}>{data.api_errors}</span>
                                            </div>
                                            <div style={{ width: '12px', fontSize: '14px' }}>{data.free_tier === false ? '£' : ''}</div>
                                        </div>
                                    );
                                });
                        })()}
                    </div>
                </div>

                {/* 3b. Data Services Card */}
                <div className="stat-box" style={{ minWidth: '160px', alignItems: 'flex-start' }}>
                    <div className="role-label-overlay" style={{ marginBottom: '6px', color: 'var(--accent)', fontSize: '12px', borderBottom: '1px solid rgba(212,175,55,0.2)', width: '100%' }}>DATA SERVICES</div>
                    <div className="stat-value" style={{
                        display: 'grid',
                        gridTemplateColumns: 'max-content 1fr max-content',
                        columnGap: '8px',
                        rowGap: '2px',
                        textAlign: 'left',
                        alignItems: 'baseline',
                        width: '100%'
                    }}>
                        {(() => {
                            if (!stats?.providers) return null;
                            const fallbackOrder = (stats as any).llm_fallback || [];

                            return Object.entries(stats.providers)
                                .filter(([key]) => !fallbackOrder.includes(key))
                                .sort(([keyA], [keyB]) => keyA.localeCompare(keyB))
                                .map(([key, data]) => {
                                    if (!data) return null;
                                    if (data.api_success === 0 && data.api_errors === 0) return null;
                                    const label = key.toUpperCase().replace('-', ' ');
                                    return (
                                        <div key={key} style={{ display: 'contents' }}>
                                            <div className="role-header" style={{ fontSize: '14px', whiteSpace: 'nowrap' }}>{label}</div>
                                            <div className="role-num-sm" style={{ textAlign: 'right', paddingRight: '4px' }}>
                                                <span style={{ color: 'var(--success)' }}>{data.api_success}</span>
                                                <span style={{ color: 'var(--muted)', margin: '0 4px', fontSize: '10px' }}>◆</span>
                                                <span style={{ color: 'var(--error)' }}>{data.api_errors}</span>
                                            </div>
                                            <div style={{ width: '12px', fontSize: '14px' }}>{data.free_tier === false ? '£' : ''}</div>
                                        </div>
                                    );
                                });
                        })()}
                    </div>
                </div>

                {/* 4. Stats Card - GRID LAYOUT */}
                <div className="stat-box" style={{ minWidth: '140px', alignItems: 'flex-start' }}>
                    <div className="stat-value" style={{
                        display: 'grid',
                        gridTemplateColumns: 'max-content 1fr 24px', // Align with Telemetry Card logic
                        columnGap: '8px',
                        rowGap: '2px',
                        alignItems: 'baseline'
                    }}>
                        {/* MEM RSS */}
                        <div className="role-label-overlay">MEM (RSS)</div>
                        <div className="role-num-sm" style={{ textAlign: 'right' }}>{stats?.system?.memory_alloc_mb || 0}</div>
                        <div className="role-label-overlay" style={{ fontSize: '12px' }}>MB</div>

                        {/* MEM MAX */}
                        <div className="role-label-overlay">MEM (max)</div>
                        <div className="role-num-sm" style={{ textAlign: 'right' }}>{stats?.system?.memory_max_mb || 0}</div>
                        <div className="role-label-overlay" style={{ fontSize: '12px' }}>MB</div>

                        {/* POIS */}
                        <div className="role-label-overlay">Tracked</div>
                        <div className="role-num-sm" style={{ textAlign: 'right' }}>{stats?.tracking?.active_pois || 0}</div>
                        <div className="role-label-overlay" style={{ fontSize: '12px' }}>POIs</div>
                    </div>
                </div>

                {/* 5. Branding Card */}
                <div className="stat-box branding-box" style={{ minWidth: '120px' }}>
                    <div className="role-title" style={{ fontSize: '18px', lineHeight: '1.1', textAlign: 'center' }}>
                        PHILEAS<br />
                        TOUR GUIDE
                    </div>
                    {/* Use role-num-sm purely without overrides, maybe color muted */}
                    <div className="role-num-sm" style={{ textAlign: 'center', marginTop: '6px', color: '#bbb' }}>
                        {version}
                    </div>
                </div>

                <div className="stat-box config-box">
                    <div className="overlay-config-status" style={{ display: 'grid', gridTemplateColumns: 'min-content min-content', gap: '4px 12px', alignItems: 'center' }}>
                        {/* Row 1: SIM */}
                        <span className="role-label-overlay" style={{ textAlign: 'left' }}>SIM</span>
                        <span style={{ fontSize: '10px', display: 'flex', alignItems: 'center', justifyContent: 'flex-end' }}>
                            {telemetry.SimState === 'active' ? '🟢' : telemetry.SimState === 'inactive' ? '🟠' : '🔴'}
                        </span>

                        {/* Row 2: MODE */}
                        <span className="role-label-overlay" style={{ textAlign: 'left', whiteSpace: 'nowrap' }}>{config.filter_mode === 'adaptive' ? 'ADAPTIVE' : 'FIXED'}</span>
                        <span className="icon" style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end' }}>{config.filter_mode === 'adaptive' ? '⚡' : '🎯'}</span>

                        {/* Row 3: FRQ */}
                        <span className="role-label-overlay" style={{ textAlign: 'left' }}>FRQ</span>
                        <div className="pips" style={{ display: 'flex', gap: '2px', alignItems: 'center', justifyContent: 'flex-end' }}>
                            {[1, 2, 3, 4, 5].map(v => (
                                <div key={v} className={`pip ${v <= (config.narration_frequency || 0) ? 'active' : ''} ${v > 3 && v <= (config.narration_frequency || 0) ? 'high' : ''}`} />
                            ))}
                        </div>

                        {/* Row 4: LEN */}
                        <span className="role-label-overlay" style={{ textAlign: 'left' }}>LEN</span>
                        <div className="pips" style={{ display: 'flex', gap: '2px', alignItems: 'center', justifyContent: 'flex-end' }}>
                            {[1, 2, 3, 4, 5].map(v => (
                                <div key={v} className={`pip ${v <= (config.text_length || 0) ? 'active' : ''} ${v > 4 && v <= (config.text_length || 0) ? 'high' : ''}`} />
                            ))}
                        </div>
                    </div>
                </div>

            </div> {/* End of stats-row */}

            {/* Log Line (Outside of flow, absolute positioned in CSS) */}
            {config.show_log_line && (
                <div className="log-line role-label-overlay" style={{ fontStyle: 'italic', fontSize: '16px', lineHeight: '30px' }}>
                    {logLine}
                </div>
            )}
        </div>
    );
};
