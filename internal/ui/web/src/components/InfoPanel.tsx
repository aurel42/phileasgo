import { useEffect, useState, useRef } from 'react';
import type { Telemetry } from '../types/telemetry';
import { useNarrator } from '../hooks/useNarrator';
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
    filterMode: string;
    onFilterModeChange: (mode: string) => void;
    targetPoiCount: number;
    onTargetPoiCountChange: (count: number) => void;
    narrationFrequency: number;
    onNarrationFrequencyChange: (freq: number) => void;
    textLength: number;
    onTextLengthChange: (length: number) => void;
    streamingMode: boolean;
    onStreamingModeChange: (streaming: boolean) => void;
    isConfigOpen: boolean;
    onConfigOpenChange: (isOpen: boolean) => void;
}

interface Geography {
    city: string;
    region?: string;
    country: string;
    city_region?: string;
    city_country?: string;
    country_code?: string;
    city_country_code?: string;
}

const ConfigPills = ({ mode, frequency, length, simState }: { mode: string, frequency: number, length: number, simState: string }) => {
    const statusInfo = {
        active: { icon: '🟢', color: '#4caf50' },
        inactive: { icon: '🟠', color: '#ff9800' },
        disconnected: { icon: '🔴', color: '#f44336' },
    }[simState] || { icon: '⚪', color: '#888' };

    return (
        <div className="config-pill" style={{ background: 'transparent', border: 'none', padding: 0 }}>
            <div className="config-pill-item" title={`Sim Status: ${simState}`}>
                <span style={{ color: statusInfo.color, fontSize: '10px' }}>{statusInfo.icon}</span>
            </div>
            <div className="config-pill-item">
                <span className="config-mode-icon">{mode === 'adaptive' ? '⚡' : '🎯'}</span>
                <span style={{ color: '#888' }}>{mode === 'adaptive' ? 'ADAPTIVE' : 'FIXED'}</span>
            </div>
            <div className="config-pill-item">
                <span style={{ color: '#666' }}>FRQ</span>
                <div className="pip-container">
                    {[1, 2, 3, 4, 5].map(v => (
                        <div key={v} className={`pip ${v <= frequency ? 'active' : ''} ${v > 3 && v <= frequency ? 'high' : ''}`} />
                    ))}
                </div>
            </div>
            <div className="config-pill-item">
                <span style={{ color: '#666' }}>LEN</span>
                <div className="pip-container">
                    {[1, 2, 3, 4, 5].map(v => (
                        <div key={v} className={`pip ${v <= length ? 'active' : ''} ${v > 4 && v <= length ? 'high' : ''}`} />
                    ))}
                </div>
            </div>
        </div>
    );
};

