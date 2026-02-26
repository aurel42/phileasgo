import React from 'react';

export type TabId = 'dashboard' | 'detail' | 'pois' | 'cities' | 'regional' | 'diagnostics';

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
                className={`dashboard-tab ${activeTab === 'detail' ? 'active' : ''}`}
                onClick={() => onTabChange('detail')}
            >
                Detail
            </button>
            <button
                className={`dashboard-tab ${activeTab === 'pois' ? 'active' : ''}`}
                onClick={() => onTabChange('pois')}
            >
                POIs
            </button>
            <button
                className={`dashboard-tab ${activeTab === 'cities' ? 'active' : ''}`}
                onClick={() => onTabChange('cities')}
            >
                Cities
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
