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
    const [mockLat, setMockLat] = useState<number>(0);
    const [mockLon, setMockLon] = useState<number>(0);
    const [mockAlt, setMockAlt] = useState<number>(0);
    const [mockHeading, setMockHeading] = useState<number | null>(null);
    const [mockDurParked, setMockDurParked] = useState<string>('120s');
    const [mockDurTaxi, setMockDurTaxi] = useState<string>('120s');
    const [mockDurHold, setMockDurHold] = useState<string>('30s');

    useEffect(() => {
        fetch('/api/config')
            .then(r => r.json())
            .then(data => {
                setSimSource(data.sim_source || 'mock');
                setMockLat(data.mock_start_lat || 0);
                setMockLon(data.mock_start_lon || 0);
                setMockAlt(data.mock_start_alt || 0);
                setMockHeading(data.mock_start_heading); // Can be null (random)
                setMockDurParked(data.mock_duration_parked || '120s');
                setMockDurTaxi(data.mock_duration_taxi || '120s');
                setMockDurHold(data.mock_duration_hold || '30s');
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
                <div className="role-header" style={{ fontSize: '14px', marginBottom: '8px' }}>SIMULATION SOURCE*</div>
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
                    * Restart required after changing
                </div>

                {simSource === 'mock' && (
                    <div style={{ marginTop: '24px', padding: '16px', background: 'rgba(212, 175, 55, 0.05)', borderLeft: '4px solid var(--accent)', borderRadius: '4px' }}>
                        <div className="role-header" style={{ fontSize: '14px', marginBottom: '12px' }}>MOCK SIMULATION SETTINGS*</div>

                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                            <div>
                                <label className="role-text-sm" style={{ display: 'block', marginBottom: '4px' }}>START LATITUDE</label>
                                <input
                                    type="number"
                                    step="0.0001"
                                    value={mockLat}
                                    onChange={(e) => {
                                        const v = parseFloat(e.target.value);
                                        setMockLat(v);
                                        fetch('/api/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mock_start_lat: v }) });
                                    }}
                                    className="role-num-sm"
                                    style={{ width: '100%', background: 'var(--card-bg)', color: 'var(--accent)', border: '1px solid rgba(212, 175, 55, 0.3)', padding: '4px' }}
                                />
                            </div>
                            <div>
                                <label className="role-text-sm" style={{ display: 'block', marginBottom: '4px' }}>START LONGITUDE</label>
                                <input
                                    type="number"
                                    step="0.0001"
                                    value={mockLon}
                                    onChange={(e) => {
                                        const v = parseFloat(e.target.value);
                                        setMockLon(v);
                                        fetch('/api/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mock_start_lon: v }) });
                                    }}
                                    className="role-num-sm"
                                    style={{ width: '100%', background: 'var(--card-bg)', color: 'var(--accent)', border: '1px solid rgba(212, 175, 55, 0.3)', padding: '4px' }}
                                />
                            </div>
                            <div>
                                <label className="role-text-sm" style={{ display: 'block', marginBottom: '4px' }}>START ALTITUDE (FT)</label>
                                <input
                                    type="number"
                                    value={mockAlt}
                                    onChange={(e) => {
                                        const v = parseFloat(e.target.value);
                                        setMockAlt(v);
                                        fetch('/api/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mock_start_alt: v }) });
                                    }}
                                    className="role-num-sm"
                                    style={{ width: '100%', background: 'var(--card-bg)', color: 'var(--accent)', border: '1px solid rgba(212, 175, 55, 0.3)', padding: '4px' }}
                                />
                            </div>
                            <div>
                                <label className="role-text-sm" style={{ display: 'block', marginBottom: '4px' }}>START HEADING (DEG)</label>
                                <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                                    <input
                                        type="number"
                                        min="0"
                                        max="359"
                                        disabled={mockHeading === null}
                                        value={mockHeading === null ? '' : mockHeading}
                                        onChange={(e) => {
                                            const v = parseFloat(e.target.value);
                                            setMockHeading(v);
                                            fetch('/api/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mock_start_heading: v }) });
                                        }}
                                        className="role-num-sm"
                                        placeholder="Random"
                                        style={{ flex: 1, background: 'var(--card-bg)', color: 'var(--accent)', border: '1px solid rgba(212, 175, 55, 0.3)', padding: '4px', opacity: mockHeading === null ? 0.5 : 1 }}
                                    />
                                    <button
                                        onClick={() => {
                                            const newVal = mockHeading === null ? 0 : null;
                                            setMockHeading(newVal);
                                            fetch('/api/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mock_start_heading: newVal }) });
                                        }}
                                        style={{ fontSize: '10px', padding: '4px 8px', background: 'rgba(212,175,55,0.1)', color: 'var(--accent)', border: '1px solid var(--accent)', cursor: 'pointer' }}
                                    >
                                        {mockHeading === null ? 'FIX' : 'RANDOM'}
                                    </button>
                                </div>
                            </div>
                        </div>

                        <div style={{ marginTop: '16px', display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '12px' }}>
                            <div>
                                <label className="role-text-sm" style={{ display: 'block', marginBottom: '2px', fontSize: '10px' }}>PARKED DUR.</label>
                                <input
                                    type="text"
                                    value={mockDurParked}
                                    onChange={(e) => {
                                        setMockDurParked(e.target.value);
                                        fetch('/api/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mock_duration_parked: e.target.value }) });
                                    }}
                                    className="role-num-sm"
                                    style={{ width: '100%', background: 'var(--card-bg)', color: 'var(--accent)', border: '1px solid rgba(212, 175, 55, 0.3)', padding: '4px' }}
                                />
                            </div>
                            <div>
                                <label className="role-text-sm" style={{ display: 'block', marginBottom: '2px', fontSize: '10px' }}>TAXI DUR.</label>
                                <input
                                    type="text"
                                    value={mockDurTaxi}
                                    onChange={(e) => {
                                        setMockDurTaxi(e.target.value);
                                        fetch('/api/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mock_duration_taxi: e.target.value }) });
                                    }}
                                    className="role-num-sm"
                                    style={{ width: '100%', background: 'var(--card-bg)', color: 'var(--accent)', border: '1px solid rgba(212, 175, 55, 0.3)', padding: '4px' }}
                                />
                            </div>
                            <div>
                                <label className="role-text-sm" style={{ display: 'block', marginBottom: '2px', fontSize: '10px' }}>HOLD DUR.</label>
                                <input
                                    type="text"
                                    value={mockDurHold}
                                    onChange={(e) => {
                                        setMockDurHold(e.target.value);
                                        fetch('/api/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mock_duration_hold: e.target.value }) });
                                    }}
                                    className="role-num-sm"
                                    style={{ width: '100%', background: 'var(--card-bg)', color: 'var(--accent)', border: '1px solid rgba(212, 175, 55, 0.3)', padding: '4px' }}
                                />
                            </div>
                        </div>
                    </div>
                )}

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
