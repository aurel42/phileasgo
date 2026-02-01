import { useEffect, useState, useRef } from 'react';
import type { Telemetry } from '../types/telemetry';
import packageJson from '../../package.json';

interface InfoPanelProps {
    telemetry?: Telemetry;
    status: 'pending' | 'error' | 'success';
    isRetrying?: boolean;
    nonBlueCount: number;
    blueCount: number;
}

import { useGeography } from '../hooks/useGeography';


export const InfoPanel = ({
    telemetry, status, isRetrying,
    nonBlueCount, blueCount
}: InfoPanelProps) => {

    const [backendVersion, setBackendVersion] = useState<string | null>(null);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const [stats, setStats] = useState<any>(null);
    const { location } = useGeography(telemetry);

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

        // Initial fetch
        fetchVersion();
        fetchStats();

        // Then poll
        const interval = setInterval(() => {
            fetchVersion();
            fetchStats();
        }, 5000);

        return () => {
            clearInterval(interval);
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
    const simPaused = telemetry.IsOnGround === false && telemetry.GroundSpeed < 1; // Heuristic (Airspeed missing from types, assume GroundSpeed proxies for now if airborn)
    const simStateDisplay = !telemetry ? 'disconnected' : (simPaused ? 'paused' : 'active');

    const agl = Math.round(telemetry.AltitudeAGL);
    const msl = Math.round(telemetry.AltitudeMSL);

    const sysMem = stats?.system?.memory_alloc_mb || 0;
    const sysMemMax = stats?.system?.memory_max_mb || 0;
    const trackedCount = stats?.tracking?.active_pois || 0;

    return (
        <div className="hud-container">

            {/* Flight Data Flex Layout */}
            <div className="flex-container">
                {/* 1. HDG @ GS */}
                <div className="flex-card">
                    <div className="role-header">HDG @ GS</div>
                    <div className="role-num-lg">
                        {telemetry.Valid ? Math.round(telemetry.Heading) + '°' : '—'} <span className="unit">@</span> {telemetry.Valid ? Math.round(telemetry.GroundSpeed) : '—'} <span className="unit">kts</span>
                    </div>
                    {telemetry.APStatus && (
                        <div className="role-num-sm" style={{ color: 'var(--success)', marginTop: '4px', fontSize: '14px' }}>
                            {telemetry.Valid ? telemetry.APStatus : 'nil'}
                        </div>
                    )}
                </div>

                {/* 2. ALTS - Using a Grid for Alignment */}
                <div className="flex-card">
                    <div className="role-header">ALTITUDE</div>
                    <div style={{ display: 'grid', gridTemplateColumns: 'min-content 1fr', columnGap: '8px', alignItems: 'baseline' }}>
                        <div className="role-num-lg" style={{ textAlign: 'right' }}>{telemetry.Valid ? agl : '—'}</div>
                        <div className="role-label">AGL</div>

                        <div className="role-num-sm" style={{ textAlign: 'right' }}>{telemetry.Valid ? msl : '—'}</div>
                        <div className="role-label">MSL</div>

                        {telemetry.ValleyAltitude !== undefined && (
                            <>
                                <div className="role-num-sm" style={{ textAlign: 'right', opacity: 0.7 }}>
                                    {telemetry.Valid ? Math.round(telemetry.AltitudeMSL - (telemetry.ValleyAltitude * 3.28084)) : '—'}
                                </div>
                                <div className="role-label" style={{ opacity: 0.7 }}>VAL</div>
                            </>
                        )}
                    </div>
                </div>

                {/* 3. COORDS */}
                <div className="flex-card" style={{ flex: '2 1 200px', position: 'relative' }}>
                    <div className="role-header">POSITION</div>

                    {simStateDisplay !== 'disconnected' && (
                        <div className={`flight-stage ${telemetry.IsOnGround ? '' : 'active'} role-btn`} style={{ position: 'absolute', top: '8px', right: '8px', padding: '2px 6px' }}>
                            {telemetry.FlightStage || (telemetry.IsOnGround ? 'GROUND' : 'AIR')}
                        </div>
                    )}

                    {(location?.city || location?.country) && (
                        <>
                            <div className="role-text-lg" style={{ marginTop: '4px' }}>
                                {location.city ? (
                                    location.city === 'Unknown' ? (
                                        <span>Far from civilization</span>
                                    ) : (
                                        <>
                                            <span className="role-label" style={{ marginRight: '6px' }}>near</span>
                                            {location.city}
                                        </>
                                    )
                                ) : (
                                    <span>{location.country}</span>
                                )}
                            </div>
                            <div className="role-text-sm" style={{ marginTop: '2px' }}>
                                {location.city_country_code && location.country_code && location.city_country_code !== location.country_code ? (
                                    <>
                                        <div>{location.city_region ? `${location.city_region}, ` : ''}{location.city_country}</div>
                                        <div style={{ color: 'var(--accent)', marginTop: '2px' }}>in {location.country}</div>
                                    </>
                                ) : (
                                    <>{location.region ? `${location.region}, ` : ''}{location.city ? location.country : (location.region ? '' : '')}</>
                                )}
                            </div>
                        </>
                    )}
                    <div className="role-num-sm" style={{ color: 'var(--muted)', marginTop: location?.city ? '8px' : '0' }}>
                        {telemetry.Valid ? `${telemetry.Latitude.toFixed(4)}, ${telemetry.Longitude.toFixed(4)}` : '—, —'}
                    </div>
                </div>
            </div>

            {/* Statistics Flex Layout (API stats) */}
            <div className="stats-grid">
                {(() => {
                    if (!stats?.providers) return null;

                    const fallbackOrder = stats.llm_fallback || [];
                    const providerEntries = Object.entries(stats.providers) as [string, any][];

                    // Identify LLM providers from fallback list
                    const llmProviders = providerEntries
                        .filter(([key]) => fallbackOrder.includes(key))
                        .sort(([keyA], [keyB]) => {
                            const idxA = fallbackOrder.indexOf(keyA);
                            const idxB = fallbackOrder.indexOf(keyB);
                            return idxA - idxB;
                        });

                    // Non-LLM providers (Data Services)
                    const dataProviders = providerEntries
                        .filter(([key]) => !fallbackOrder.includes(key))
                        .sort(([keyA], [keyB]) => keyA.localeCompare(keyB));

                    const renderProvider = (key: string, data: any, showIndicator: boolean = false) => {
                        if (data.api_success === 0 && data.api_errors === 0) return null;

                        const label = key.toUpperCase().replace('-', ' ');
                        const hasCacheActivity = (data.cache_hits || 0) + (data.cache_misses || 0) > 0;
                        const hitRate = hasCacheActivity && data.hit_rate !== undefined ? `${data.hit_rate}% Hit` : null;

                        return (
                            <div className="stat-card-container" key={key}>
                                <div className="flex-card stat-card">
                                    <div className="role-header">
                                        {label}
                                        {data.free_tier === false && <span>£</span>}
                                    </div>
                                    <div className="role-num-lg">
                                        <span style={{ color: 'var(--success)' }}>{data.api_success}</span>
                                        <span style={{ color: 'var(--muted)', fontSize: '0.6em', verticalAlign: 'middle', position: 'relative', top: '-1px' }}>◆</span>
                                        {data.api_zero !== undefined && (
                                            <>
                                                <span>{data.api_zero}</span>
                                                <span style={{ color: 'var(--muted)', fontSize: '0.6em', verticalAlign: 'middle', position: 'relative', top: '-1px' }}>◆</span>
                                            </>
                                        )}
                                        <span style={{ color: 'var(--error)' }}>{data.api_errors}</span>
                                    </div>
                                    {hitRate && <span className="role-label">{hitRate}</span>}
                                </div>
                                {showIndicator && (
                                    <div className="fallback-flow-indicator">
                                        <div className="fallback-triangle" />
                                    </div>
                                )}
                            </div>
                        );
                    };

                    return (
                        <>
                            {llmProviders.map(([key, data], idx) =>
                                renderProvider(key, data, idx < llmProviders.length - 1)
                            )}
                            {dataProviders.map(([key, data]) =>
                                renderProvider(key, data)
                            )}
                        </>
                    );
                })()}
            </div >


            {/* CONFIGURATION */}
            {/* Removed CONFIGURATION section - moved to SettingsPanel */}

            <div className="hud-footer">
                <div className="hud-card footer" style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', padding: '8px 16px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
                        <div className="role-label" style={{ display: 'flex', gap: '12px' }}>
                            <span>POI(vis) <span className="role-num-sm">{nonBlueCount}</span><span style={{ color: 'var(--muted)', fontSize: '0.6em', verticalAlign: 'middle', position: 'relative', top: '-1px', marginLeft: '4px', marginRight: '4px' }}>◆</span><span className="role-num-sm" style={{ color: '#3b82f6' }}>{blueCount}</span></span>
                            <span>POI(tracked) <span className="role-num-sm">{trackedCount}</span></span>
                            <span>MEM(rss) <span className="role-num-sm">{sysMem}MB</span></span>
                            <span>MEM(max) <span className="role-num-sm">{sysMemMax}MB</span></span>
                        </div>
                    </div>

                    {/* Version on the Right Border */}
                    {versionMatch ? (
                        <div className="role-num-sm" style={{ opacity: 0.5 }}>{frontendVersion}</div>
                    ) : (
                        <div className="role-num-sm" style={{ color: 'var(--error)' }}>
                            ⚠ {frontendVersion}
                        </div>
                    )}
                </div>
            </div>
        </div >
    );
};