export const InfoPanel = ({
    telemetry, status, isRetrying, units, onUnitsChange,
    showCacheLayer, onCacheLayerChange,
    showVisibilityLayer, onVisibilityLayerChange,
    displayedCount,
    minPoiScore,
    onMinPoiScoreChange,
    filterMode,
    onFilterModeChange,
    targetPoiCount,
    onTargetPoiCountChange,
    narrationFrequency,
    onNarrationFrequencyChange,
    textLength,
    onTextLengthChange,
    streamingMode,
    onStreamingModeChange,
    isConfigOpen,
    onConfigOpenChange
}: InfoPanelProps) => {

    const [backendVersion, setBackendVersion] = useState<string | null>(null);
    const [simSource, setSimSource] = useState<string>('mock');
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const [stats, setStats] = useState<any>(null);
    const [location, setLocation] = useState<Geography | null>(null);
    const { status: narratorStatus } = useNarrator();

    // Use ref to access latest telemetry in interval without resetting it
    const telemetryRef = useRef(telemetry);
    useEffect(() => { telemetryRef.current = telemetry; }, [telemetry]);

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

        const fetchLocation = () => {
            const t = telemetryRef.current;
            if (!t) return;
            fetch(`/api/geography?lat=${t.Latitude}&lon=${t.Longitude}`)
                .then(r => r.json())
                .then(data => setLocation(data))
                .catch(() => { });
        };

        // Initial fetch
        fetchVersion();
        fetchStats();
        fetchLocation();

        // Then poll
        const interval = setInterval(() => {
            fetchVersion();
            fetchStats();
        }, 5000);

        const locInterval = setInterval(fetchLocation, 10000);

        return () => {
            clearInterval(interval);
            clearInterval(locInterval);
        };
    }, []);

    useEffect(() => {
        fetch('/api/config')
            .then(r => r.json())
            .then(data => {
                setSimSource(data.sim_source || 'mock');
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


    const frontendVersion = `v${packageJson.version}`;
    // Treat null (loading) as a match to prevent flashing orange on load
    const versionMatch = !backendVersion || backendVersion === frontendVersion;

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

    const agl = Math.round(telemetry.AltitudeAGL);
    const msl = Math.round(telemetry.AltitudeMSL);

    const sysMem = stats?.system?.memory_alloc_mb || 0;
    const sysMemMax = stats?.system?.memory_max_mb || 0;
    const trackedCount = stats?.tracking?.active_pois || 0;

    return (
        <div className="hud-container">
            {/* Screenshot Display */}
            {narratorStatus?.current_image_path && (
                <div className="hud-card" style={{ marginBottom: '10px', padding: '0', overflow: 'hidden', position: 'relative' }}>
                    <div style={{
                        position: 'absolute',
                        top: '5px',
                        left: '5px',
                        background: 'rgba(0,0,0,0.7)',
                        color: 'white',
                        padding: '2px 6px',
                        borderRadius: '4px',
                        fontSize: '10px',
                        fontWeight: 'bold',
                        zIndex: 10
                    }}>
                        SCREENSHOT NARRATION
                    </div>
                    <img
                        src={`/api/images/serve?path=${encodeURIComponent(narratorStatus.current_image_path)}`}
                        alt="Screenshot"
                        style={{ width: '100%', height: 'auto', display: 'block' }}
                    />
                </div>
            )}

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
                    {telemetry.ValleyAltitude !== undefined && (
                        <div className="sub-value" style={{ fontSize: '11px', color: '#888', marginTop: '1px' }}>
                            {Math.round(telemetry.AltitudeMSL - (telemetry.ValleyAltitude * 3.28084))} <span className="unit">VAL</span>
                        </div>
                    )}
                </div>

                {/* 3. COORDS */}
                <div className="flex-card" style={{ flex: '2 1 200px', position: 'relative' }}> {/* Give coords more width pref */}
                    <div className="label">POSITION</div>

                    {/* Flight Stage Pill (Absolute positioned in corner) */}
                    {simState !== 'disconnected' && (
                        <div className={`flight-stage ${telemetry.IsOnGround ? '' : 'active'}`} style={{ position: 'absolute', top: '8px', right: '8px', fontSize: '9px', padding: '2px 6px' }}>
                            {telemetry.FlightStage || (telemetry.IsOnGround ? 'GROUND' : 'AIR')}
                        </div>
                    )}


                    {(location?.city || location?.country) && (
                        <>
                            <div className="value" style={{ fontSize: '16px', color: '#fff', fontFamily: 'Inter, sans-serif', fontWeight: 600, marginTop: '4px' }}>
                                {location.city ? (
                                    location.city === 'Unknown' ? (
                                        <span>Far from civilization</span>
                                    ) : (
                                        <>
                                            <span style={{ color: '#ddd', fontWeight: 400, marginRight: '6px', fontSize: '14px' }}>near</span>
                                            {location.city}
                                        </>
                                    )
                                ) : (
                                    <span>{location.country}</span>
                                )}
                            </div>
                            <div style={{ color: '#eee', fontSize: '14px', marginTop: '2px', fontFamily: 'Inter, sans-serif' }}>
                                {location.city_country_code && location.country_code && location.city_country_code !== location.country_code ? (
                                    <>
                                        <div>{location.city_region ? `${location.city_region}, ` : ''}{location.city_country}</div>
                                        <div style={{ color: '#4a9eff', fontWeight: 500, marginTop: '2px' }}>in {location.country}</div>
                                    </>
                                ) : (
                                    <>{location.region ? `${location.region}, ` : ''}{location.city ? location.country : (location.region ? '' : '')}</>
                                )}
                            </div>
                        </>
                    )}
                    <div className="value" style={{ fontSize: '13px', fontFamily: 'monospace', color: '#ccc', marginTop: location?.city ? '8px' : '0' }}>
                        {telemetry.Latitude.toFixed(4)}, {telemetry.Longitude.toFixed(4)}
                    </div>
                    {telemetry.APStatus && (
                        <div className="sub-value" style={{ fontSize: '11px', fontFamily: 'monospace', color: '#4caf50', marginTop: '4px' }}>
                            {telemetry.APStatus}
                        </div>
                    )}
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

                {/* Dynamic API Stats */}
                {stats?.providers && Object.entries(stats.providers)
                    // eslint-disable-next-line @typescript-eslint/no-explicit-any
                    .sort(([keyA], [keyB]) => keyA.localeCompare(keyB))
                    // eslint-disable-next-line @typescript-eslint/no-explicit-any
                    .map(([key, data]: [string, any]) => {
                        // Filter empty stats (0 success AND 0 errors)
                        if (data.api_success === 0 && data.api_errors === 0) return null;

                        const label = key.toUpperCase().replace('-', ' ');
                        const hasCacheActivity = (data.cache_hits || 0) + (data.cache_misses || 0) > 0;
                        const hitRate = hasCacheActivity && data.hit_rate !== undefined ? `${data.hit_rate}% Hit` : null;

                        return (
                            <div className="flex-card stat-card" key={key}>
                                <div className="label">
                                    {label}
                                    {data.free_tier === false && <span style={{ marginLeft: '4px', fontSize: '10px' }}>💵</span>}
                                </div>
                                <div className="value">
                                    <span className="stat-success">{data.api_success}</span>
                                    <span className="stat-neutral"> / </span>
                                    {/* Only show 'zero' results if relevant (e.g. not present for all APIs) */}
                                    {data.api_zero !== undefined && (
                                        <>
                                            <span className="stat-neutral">{data.api_zero}</span>
                                            <span className="stat-neutral"> / </span>
                                        </>
                                    )}
                                    <span className="stat-error">{data.api_errors}</span>
                                </div>
                                {hitRate && <span className="stat-neutral" style={{ fontSize: '10px' }}>{hitRate}</span>}
                            </div>
                        );
                    })}
            </div>


            {/* CONFIGURATION */}
            <div className="hud-card col-layout" style={{ gap: isConfigOpen ? '12px' : '0' }}>
                <div
                    className="label interactive"
                    onClick={() => onConfigOpenChange(!isConfigOpen)}
                    style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', width: '100%', userSelect: 'none' }}
                >
                    <span style={{ marginRight: 'auto' }}>CONFIGURATION</span>
                    <span style={{ transform: isConfigOpen ? 'rotate(180deg)' : 'rotate(0deg)', transition: 'transform 0.2s' }}>▼</span>
                </div>

                {isConfigOpen && (
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

                        <div className="config-label" style={{ marginTop: '16px' }}>POI FILTERING MODE</div>
                        <div className="radio-group">
                            <label className="radio-label">
                                <input
                                    type="radio"
                                    name="filter-mode"
                                    checked={filterMode === 'fixed'}
                                    onChange={() => onFilterModeChange('fixed')}
                                /> Fixed Score
                            </label>
                            <label className="radio-label">
                                <input
                                    type="radio"
                                    name="filter-mode"
                                    checked={filterMode === 'adaptive'}
                                    onChange={() => onFilterModeChange('adaptive')}
                                /> Adaptive (Target Count)
                            </label>
                        </div>

                        {filterMode === 'fixed' ? (
                            <>
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
                            </>
                        ) : (
                            <>
                                <div className="config-label" style={{ marginTop: '16px' }}>TARGET POI COUNT</div>
                                <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginTop: '4px' }}>
                                    <input
                                        type="range"
                                        min="1"
                                        max="50"
                                        step="1"
                                        value={targetPoiCount}
                                        onChange={(e) => onTargetPoiCountChange(parseInt(e.target.value))}
                                        style={{ flex: 1 }}
                                    />
                                    <span style={{ fontSize: '12px', minWidth: '24px', textAlign: 'right' }}>{targetPoiCount}</span>
                                </div>
                                <div className="config-note" style={{ fontSize: '0.75rem', opacity: 0.7, marginTop: '4px' }}>
                                    Dynamic visibility threshold to show approximately {targetPoiCount} POIs
                                </div>
                            </>
                        )}

                        <div className="config-label" style={{ marginTop: '16px' }}>NARRATION FREQUENCY</div>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', marginTop: '4px' }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                                <input
                                    type="range"
                                    min="1"
                                    max="5"
                                    step="1"
                                    value={narrationFrequency}
                                    onChange={(e) => onNarrationFrequencyChange(parseInt(e.target.value))}
                                    style={{ flex: 1 }}
                                />
                                <span style={{ fontSize: '12px', minWidth: '12px', textAlign: 'right', fontWeight: 'bold' }}>{narrationFrequency}</span>
                            </div>
                            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '9px', opacity: 0.7 }}>
                                <span>Rarely</span>
                                <span>Normal</span>
                                <span>Active</span>
                                <span>Busy</span>
                                <span>Constant</span>
                            </div>
                        </div>

                        <div className="config-label" style={{ marginTop: '16px' }}>TEXT LENGTH</div>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', marginTop: '4px' }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                                <input
                                    type="range"
                                    min="1"
                                    max="5"
                                    step="1"
                                    value={textLength}
                                    onChange={(e) => onTextLengthChange(parseInt(e.target.value))}
                                    style={{ flex: 1 }}
                                />
                                <span style={{ fontSize: '12px', minWidth: '12px', textAlign: 'right', fontWeight: 'bold' }}>{textLength}</span>
                            </div>
                            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '9px', opacity: 0.7 }}>
                                <span>Shortest</span>
                                <span>Shorter</span>
                                <span>Normal</span>
                                <span>Longer</span>
                                <span>Longest</span>
                            </div>
                        </div>

                        <div className="config-label" style={{ marginTop: '16px' }}>STREAMING MODE</div>
                        <div className="radio-group">
                            <label className="radio-label">
                                <input
                                    type="checkbox"
                                    checked={streamingMode}
                                    onChange={(e) => onStreamingModeChange(e.target.checked)}
                                /> Keep updating in background
                            </label>
                        </div>
                        <div className="config-note" style={{ fontSize: '0.75rem', opacity: 0.7, marginTop: '4px' }}>
                            Enable for OBS/streaming to prevent UI pause when alt-tabbing
                        </div>

                        <div className="config-label" style={{ marginTop: '16px', color: '#ff4444' }}>DANGER ZONE</div>
                        <button
                            onClick={() => {
                                if (confirm('Are you sure you want to RESET history for POIs within 100km? This cannot be undone.')) {
                                    fetch('/api/pois/reset-last-played', {
                                        method: 'POST',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({
                                            lat: telemetry?.Latitude || 0,
                                            lon: telemetry?.Longitude || 0
                                        })
                                    }).then(res => {
                                        if (res.ok) alert('History reset successfully. Marker colors will update shortly.');
                                        else alert('Failed to reset history.');
                                    }).catch(e => console.error(e));
                                }
                            }}
                            style={{
                                marginTop: '4px',
                                padding: '6px 12px',
                                background: 'rgba(255, 152, 0, 0.1)',
                                color: '#f57c00',
                                border: '1px solid #f57c00',
                                borderRadius: '4px',
                                cursor: 'pointer',
                                fontSize: '10px',
                                fontWeight: 'bold',
                                width: '100%',
                                transition: 'all 0.2s'
                            }}
                            onMouseOver={(e) => { e.currentTarget.style.background = '#f57c00'; e.currentTarget.style.color = 'white'; }}
                            onMouseOut={(e) => { e.currentTarget.style.background = 'rgba(255, 152, 0, 0.1)'; e.currentTarget.style.color = '#f57c00'; }}
                        >
                            RESET HISTORY (100km)
                        </button>
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
                <div className="hud-card footer" style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }}>
                    {versionMatch ? (
                        <div className="version-info clean">{frontendVersion}</div>
                    ) : (
                        <div className="version-info warning">
                            ⚠ Frontend: {frontendVersion} / Backend: {backendVersion || '...'}
                        </div>
                    )}
                    <ConfigPills mode={filterMode} frequency={narrationFrequency} length={textLength} simState={simState} />
                </div>
            </div>
        </div >
    );
};
