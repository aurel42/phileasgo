import { useSpatialFeatures } from '../hooks/useSpatialFeatures';

export const SpatialFeaturesCard = () => {
    const { data: features, isLoading, error } = useSpatialFeatures();

    // Do not render anything if there are no features active
    if (!features || features.length === 0 || error) {
        return null;
    }

    return (
        <div className="flex-card" style={{ marginTop: '12px', padding: '12px 16px' }}>
            <div className="role-header" style={{ marginBottom: '8px' }}>
                Active Geographical Features
            </div>

            {isLoading ? (
                <div className="role-text-sm" style={{ opacity: 0.5 }}>Loading...</div>
            ) : (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                    {features.map((feat) => (
                        <div
                            key={feat.qid}
                            style={{
                                display: 'inline-flex',
                                alignItems: 'center',
                                background: 'rgba(212, 175, 55, 0.1)',
                                border: '1px solid rgba(212, 175, 55, 0.3)',
                                borderRadius: '4px',
                                padding: '4px 8px',
                            }}
                        >
                            <span className="role-label" style={{ color: 'var(--accent)', marginRight: '6px' }}>
                                {feat.category}
                            </span>
                            <span className="role-num-sm">
                                {feat.name}
                            </span>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
};
