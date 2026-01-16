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

        fetchStats();
        fetchVersion();
        fetchConfig();
        fetchLocation();

        const statsInterval = setInterval(fetchStats, 5000);
        const configInterval = setInterval(fetchConfig, 5000);
        const locInterval = setInterval(fetchLocation, 10000);

        return () => {
            clearInterval(statsInterval);
            clearInterval(configInterval);
            clearInterval(locInterval);
        };
    }, []);

    if (!telemetry) {
        return (
            <div className="overlay-telemetry-bar">
                <div className="stat-box">
                    <div className="stat-label">STATUS</div>
                    <div className="stat-value">
                        <span className="status-dot error"></span>
                        Disconnected
                    </div>
                </div>
            </div>
        );
    }

    const simState = telemetry.SimState || 'disconnected';
    const statusInfo = {
        active: { label: 'Active', className: 'connected' },
        inactive: { label: 'Paused', className: 'paused' },
        disconnected: { label: 'Disconnected', className: 'error' },
    }[simState] || { label: 'Unknown', className: '' };

    const wdStats = stats?.providers?.wikidata || { api_success: 0, api_errors: 0 };
    const wpStats = stats?.providers?.wikipedia || { api_success: 0, api_errors: 0 };
    const geminiStats = stats?.providers?.gemini || { api_success: 0, api_errors: 0 };
    const ttsStats = stats?.providers?.['edge-tts'] || stats?.providers?.['azure-speech'] || { api_success: 0, api_errors: 0 };

    return (
        <div className="overlay-telemetry-bar">
            {/* HDG @ GS */}
            <div className="stat-box">
                <div className="stat-label">HDG @ GS</div>
                <div className="stat-value" style={{ fontFamily: 'monospace' }}>
                    {Math.round(telemetry.Heading)}°
                </div>
                <div className="stat-value" style={{ fontFamily: 'monospace' }}>
                    {Math.round(telemetry.GroundSpeed)}<span className="unit"> kts</span>
                </div>
            </div>

            {/* Altitude */}
            <div className="stat-box">
                <div className="stat-label">ALTITUDE</div>
                <div className="stat-value" style={{ fontFamily: 'monospace' }}>
                    {Math.round(telemetry.AltitudeAGL)} <span className="unit">ft AGL</span>
                </div>
                <div className="stat-sub" style={{ fontFamily: 'monospace' }}>
                    {Math.round(telemetry.AltitudeMSL)} <span className="unit">ft MSL</span>
                </div>
            </div>

            {/* Position */}
            <div className="stat-box" style={{ minWidth: '280px' }}>
                <div className="stat-label">POSITION</div>
                <div className="stat-value" style={{ fontSize: '13px', fontFamily: 'monospace', whiteSpace: 'nowrap' }}>
                    <span className="unit">LAT </span>{telemetry.Latitude.toFixed(4)}
                    <span className="unit" style={{ marginLeft: '8px' }}>LON </span>{telemetry.Longitude.toFixed(4)}
                </div>
                {location?.city && (
                    <div className="stat-value" style={{ fontSize: '14px', marginTop: '4px', color: '#fff', fontFamily: 'monospace' }}>
                        <span style={{ color: '#888', marginRight: '4px', fontSize: '12px' }}>near</span>
                        {location.city}, <span style={{ color: '#aaa' }}>
                            {location.region ? `${location.region}, ` : ''}{location.country}
                        </span>
                    </div>
                )}
            </div>

            {/* Status */}
            <div className="stat-box">
                <div className="stat-label">STATUS</div>
                <div className="stat-value">
                    {simState !== 'disconnected' && (
                        <span className={`flight-stage ${telemetry.IsOnGround ? '' : 'active'}`}>
                            {telemetry.IsOnGround ? 'GROUND' : 'AIR'}
                        </span>
                    )}
                </div>
                <div className="stat-sub">
                    <span className={`status-dot ${statusInfo.className}`}></span>
                    {statusInfo.label}
                </div>
            </div>

            {/* Wikidata */}
            <div className="stat-box">
                <div className="stat-label">WIKIDATA</div>
                <div className="stat-value">
                    <span className="stat-success">{wdStats.api_success}</span>
                    <span className="stat-neutral"> / </span>
                    <span className="stat-error">{wdStats.api_errors}</span>
                </div>
            </div>

            {/* Wikipedia */}
            <div className="stat-box">
                <div className="stat-label">WIKIPEDIA</div>
                <div className="stat-value">
                    <span className="stat-success">{wpStats.api_success}</span>
                    <span className="stat-neutral"> / </span>
                    <span className="stat-error">{wpStats.api_errors}</span>
                </div>
            </div>

            {/* Gemini */}
            <div className="stat-box">
                <div className="stat-label">GEMINI</div>
                <div className="stat-value">
                    <span className="stat-success">{geminiStats.api_success}</span>
                    <span className="stat-neutral"> / </span>
                    <span className="stat-error">{geminiStats.api_errors}</span>
                </div>
            </div>

            {/* TTS */}
            <div className="stat-box">
                <div className="stat-label">TTS</div>
                <div className="stat-value">
                    <span className="stat-success">{ttsStats.api_success}</span>
                    <span className="stat-neutral"> / </span>
                    <span className="stat-error">{ttsStats.api_errors}</span>
                </div>
            </div>

            {/* POI Filter */}
            <div className="stat-box">
                <div className="stat-label">POI FILTER</div>
                <div className="stat-value" style={{ fontSize: '12px' }}>
                    {config.filter_mode === 'adaptive'
                        ? `Adaptive (${config.target_poi_count ?? 20})`
                        : `Fixed (${(config.min_poi_score ?? 0.5).toFixed(1)})`}
                </div>
            </div>



            {/* Frequency */}
            <div className="stat-box">
                <div className="stat-label">NARRATION PACE</div>
                <div className="stat-value" style={{ fontSize: '12px' }}>
                    {(() => {
                        const freq = config.narration_frequency ?? 3;
                        const labels = ['Rarely', 'Normal', 'Active', 'Busy', 'Constant'];
                        return labels[freq - 1] || 'Active';
                    })()}
                </div>
            </div>

            {/* Length */}
            <div className="stat-box">
                <div className="stat-label">NARRATION DETAIL</div>
                <div className="stat-value" style={{ fontSize: '12px' }}>
                    {(() => {
                        const len = config.text_length ?? 3;
                        const labels = ['Shortest', 'Shorter', 'Normal', 'Longer', 'Longest'];
                        return labels[len - 1] || 'Normal';
                    })()}
                </div>
            </div>

            {/* Branding */}
            <div className="stat-box branding-box" style={{ minWidth: '120px' }}>
                <div className="stat-value" style={{ fontSize: '10px', lineHeight: '1.2', textAlign: 'center', color: '#4a9eff' }}>
                    PHILEAS<br />
                    TOUR GUIDE<br />
                    FOR MSFS
                </div>
                <div className="stat-value" style={{ textAlign: 'center', marginTop: '4px', fontSize: '12px', color: '#888' }}>
                    {version}
                </div>
            </div>
        </div>
    );
};
