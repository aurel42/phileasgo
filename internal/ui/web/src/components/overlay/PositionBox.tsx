import type { Telemetry } from '../../types/telemetry';

interface PositionBoxProps {
    telemetry: Telemetry;
    location: any;
}

export const PositionBox = ({ telemetry, location }: PositionBoxProps) => (
    <div className="stat-box" style={{ minWidth: (location?.city || location?.country) ? '220px' : '180px' }}>
        {(location?.city || location?.country) ? (
            <>
                <div className="role-text-lg" style={{ textAlign: 'center' }}>
                    {location.city ? (
                        location.city === 'Unknown' ? (
                            <span>Far from civilization</span>
                        ) : (
                            <>
                                <span className="role-label-overlay" style={{ marginRight: '6px' }}>near</span>
                                {location.city}
                            </>
                        )
                    ) : (
                        <span>{location.country}</span>
                    )}
                </div>
                <div className="role-text-sm" style={{ textAlign: 'center', marginTop: '4px' }}>
                    {location.city_country_code && location.country_code && location.city_country_code !== location.country_code ? (
                        <>
                            <div>{location.city_region ? `${location.city_region}, ` : ''}{location.city_country}</div>
                            <div style={{ color: 'var(--accent)', marginTop: '2px' }}>in {location.country}</div>
                        </>
                    ) : (
                        <>{location.region ? `${location.region}, ` : ''}{location.city ? location.country : (location.region ? '' : '')}</>
                    )}
                </div>
                <div className="role-num-sm" style={{ color: 'var(--muted)', marginTop: '8px', textAlign: 'center' }}>
                    {telemetry.Latitude.toFixed(4)}, {telemetry.Longitude.toFixed(4)}
                </div>
            </>
        ) : (
            <div className="stat-value" style={{ textAlign: 'center' }}>
                <span className="role-label-overlay">LAT </span><span className="role-num-sm">{telemetry.Latitude.toFixed(4)}</span> <br />
                <span className="role-label-overlay">LON </span><span className="role-num-sm">{telemetry.Longitude.toFixed(4)}</span>
            </div>
        )}
    </div>
);
