import { useEffect, useState, useRef } from 'react';
import type { Telemetry } from '../types/telemetry';
import packageJson from '../../package.json';

interface InfoPanelProps {
    telemetry?: Telemetry;
    status: 'pending' | 'error' | 'success';
    isRetrying?: boolean;
    nonBlueCount: number;
    blueCount: number;
    minPoiScore?: number;
    targetCount?: number;
    filterMode?: string;
    narrationFrequency?: number;
    textLength?: number;
    onSettingsClick?: () => void;
}

import { useGeography } from '../hooks/useGeography';


export const InfoPanel = ({
    telemetry, status, isRetrying,
    nonBlueCount, blueCount,
    minPoiScore, targetCount, filterMode,
    narrationFrequency, textLength,
    onSettingsClick
}: InfoPanelProps) => {

    const [backendVersion, setBackendVersion] = useState<string | null>(null);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const [stats, setStats] = useState<any>(null);
    const [isDiagnosticsOpen, setIsDiagnosticsOpen] = useState(false);
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
    const simStateDisplay = telemetry.SimState === 'disconnected' ? 'disconnected'
        : telemetry.SimState === 'inactive' ? 'paused'
            : 'active';

    const agl = Math.round(telemetry.AltitudeAGL);
    const msl = Math.round(telemetry.AltitudeMSL);

    const trackedCount = stats?.tracking?.active_pois || 0;
    const diagnostics = stats?.diagnostics || [];
    const goMem = stats?.go_mem;

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
            <div className="stats-container">
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

                    // Pre-filter active providers
                    const activeLLMProviders = llmProviders.filter(([_, data]) => data.api_success > 0 || data.api_errors > 0);
                    const activeDataProviders = dataProviders.filter(([_, data]) => data.api_success > 0 || data.api_errors > 0);

                    const renderProvider = (key: string, data: any) => {
                        const label = key.toUpperCase().replace('-', ' ');
                        const hasCacheActivity = (data.cache_hits || 0) + (data.cache_misses || 0) > 0;
                        const hitRate = hasCacheActivity && data.hit_rate !== undefined ? `${data.hit_rate}% Hit` : null;

                        return (
                            <div className="flex-card stat-card" key={key}>
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
                        );
                    };

                    const renderIndicator = (key: string) => (
                        <div className="fallback-flow-indicator" key={`arrow-${key}`}>
                            <div className="fallback-triangle" />
                        </div>
                    );

                    return (
                        <>
                            {activeLLMProviders.map(([key, data], idx) => (
                                <div key={key} style={{ display: 'contents' }}>
                                    {renderProvider(key, data)}
                                    {idx < activeLLMProviders.length - 1 && renderIndicator(key)}
                                </div>
                            ))}
                            {activeDataProviders.map(([key, data]) => renderProvider(key, data))}
                        </>
                    );
                })()}
            </div >


            {/* CONFIGURATION */}
            {/* Removed CONFIGURATION section - moved to SettingsPanel */}

            <div className="hud-footer">
                <div className="hud-card footer" onClick={onSettingsClick} style={{
                    flexDirection: 'row',
                    flexWrap: 'wrap',
                    gap: '12px 24px',
                    padding: '10px 16px',
                    cursor: 'pointer',
                    alignItems: 'center',
                    justifyContent: 'flex-start'
                }}>
                    {/* 1. SIM & POI STATS */}
                    <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                        <div style={{ display: 'flex', alignItems: 'center' }}>
                            <div className="status-dot" style={{
                                width: '8px',
                                height: '8px',
                                marginRight: '8px',
                                backgroundColor: simStateDisplay === 'disconnected' ? '#ef4444' : (simStateDisplay === 'paused' ? '#fbbf24' : '#22c55e')
                            }}></div>
                            <span className="role-label" style={{ textTransform: 'uppercase', letterSpacing: '1px', fontSize: '11px' }}>
                                SIM {simStateDisplay}
                            </span>
                        </div>

                        <div className="role-label" style={{ display: 'flex', gap: '12px', borderLeft: '1px solid rgba(255,255,255,0.1)', paddingLeft: '12px' }}>
                            <span>POI(vis) <span className="role-num-sm">{nonBlueCount}</span><span style={{ color: 'var(--muted)', fontSize: '0.6em', verticalAlign: 'middle', position: 'relative', top: '-1px', marginLeft: '4px', marginRight: '4px' }}>◆</span><span className="role-num-sm" style={{ color: '#3b82f6' }}>{blueCount}</span></span>
                            <span>POI(tracked) <span className="role-num-sm">{trackedCount}</span></span>
                        </div>
                    </div>

                    {/* 2. NARRATOR CONFIG */}
                    <div style={{ display: 'flex', gap: '16px', alignItems: 'center', borderLeft: '1px solid rgba(255,255,255,0.1)', paddingLeft: '24px' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                            <span className="role-label" style={{ color: 'var(--accent)' }}>{filterMode === 'adaptive' ? '⚡' : '🎯'}</span>
                            <span className="role-num-sm" style={{ color: 'var(--muted)' }}>
                                {filterMode === 'adaptive' ? targetCount : minPoiScore}
                            </span>
                        </div>

                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                            <span className="role-label" style={{ color: 'var(--muted)' }}>FRQ</span>
                            <div className="pip-container">
                                {[1, 2, 3, 4].map(v => (
                                    <div key={v} className={`pip ${(narrationFrequency || 0) >= v ? 'active' : ''} ${(narrationFrequency || 0) >= v && v > 2 ? 'high' : ''}`} />
                                ))}
                            </div>
                        </div>

                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                            <span className="role-label" style={{ color: 'var(--muted)' }}>LEN</span>
                            <div className="pip-container">
                                {[1, 2, 3, 4, 5].map(v => (
                                    <div key={v} className={`pip ${(textLength || 0) >= v ? 'active' : ''} ${(textLength || 0) >= v && v > 4 ? 'high' : ''}`} />
                                ))}
                            </div>
                        </div>
                    </div>

                    {/* 3. VERSION (Pushed right if space permits) */}
                    <div style={{ marginLeft: 'auto' }}>
                        {versionMatch ? (
                            <div className="role-num-sm" style={{ opacity: 0.5 }}>{frontendVersion}</div>
                        ) : (
                            <div className="role-num-sm" style={{ color: 'var(--error)' }}>
                                ⚠ {frontendVersion}
                            </div>
                        )}
                    </div>
                </div>

                {/* Diagnostics Table moved to bottom */}
                {diagnostics.length > 0 && (
                    <div className="flex-card" style={{ marginTop: '12px', padding: '12px 16px' }}>
                        <div
                            className="role-header"
                            style={{
                                marginBottom: isDiagnosticsOpen ? '8px' : '0',
                                display: 'flex',
                                justifyContent: 'space-between',
                                alignItems: 'center',
                                cursor: 'pointer'
                            }}
                            onClick={() => setIsDiagnosticsOpen(!isDiagnosticsOpen)}
                        >
                            <span>System Diagnostics</span>
                            <span style={{ fontSize: '10px', opacity: 0.5 }}>{isDiagnosticsOpen ? '▼' : '▶'}</span>
                        </div>
                        {isDiagnosticsOpen && (
                            <>
                                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                                    <thead>
                                        <tr style={{ textAlign: 'left' }}>
                                            <th className="role-label" style={{ paddingBottom: '4px' }}>Process</th>
                                            <th className="role-label" style={{ paddingBottom: '4px', textAlign: 'right' }}>Memory</th>
                                            <th className="role-label" style={{ paddingBottom: '4px', textAlign: 'right', opacity: 0.5 }}>(Max)</th>
                                            <th className="role-label" style={{ paddingBottom: '4px', textAlign: 'right' }}>CPU/s</th>
                                            <th className="role-label" style={{ paddingBottom: '4px', textAlign: 'right', opacity: 0.5 }}>(Max)</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {diagnostics.map((d: any) => (
                                            <tr key={d.name} style={{ borderTop: '1px solid rgba(255,255,255,0.05)' }}>
                                                <td className="role-label" style={{ padding: '4px 0', color: 'var(--text-color)' }}>{d.name}</td>
                                                <td className="role-num-sm" style={{ padding: '4px 0', textAlign: 'right' }}>{d.memory_mb}MB</td>
                                                <td className="role-num-sm" style={{ padding: '4px 0', textAlign: 'right', opacity: 0.5 }}>{d.memory_max_mb}MB</td>
                                                <td className="role-num-sm" style={{ padding: '4px 0', textAlign: 'right' }}>{d.cpu_sec.toFixed(2)}</td>
                                                <td className="role-num-sm" style={{ padding: '4px 0', textAlign: 'right', opacity: 0.5 }}>{d.cpu_max_sec.toFixed(2)}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>

                                {goMem && (
                                    <>
                                        <div className="role-header" style={{ marginTop: '12px', marginBottom: '6px' }}>Go Runtime Memory</div>
                                        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                                            <thead>
                                                <tr style={{ textAlign: 'left' }}>
                                                    <th className="role-label" style={{ paddingBottom: '4px' }}>Region</th>
                                                    <th className="role-label" style={{ paddingBottom: '4px', textAlign: 'right' }}>MB</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {[
                                                    { label: 'Heap (live)', value: goMem.heap_alloc_mb },
                                                    { label: 'Heap (in-use spans)', value: goMem.heap_inuse_mb },
                                                    { label: 'Heap (idle spans)', value: goMem.heap_idle_mb },
                                                    { label: 'Heap (from OS)', value: goMem.heap_sys_mb, bold: true },
                                                    { label: 'Stack', value: goMem.stack_inuse_mb },
                                                    { label: 'GC metadata', value: goMem.gc_sys_mb },
                                                    { label: 'Runtime other', value: goMem.other_sys_mb + goMem.mspan_inuse_mb + goMem.mcache_inuse_mb },
                                                    { label: 'Total from OS', value: goMem.total_sys_mb, bold: true },
                                                ].map((row) => (
                                                    <tr key={row.label} style={{ borderTop: '1px solid rgba(255,255,255,0.05)' }}>
                                                        <td className="role-label" style={{ padding: '4px 0', color: 'var(--text-color)', fontWeight: row.bold ? 600 : 400 }}>{row.label}</td>
                                                        <td className="role-num-sm" style={{ padding: '4px 0', textAlign: 'right', fontWeight: row.bold ? 600 : 400 }}>{row.value.toFixed(1)}</td>
                                                    </tr>
                                                ))}
                                            </tbody>
                                        </table>
                                        <div style={{ marginTop: '6px', display: 'flex', gap: '16px' }}>
                                            <span className="role-label">Goroutines <span className="role-num-sm">{goMem.num_goroutine}</span></span>
                                            <span className="role-label">Heap objects <span className="role-num-sm">{goMem.heap_objects.toLocaleString()}</span></span>
                                            <span className="role-label">GC cycles <span className="role-num-sm">{goMem.num_gc}</span></span>
                                        </div>
                                    </>
                                )}
                            </>
                        )}
                    </div>
                )}
            </div>
        </div>
    );
};
