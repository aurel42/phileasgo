import React from 'react';
import type { Telemetry } from '../types/telemetry';
import type { BackendStats, BackendVersion } from '../hooks/useBackendInfo';
import packageJson from '../../package.json';

interface DashboardFooterProps {
    telemetry?: Telemetry;
    stats?: BackendStats;
    version?: BackendVersion;
    nonBlueCount: number;
    blueCount: number;
    minPoiScore?: number;
    targetCount?: number;
    filterMode?: string;
    narrationFrequency?: number;
    textLength?: number;
    onSettingsClick?: () => void;
}

export const DashboardFooter: React.FC<DashboardFooterProps> = ({
    telemetry, stats, version,
    nonBlueCount, blueCount,
    minPoiScore, targetCount, filterMode,
    narrationFrequency, textLength,
    onSettingsClick
}) => {
    const frontendVersion = `v${packageJson.version}`;
    const backendVersion = version?.version;
    const versionMatch = !backendVersion || backendVersion === frontendVersion;

    const simStateDisplay = !telemetry || telemetry.SimState === 'disconnected' ? 'disconnected'
        : telemetry.SimState === 'inactive' ? 'paused'
            : 'active';

    const trackedCount = stats?.tracking?.active_pois || 0;

    return (
        <div className="hud-footer" style={{ marginTop: 'auto' }}>
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
        </div>
    );
};
