import React from 'react';

export type TabId = 'dashboard' | 'poi' | 'regional' | 'diagnostics';

interface DashboardTabsProps {
    activeTab: TabId;
    onTabChange: (tab: TabId) => void;
}

export const DashboardTabs: React.FC<DashboardTabsProps> = ({ activeTab, onTabChange }) => {
    return (
        <div className="dashboard-tabs">
            <button
                className={`dashboard-tab ${activeTab === 'dashboard' ? 'active' : ''}`}
                onClick={() => onTabChange('dashboard')}
            >
                Dashboard
            </button>
            <button
                className={`dashboard-tab ${activeTab === 'poi' ? 'active' : ''}`}
                onClick={() => onTabChange('poi')}
            >
                POI
            </button>
            <button
                className={`dashboard-tab ${activeTab === 'regional' ? 'active' : ''}`}
                onClick={() => onTabChange('regional')}
            >
                Regional
            </button>
            <button
                className={`dashboard-tab ${activeTab === 'diagnostics' ? 'active' : ''}`}
                onClick={() => onTabChange('diagnostics')}
            >
                Diagnostics
            </button>
        </div>
    );
};
