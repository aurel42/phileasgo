import type { Telemetry } from '../../types/telemetry';
import { useNavigate } from 'react-router-dom';

interface ConfigBoxProps {
    telemetry: Telemetry;
    config: any;
}

export const ConfigBox = ({ telemetry, config }: ConfigBoxProps) => {
    const navigate = useNavigate();

    return (
        <div className="stat-box config-box" onClick={() => navigate('/settings')} style={{ cursor: 'pointer' }}>
            <div className="overlay-config-status" style={{ display: 'grid', gridTemplateColumns: 'min-content min-content', gap: '4px 12px', alignItems: 'center' }}>
                <span className="role-label-overlay" style={{ textAlign: 'left' }}>SIM</span>
                <span style={{ fontSize: '10px', display: 'flex', alignItems: 'center', justifyContent: 'flex-end' }}>
                    {telemetry.SimState === 'active' ? ((telemetry.IsOnGround === false && telemetry.GroundSpeed < 1) ? '🟠' : '🟢') : telemetry.SimState === 'inactive' ? '🟠' : '🔴'}
                </span>

                <span className="role-label-overlay" style={{ textAlign: 'left', whiteSpace: 'nowrap' }}>{config.filter_mode === 'adaptive' ? 'ADAPTIVE' : 'FIXED'}</span>
                <span className="icon" style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end' }}>{config.filter_mode === 'adaptive' ? '⚡' : '🎯'}</span>

                <span className="role-label-overlay" style={{ textAlign: 'left' }}>FRQ</span>
                <div className="pips" style={{ display: 'flex', gap: '2px', alignItems: 'center', justifyContent: 'flex-end' }}>
                    {[1, 2, 3, 4, 5].map(v => (
                        <div key={v} className={`pip ${v <= (config.narration_frequency || 0) ? 'active' : ''} ${v > 3 && v <= (config.narration_frequency || 0) ? 'high' : ''}`} />
                    ))}
                </div>

                <span className="role-label-overlay" style={{ textAlign: 'left' }}>LEN</span>
                <div className="pips" style={{ display: 'flex', gap: '2px', alignItems: 'center', justifyContent: 'flex-end' }}>
                    {[1, 2, 3, 4, 5].map(v => (
                        <div key={v} className={`pip ${v <= (config.text_length || 0) ? 'active' : ''} ${v > 4 && v <= (config.text_length || 0) ? 'high' : ''}`} />
                    ))}
                </div>
            </div>
        </div>
    );
};
