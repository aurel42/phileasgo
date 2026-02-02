import { useEffect, useState } from 'react';
import type { Telemetry } from '../types/telemetry';

interface SettingsPanelProps {
    telemetry?: Telemetry;
    units: 'km' | 'nm';
    onUnitsChange: (units: 'km' | 'nm') => void;
    showCacheLayer: boolean;
    onCacheLayerChange: (show: boolean) => void;
    showVisibilityLayer: boolean;
    onVisibilityLayerChange: (show: boolean) => void;
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
    isGui?: boolean;
    onBack?: () => void;
}

export const SettingsPanel = ({
    telemetry,
    units, onUnitsChange,
    showCacheLayer, onCacheLayerChange,
    showVisibilityLayer, onVisibilityLayerChange,
    minPoiScore, onMinPoiScoreChange,
    filterMode, onFilterModeChange,
    targetPoiCount, onTargetPoiCountChange,
    narrationFrequency, onNarrationFrequencyChange,
    textLength, onTextLengthChange,
    isGui, onBack
}: SettingsPanelProps) => {

    const [simSource, setSimSource] = useState<string>('mock');

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

    return (
        <div className="hud-container" style={{ padding: '24px', height: '100vh', boxSizing: 'border-box', overflowY: 'auto' }}>
            {!isGui && (
                <div style={{ marginBottom: '24px', display: 'flex', justifyContent: 'flex-start' }}>
                    <button
                        className="role-btn"
                        onClick={onBack}
                        style={{
                            padding: '8px 16px',
                            background: 'var(--card-bg)',
                            color: 'var(--accent)',
                            border: '3px double rgba(212, 175, 55, 0.5)',
                            borderRadius: '4px',
                            cursor: 'pointer',
                            fontFamily: 'var(--font-display)',
                            fontSize: '12px',
                            letterSpacing: '0.1em',
                            display: 'flex',
                            alignItems: 'center',
                            gap: '8px',
                            boxShadow: '0 4px 10px rgba(0,0,0,0.5)'
                        }}
                    >
                        <span>⚓</span> BACK TO NAVIGATOR
                    </button>
                </div>
            )}

            <div className="config-group" style={{ maxWidth: '600px', width: '100%' }}>
                <div className="role-header" style={{ fontSize: '14px', marginBottom: '8px' }}>SIMULATION SOURCE</div>
                <div className="radio-group">
                    <label className="role-text-sm" style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                        <input
                            type="radio"
                            name="sim-source"
                            checked={simSource === 'mock'}
                            onChange={() => handleSimSourceChange('mock')}
                        /> Mock Sim
                    </label>
                    <label className="role-text-sm" style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                        <input
                            type="radio"
                            name="sim-source"
                            checked={simSource === 'simconnect'}
                            onChange={() => handleSimSourceChange('simconnect')}
                        /> SimConnect
                    </label>
                </div>
                <div className="role-text-sm" style={{ fontSize: '12px', opacity: 0.7, marginTop: '4px' }}>
                    Restart required after changing source
                </div>

                <div className="role-header" style={{ fontSize: '14px', marginTop: '16px', marginBottom: '8px' }}>RANGE RING UNITS</div>
                <div className="radio-group">
                    <label className="role-text-sm" style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                        <input
                            type="radio"
                            name="units"
                            checked={units === 'km'}
                            onChange={() => onUnitsChange('km')}
                        /> Kilometers (km)
                    </label>
                    <label className="role-text-sm" style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                        <input
                            type="radio"
                            name="units"
                            checked={units === 'nm'}
                            onChange={() => onUnitsChange('nm')}
                        /> Nautical Miles (nm)
                    </label>
                </div>

                {/* Debug Layers Moved Down */}

                <div className="role-header" style={{ fontSize: '14px', marginTop: '16px', marginBottom: '8px' }}>POI FILTERING MODE</div>
                <div className="radio-group">
                    <label className="role-text-sm" style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                        <input
                            type="radio"
                            name="filter-mode"
                            checked={filterMode === 'fixed'}
                            onChange={() => onFilterModeChange('fixed')}
                        /> Fixed Score
                    </label>
                    <label className="role-text-sm" style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
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
                        <div className="role-header" style={{ fontSize: '14px', marginTop: '16px', marginBottom: '8px' }}>POI SCORE THRESHOLD</div>
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
                            <span className="role-num-sm" style={{ minWidth: '24px', textAlign: 'right' }}>{minPoiScore.toFixed(1)}</span>
                        </div>
                        <div className="role-text-sm" style={{ fontSize: '12px', opacity: 0.7, marginTop: '4px' }}>
                            Show POIs with score higher than this value
                        </div>
                    </>
                ) : (
                    <>
                        <div className="role-header" style={{ fontSize: '14px', marginTop: '16px', marginBottom: '8px' }}>TARGET POI COUNT</div>
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
                            <span className="role-num-sm" style={{ minWidth: '24px', textAlign: 'right' }}>{targetPoiCount}</span>
                        </div>
                        <div className="role-text-sm" style={{ fontSize: '12px', opacity: 0.7, marginTop: '4px' }}>
                            Dynamic visibility threshold to show approximately {targetPoiCount} POIs
                        </div>
                    </>
                )}

                <div className="role-header" style={{ fontSize: '14px', marginTop: '16px', marginBottom: '8px' }}>NARRATION FREQUENCY</div>
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
                        <span className="role-num-sm" style={{ minWidth: '12px', textAlign: 'right' }}>{narrationFrequency}</span>
                    </div>
                    <div className="role-text-sm" style={{ display: 'flex', justifyContent: 'space-between', fontSize: '9px', opacity: 0.7 }}>
                        <span>Rarely</span>
                        <span>Normal</span>
                        <span>Active</span>
                        <span>Busy</span>
                        <span>Constant</span>
                    </div>
                </div>

                <div className="role-header" style={{ fontSize: '14px', marginTop: '16px', marginBottom: '8px' }}>TEXT LENGTH</div>
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
                        <span className="role-num-sm" style={{ minWidth: '12px', textAlign: 'right' }}>{textLength}</span>
                    </div>
                    <div className="role-text-sm" style={{ display: 'flex', justifyContent: 'space-between', fontSize: '9px', opacity: 0.7 }}>
                        <span>Shortest</span>
                        <span>Shorter</span>
                        <span>Normal</span>
                        <span>Longer</span>
                        <span>Longest</span>
                    </div>
                </div>


                <div className="role-header" style={{ fontSize: '14px', marginTop: '16px', marginBottom: '8px' }}>DEBUG LAYERS</div>
                <div className="radio-group">
                    <label className="role-text-sm" style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                        <input
                            type="checkbox"
                            checked={showCacheLayer}
                            onChange={(e) => onCacheLayerChange(e.target.checked)}
                        /> Show Cache Layer
                    </label>
                    <label className="role-text-sm" style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                        <input
                            type="checkbox"
                            checked={showVisibilityLayer}
                            onChange={(e) => onVisibilityLayerChange(e.target.checked)}
                        /> Show Visibility Overlay
                    </label>
                </div>

                <div className="role-header" style={{ fontSize: '14px', marginTop: '16px', marginBottom: '8px', color: '#ff4444' }}>DANGER ZONE</div>
                <button
                    className="role-btn"
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
                        fontSize: '11px',
                        width: '100%',
                        transition: 'all 0.2s'
                    }}
                    onMouseOver={(e) => { e.currentTarget.style.background = '#f57c00'; e.currentTarget.style.color = 'white'; }}
                    onMouseOut={(e) => { e.currentTarget.style.background = 'rgba(255, 152, 0, 0.1)'; e.currentTarget.style.color = '#f57c00'; }}
                >
                    RESET HISTORY (100km)
                </button>
                <button
                    className="role-btn"
                    onClick={() => { if (confirm('Are you sure you want to SHUTDOWN the server?')) fetch('/api/shutdown', { method: 'POST' }) }}
                    style={{
                        marginTop: '8px',
                        padding: '6px 12px',
                        background: 'rgba(211, 47, 47, 0.1)',
                        color: '#ff4444',
                        border: '1px solid #d32f2f',
                        borderRadius: '4px',
                        cursor: 'pointer',
                        fontSize: '11px',
                        width: '100%',
                        transition: 'all 0.2s'
                    }}
                    onMouseOver={(e) => { e.currentTarget.style.background = '#d32f2f'; e.currentTarget.style.color = 'white'; }}
                    onMouseOut={(e) => { e.currentTarget.style.background = 'rgba(211, 47, 47, 0.1)'; e.currentTarget.style.color = '#ff4444'; }}
                >
                    SHUTDOWN SERVER
                </button>
            </div>
        </div>
    );
};
