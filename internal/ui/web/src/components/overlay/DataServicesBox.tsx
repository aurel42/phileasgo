interface DataServicesBoxProps {
    stats: any;
}

export const DataServicesBox = ({ stats }: DataServicesBoxProps) => {
    if (!stats?.providers) return null;
    const fallbackOrder = stats.llm_fallback || [];

    const serviceItems = Object.entries(stats.providers)
        .filter(([key]) => !fallbackOrder.includes(key))
        .sort(([keyA], [keyB]) => keyA.localeCompare(keyB));

    if (serviceItems.length === 0) return null;

    return (
        <div className="stat-box" style={{ minWidth: '160px', alignItems: 'flex-start' }}>
            <div className="stat-value" style={{
                display: 'grid',
                gridTemplateColumns: 'max-content 1fr max-content',
                columnGap: '8px',
                rowGap: '2px',
                textAlign: 'left',
                alignItems: 'baseline',
                width: '100%'
            }}>
                {serviceItems.map(([key, data]: [string, any]) => {
                    if (!data) return null;
                    if (data.api_success === 0 && data.api_errors === 0) return null;
                    const label = key.toUpperCase().replace('-', ' ');
                    return (
                        <div key={key} style={{ display: 'contents' }}>
                            <div className="role-header" style={{ fontSize: '14px', whiteSpace: 'nowrap' }}>{label}</div>
                            <div className="role-num-sm" style={{ textAlign: 'right', paddingRight: '4px' }}>
                                <span style={{ color: 'var(--success)' }}>{data.api_success}</span>
                                <span style={{ color: 'var(--muted)', margin: '0 4px', fontSize: '10px' }}>◆</span>
                                <span style={{ color: 'var(--error)' }}>{data.api_errors}</span>
                            </div>
                            <div style={{ width: '12px', fontSize: '14px' }}>{data.free_tier === false ? '£' : ''}</div>
                        </div>
                    );
                })}
            </div>
        </div>
    );
};
