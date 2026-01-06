import { useEffect, useState } from 'react';
import type { Telemetry } from '../types/telemetry';
// @ts-ignore
import packageJson from '../../package.json';

interface InfoPanelProps {
    telemetry?: Telemetry;
    status: 'pending' | 'error' | 'success';
    isRetrying?: boolean;
    units: 'km' | 'nm';
    onUnitsChange: (units: 'km' | 'nm') => void;
    showCacheLayer: boolean;
    onCacheLayerChange: (show: boolean) => void;
    showVisibilityLayer: boolean;
    onVisibilityLayerChange: (show: boolean) => void;
    displayedCount: number;
    minPoiScore: number;
    onMinPoiScoreChange: (score: number) => void;
}

export const InfoPanel = ({
    telemetry, status, isRetrying, units, onUnitsChange,
    showCacheLayer, onCacheLayerChange,
    showVisibilityLayer, onVisibilityLayerChange,
    displayedCount,
    minPoiScore,
    onMinPoiScoreChange
}: InfoPanelProps) => {

    const [backendVersion, setBackendVersion] = useState<string | null>(null);
    const [configOpen, setConfigOpen] = useState(false);
    const [simSource, setSimSource] = useState<string>('mock');
    const [volume, setVolume] = useState<number>(1.0);
    const [stats, setStats] = useState<any>(null); // Quick any for stats map

    useEffect(() => {
        const fetchVersion = () => {
            fetch('/api/version')
                .then(r => r.json())
                .then(data => setBackendVersion(data.version))
                .catch(e => console.error("Failed to fetch backend version", e));
        };
        const fetchStats = () => {
            fetch('/api/stats')
                .then(r => r.json())
                .then(data => setStats(data))
                .catch(e => console.error("Failed to fetch stats", e));
        }

        // Then poll every 5 seconds to detect backend restart with new version
        const interval = setInterval(() => {
            fetchVersion();
            fetchStats();
        }, 5000);

        return () => {
            clearInterval(interval);
        };
    }, []);

    useEffect(() => {
        fetch('/api/config')
            .then(r => r.json())
            .then(data => {
                setSimSource(data.sim_source || 'mock');
                if (data.volume !== undefined) {
                    setVolume(data.volume);
                }
            })
            .catch(e => console.error("Failed to fetch config", e));
    }, []);

    const handleSimSourceChange = (source: string) => {
        setSimSource(source);
        fetch('/api/config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ sim_source: source })
        }).catch(e => console.error("Failed to update config", e));
    };

    const handleVolumeChange = (vol: number) => {
        setVolume(vol);
        fetch('/api/audio/volume', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ volume: vol })
        }).catch(e => console.error("Failed to set volume", e));
    };

    const frontendVersion = `v${packageJson.version}`;
    const versionMatch = backendVersion === frontendVersion;

    // We render the container even if loading/error to keep layout, but show message
    if (status === 'pending' && !isRetrying) {
        return (
            <div className="hud-container">
                <div className="hud-header loading"><span className="status-dot"></span> Connecting...</div>
                <div className="hud-footer">
                    <div className="hud-card footer">
                        {versionMatch ? (
                            <div className="version-info clean">{frontendVersion}</div>
                        ) : (
                            <div className="version-info warning">
                                ⚠ Frontend: {frontendVersion} / Backend: {backendVersion || '...'}
                            </div>
                        )}
                    </div>
                </div>
            </div>
        );
    }

    if (status === 'error' || (status === 'pending' && isRetrying) || !telemetry) {
        return (
            <div className="hud-container">
                <div className="hud-header error">
                    <span className="status-dot error"></span>
                    Connection Error
                    {isRetrying && <span style={{ marginLeft: '10px', fontSize: '12px', opacity: 0.8 }}>(Retrying...)</span>}
                </div>
                <div className="hud-footer">
                    <div className="hud-card footer">
                        {versionMatch ? (
                            <div className="version-info clean">{frontendVersion}</div>
                        ) : (
                            <div className="version-info warning">
                                ⚠ Frontend: {frontendVersion} / Backend: {backendVersion || '...'}
                            </div>
                        )}
                    </div>
                </div>
            </div>
        );
    }

    // Determine status display based on SimState from telemetry
    const simState = telemetry.SimState || 'disconnected';
    const statusInfo = {
        active: { label: 'Active', className: 'connected' },
        inactive: { label: 'Paused', className: 'paused' },
        disconnected: { label: 'Disconnected', className: 'error' },
    }[simState] || { label: 'Unknown', className: '' };

    const agl = Math.round(telemetry.AltitudeAGL);
    const msl = Math.round(telemetry.AltitudeMSL);

    const wdStats = stats?.providers?.wikidata || { api_success: 0, api_zero: 0, api_errors: 0, hit_rate: 0 };
    const wpStats = stats?.providers?.wikipedia || { api_success: 0, api_zero: 0, api_errors: 0, hit_rate: 0 };
    const geminiStats = stats?.providers?.gemini || { api_success: 0, api_zero: 0, api_errors: 0, hit_rate: 0 };
    const ttsStats = stats?.providers?.['edge-tts'] || { api_success: 0, api_zero: 0, api_errors: 0, hit_rate: 0 };

    const sysMem = stats?.system?.memory_alloc_mb || 0;
    const sysMemMax = stats?.system?.memory_max_mb || 0;
    const trackedCount = stats?.tracking?.active_pois || 0;

    return (
        <div className="hud-container">
            {/* Flight Data Flex Layout */}
            <div className="flex-container">
                {/* 1. HDG @ GS */}
                <div className="flex-card">
                    <div className="label">HDG @ GS</div>
                    <div className="value">
                        {Math.round(telemetry.Heading)}° <span className="unit">@</span> {Math.round(telemetry.GroundSpeed)} <span className="unit">kts</span>
                    </div>
                </div>

                {/* 2. ALTS */}
                <div className="flex-card">
                    <div className="label">ALTITUDE</div>
                    <div className="value">
                        {agl} <span className="unit">AGL</span>
                    </div>
                    <div className="sub-value" style={{ fontSize: '11px', color: '#666' }}>
                        {msl} <span className="unit">MSL</span>
                    </div>
                </div>

                {/* 3. COORDS */}
                <div className="flex-card" style={{ flex: '2 1 200px' }}> {/* Give coords more width pref */}
                    <div className="label">POSITION</div>
                    <div className="value" style={{ fontSize: '14px', fontFamily: 'monospace' }}>
                        {telemetry.Latitude.toFixed(4)}, {telemetry.Longitude.toFixed(4)}
                    </div>
                </div>

                {/* 4. VISIBILITY */}
                <div className="flex-card">
                    <div className="label">VISIBILITY</div>
                    <div className="value">
                        {(telemetry.AmbientVisibility !== undefined) ? (telemetry.AmbientVisibility / 1000).toFixed(1) : '-'} <span className="unit">km</span>
                    </div>
                    <div className="sub-value" style={{ fontSize: '10px', color: telemetry.AmbientInCloud ? '#d32f2f' : '#666' }}>
                        In Cloud: {telemetry.AmbientInCloud ? 'YES' : 'NO'}
                    </div>
                </div>

                {/* 5. FLIGHT STAGE & CONNECTION */}
                <div className="flex-card" style={{ flex: '1 1 80px', alignItems: 'center', justifyContent: 'center' }}>
                    {/* Combined Stage and Connection Status */}
                    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '4px' }}>
                        {simState !== 'disconnected' && (
                            <div className={`flight-stage ${telemetry.IsOnGround ? '' : 'active'}`} style={{ marginBottom: '4px' }}>
                                {telemetry.FlightStage || (telemetry.IsOnGround ? 'GROUND' : 'AIR')}
                            </div>
                        )}
                        <div className="status-pill" style={{ padding: '2px 6px', fontSize: '9px', background: 'transparent', border: 'none' }}>
                            <span className={`status-dot ${statusInfo.className}`} style={{ width: '6px', height: '6px' }}></span>
                            <span style={{ color: statusInfo.className === 'connected' ? '#4caf50' : '#888' }}>{statusInfo.label}</span>
                        </div>
                    </div>
                </div>
            </div>


            {/* Statistics Flex Layout */}
            <div className="flex-container">
                {/* System Stats */}
                <div className="flex-card stat-card">
                    <div className="value" style={{ fontSize: '11px', display: 'flex', flexDirection: 'column', gap: '2px', textAlign: 'left', lineHeight: '1.2' }}>
                        <div><span className="unit">POI (vis) </span> {displayedCount}</div>
                        <div><span className="unit">tracked </span> {trackedCount}</div>
                        <div><span className="unit">mem </span> {sysMem} / {sysMemMax} MB</div>
                    </div>
                </div>

                {/* Wikidata */}
                <div className="flex-card stat-card">
                    <div className="label">WIKIDATA</div>
                    <div className="value">
                        <span className="stat-success">{wdStats.api_success}</span>
                        <span className="stat-neutral"> / </span>
                        <span className="stat-neutral">{wdStats.api_zero}</span>
                        <span className="stat-neutral"> / </span>
                        <span className="stat-error">{wdStats.api_errors}</span>
                    </div>
                    <span className="stat-neutral" style={{ fontSize: '10px' }}>{wdStats.hit_rate}% Hit</span>
                </div>

                {/* Wikipedia */}
                <div className="flex-card stat-card">
                    <div className="label">WIKIPEDIA</div>
                    <div className="value">
                        <span className="stat-success">{wpStats.api_success}</span>
                        <span className="stat-neutral"> / </span>
                        <span className="stat-error">{wpStats.api_errors}</span>
                    </div>
                    <span className="stat-neutral" style={{ fontSize: '10px' }}>{wpStats.hit_rate}% Hit</span>
                </div>

                {/* Gemini */}
                <div className="flex-card stat-card">
                    <div className="label">GEMINI</div>
                    <div className="value">
                        <span className="stat-success">{geminiStats.api_success}</span>
                        <span className="stat-neutral"> / </span>
                        <span className="stat-error">{geminiStats.api_errors}</span>
                    </div>
                </div>

                {/* Edge TTS */}
                <div className="flex-card stat-card">
                    <div className="label">EDGE TTS</div>
                    <div className="value">
                        <span className="stat-success">{ttsStats.api_success}</span>
                        <span className="stat-neutral"> / </span>
                        <span className="stat-error">{ttsStats.api_errors}</span>
                    </div>
                </div>
            </div>


            {/* CONFIGURATION */}
            <div className="hud-card col-layout" style={{ gap: configOpen ? '12px' : '0' }}>
                <div
                    className="label interactive"
                    onClick={() => setConfigOpen(!configOpen)}
                    style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', width: '100%', userSelect: 'none' }}
                >
                    <span style={{ marginRight: 'auto' }}>CONFIGURATION</span>
                    <span style={{ transform: configOpen ? 'rotate(180deg)' : 'rotate(0deg)', transition: 'transform 0.2s' }}>▼</span>
                </div>

                {configOpen && (
                    <div className="config-group">
                        <div className="config-label">SIMULATION SOURCE</div>
                        <div className="radio-group">
                            <label className="radio-label">
                                <input
                                    type="radio"
                                    name="sim-source"
                                    checked={simSource === 'mock'}
                                    onChange={() => handleSimSourceChange('mock')}
                                /> Mock Sim
                            </label>
                            <label className="radio-label">
                                <input
                                    type="radio"
                                    name="sim-source"
                                    checked={simSource === 'simconnect'}
                                    onChange={() => handleSimSourceChange('simconnect')}
                                /> SimConnect
                            </label>
                        </div>
                        <div className="config-note" style={{ fontSize: '0.75rem', opacity: 0.7, marginTop: '4px' }}>
                            Restart required after changing source
                        </div>

                        <div className="config-label" style={{ marginTop: '16px' }}>UNITS</div>
                        <div className="radio-group">
                            <label className="radio-label">
                                <input
                                    type="radio"
                                    name="units"
                                    checked={units === 'km'}
                                    onChange={() => onUnitsChange('km')}
                                /> Kilometers (km)
                            </label>
                            <label className="radio-label">
                                <input
                                    type="radio"
                                    name="units"
                                    checked={units === 'nm'}
                                    onChange={() => onUnitsChange('nm')}
                                /> Nautical Miles (nm)
                            </label>
                        </div>

                        <div className="config-label" style={{ marginTop: '16px' }}>DEBUG LAYERS</div>
                        <div className="radio-group">
                            <label className="radio-label">
                                <input
                                    type="checkbox"
                                    checked={showCacheLayer}
                                    onChange={(e) => onCacheLayerChange(e.target.checked)}
                                /> Show Cache Layer
                            </label>
                            <label className="radio-label">
                                <input
                                    type="checkbox"
                                    checked={showVisibilityLayer}
                                    onChange={(e) => onVisibilityLayerChange(e.target.checked)}
                                /> Show Visibility Overlay
                            </label>
                        </div>

                        <div className="config-label" style={{ marginTop: '16px' }}>POI SCORE THRESHOLD</div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginTop: '4px' }}>
                            <input
                                type="range"
                                min="-10"
                                max="10"
                                step="0.5"
                                value={minPoiScore}
                                onChange={(e) => onMinPoiScoreChange(parseFloat(e.target.value))}
                                style={{ flex: 1 }}
                            />
                            <span style={{ fontSize: '12px', minWidth: '24px', textAlign: 'right' }}>{minPoiScore.toFixed(1)}</span>
                        </div>
                        <div className="config-note" style={{ fontSize: '0.75rem', opacity: 0.7, marginTop: '4px' }}>
                            Show POIs with score higher than this value
                        </div>

                        <div className="config-label" style={{ marginTop: '16px' }}>AUDIO VOLUME</div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginTop: '4px' }}>
                            <input
                                type="range"
                                min="0"
                                max="1"
                                step="0.05"
                                value={volume}
                                onChange={(e) => handleVolumeChange(parseFloat(e.target.value))}
                                style={{ flex: 1 }}
                            />
                            <span style={{ fontSize: '12px', minWidth: '24px', textAlign: 'right' }}>{(volume * 100).toFixed(0)}%</span>
                        </div>

                        <div className="config-label" style={{ marginTop: '16px', color: '#ff4444' }}>DANGER ZONE</div>
                        <button
                            onClick={() => { if (confirm('Are you sure you want to SHUTDOWN the server?')) fetch('/api/shutdown', { method: 'POST' }) }}
                            style={{
                                marginTop: '8px',
                                padding: '6px 12px',
                                background: 'rgba(211, 47, 47, 0.1)',
                                color: '#ff4444',
                                border: '1px solid #d32f2f',
                                borderRadius: '4px',
                                cursor: 'pointer',
                                fontSize: '10px',
                                fontWeight: 'bold',
                                width: '100%',
                                transition: 'all 0.2s'
                            }}
                            onMouseOver={(e) => { e.currentTarget.style.background = '#d32f2f'; e.currentTarget.style.color = 'white'; }}
                            onMouseOut={(e) => { e.currentTarget.style.background = 'rgba(211, 47, 47, 0.1)'; e.currentTarget.style.color = '#ff4444'; }}
                        >
                            SHUTDOWN SERVER
                        </button>
                    </div>
                )}
            </div>

            <div className="hud-footer">
                <div className="hud-card footer">
                    {versionMatch ? (
                        <div className="version-info clean">{frontendVersion}</div>
                    ) : (
                        <div className="version-info warning">
                            ⚠ Frontend: {frontendVersion} / Backend: {backendVersion || '...'}
                        </div>
                    )}
                </div>
            </div>
        </div >
    );
};
