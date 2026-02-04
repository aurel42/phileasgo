interface StatsBoxProps {
    stats: any;
}

export const StatsBox = ({ stats }: StatsBoxProps) => (
    <div className="stat-box" style={{ minWidth: '140px', alignItems: 'flex-start' }}>
        <div className="stat-value" style={{
            display: 'grid',
            gridTemplateColumns: 'max-content 1fr 24px',
            columnGap: '8px',
            rowGap: '2px',
            alignItems: 'baseline'
        }}>
            <div className="role-label-overlay">MEM (RSS)</div>
            <div className="role-num-sm" style={{ textAlign: 'right' }}>{stats?.system?.memory_alloc_mb || 0}</div>
            <div className="role-label-overlay" style={{ fontSize: '12px' }}>MB</div>

            <div className="role-label-overlay">MEM (max)</div>
            <div className="role-num-sm" style={{ textAlign: 'right' }}>{stats?.system?.memory_max_mb || 0}</div>
            <div className="role-label-overlay" style={{ fontSize: '12px' }}>MB</div>

            <div className="role-label-overlay">Tracked</div>
            <div className="role-num-sm" style={{ textAlign: 'right' }}>{stats?.tracking?.active_pois || 0}</div>
            <div className="role-label-overlay" style={{ fontSize: '12px' }}>POIs</div>
        </div>
    </div>
);
