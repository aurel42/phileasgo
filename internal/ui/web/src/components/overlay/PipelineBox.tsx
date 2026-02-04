interface PipelineBoxProps {
    stats: any;
}

export const PipelineBox = ({ stats }: PipelineBoxProps) => {
    if (!stats?.providers) return null;
    const fallbackOrder = stats.llm_fallback || [];
    const toRoman = (num: number) => ["I", "II", "III", "IV", "V"][num] || (num + 1).toString();

    const pipelineItems = Object.entries(stats.providers)
        .filter(([key]) => fallbackOrder.includes(key))
        .sort(([keyA], [keyB]) => fallbackOrder.indexOf(keyA) - fallbackOrder.indexOf(keyB));

    if (pipelineItems.length === 0) return null;

    return (
        <div className="stat-box" style={{ minWidth: '180px', alignItems: 'flex-start' }}>
            <div className="stat-value" style={{
                display: 'grid',
                gridTemplateColumns: 'max-content 1fr max-content',
                columnGap: '8px',
                rowGap: '2px',
                textAlign: 'left',
                alignItems: 'baseline',
                width: '100%'
            }}>
                {pipelineItems.map(([key, data]: [string, any], idx) => {
                    if (!data) return null;
                    if (data.api_success === 0 && data.api_errors === 0) return null;
                    const label = key.toUpperCase().replace('-', ' ');
                    return (
                        <div key={key} style={{ display: 'contents' }}>
                            <div style={{ display: 'flex', alignItems: 'baseline' }}>
                                <span className="roman-numeral">{toRoman(idx)}</span>
                                <div className="role-header" style={{ fontSize: '14px', whiteSpace: 'nowrap' }}>
                                    {label}
                                </div>
                            </div>
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
