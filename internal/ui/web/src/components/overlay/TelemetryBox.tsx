import type { Telemetry } from '../../types/telemetry';

interface TelemetryBoxProps {
    telemetry: Telemetry;
}

export const TelemetryBox = ({ telemetry }: TelemetryBoxProps) => (
    <div className="stat-box" style={{ alignItems: 'flex-start', minWidth: '140px' }}>
        <div className="stat-value" style={{
            display: 'grid',
            gridTemplateColumns: '30px 1fr 34px',
            columnGap: '8px',
            rowGap: '2px',
            textAlign: 'right',
            alignItems: 'baseline'
        }}>
            <div className="role-label-overlay" style={{ textAlign: 'left' }}>HDG</div>
            <div className="role-num-lg" style={{ fontSize: '20px' }}>{Math.round(telemetry.Heading)}</div>
            <div className="role-label-overlay" style={{ fontSize: '14px', textAlign: 'left' }}>deg.</div>

            <div className="role-label-overlay" style={{ textAlign: 'left' }}>GS</div>
            <div className="role-num-lg" style={{ fontSize: '20px' }}>{Math.round(telemetry.GroundSpeed)}</div>
            <div className="role-label-overlay" style={{ fontSize: '14px', textAlign: 'left' }}>kts</div>

            <div className="role-label-overlay" style={{ textAlign: 'left' }}>AGL</div>
            <div className="role-num-lg" style={{ fontSize: '20px' }}>{Math.round(telemetry.AltitudeAGL)}</div>
            <div className="role-label-overlay" style={{ fontSize: '14px', textAlign: 'left' }}>ft</div>

            <div className="role-label-overlay" style={{ textAlign: 'left' }}>MSL</div>
            <div className="role-num-lg" style={{ fontSize: '20px' }}>{Math.round(telemetry.AltitudeMSL)}</div>
            <div className="role-label-overlay" style={{ fontSize: '14px', textAlign: 'left' }}>ft</div>
        </div>
    </div>
);
