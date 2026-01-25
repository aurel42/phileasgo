import { useEffect, useState, useRef } from 'react';
import type { Telemetry } from '../types/telemetry';
import { useNarrator } from '../hooks/useNarrator';
import packageJson from '../../package.json';

interface InfoPanelProps {
    telemetry?: Telemetry;
    status: 'pending' | 'error' | 'success';
    isRetrying?: boolean;
    displayedCount: number;
    filterMode?: string;
    narrationFrequency?: number;
    textLength?: number;
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


export const InfoPanel = ({
    telemetry, status, isRetrying,
    displayedCount,
    filterMode, narrationFrequency, textLength
}: InfoPanelProps) => {

    const [backendVersion, setBackendVersion] = useState<string | null>(null);
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
            {/* Removed CONFIGURATION section - moved to SettingsPanel */}

            <div className="hud-footer">
                <div className="hud-card footer" style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }}>
                    {versionMatch ? (
                        <div className="version-info clean">{frontendVersion}</div>
                    ) : (
                        <div className="version-info warning">
                            ⚠ Frontend: {frontendVersion} / Backend: {backendVersion || '...'}
                        </div>
                    )}

                    {/* CONFIG PILL */}
                    <a href="#/settings" className="config-pill" style={{ textDecoration: 'none', color: 'inherit' }}>
                        <div className="config-pill-item">
                            <span className="config-mode-icon">{filterMode === 'adaptive' ? '⚡' : '🎯'}</span>
                        </div>
                        <div className="config-pill-item">
                            <span className="text-grey" style={{ fontSize: '9px', fontWeight: 700 }}>FRQ</span>
                            <div className="pip-container">
                                {[1, 2, 3, 4, 5].map(v => (
                                    <div key={v} className={`pip ${(narrationFrequency || 0) >= v ? 'active' : ''} ${(narrationFrequency || 0) >= v && v > 3 ? 'high' : ''}`} />
                                ))}
                            </div>
                        </div>
                        <div className="config-pill-item">
                            <span className="text-grey" style={{ fontSize: '9px', fontWeight: 700 }}>LEN</span>
                            <div className="pip-container">
                                {[1, 2, 3, 4, 5].map(v => (
                                    <div key={v} className={`pip ${(textLength || 0) >= v ? 'active' : ''} ${(textLength || 0) >= v && v > 4 ? 'high' : ''}`} />
                                ))}
                            </div>
                        </div>
                    </a>
                </div>
            </div>
        </div >
    );
};
