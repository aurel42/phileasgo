import { useRegionalCategories } from '../hooks/useRegionalCategories';

export const RegionalCategoriesCard = () => {
    const { data: categories, isLoading, error } = useRegionalCategories();

    // Do not render anything if there are no regional categories active
    if (!categories || categories.length === 0 || error) {
        return null;
    }

    return (
        <div className="flex-card" style={{ marginTop: '12px', padding: '12px 16px' }}>
            <div className="role-header" style={{ marginBottom: '8px' }}>
                Active Regional Context
            </div>

            {isLoading ? (
                <div className="role-text-sm" style={{ opacity: 0.5 }}>Loading...</div>
            ) : (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                    {categories.map((cat) => (
                        <div
                            key={cat.qid}
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
                                {cat.category}
                            </span>
                            <span className="role-num-sm">
                                {cat.name}
                            </span>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
};
