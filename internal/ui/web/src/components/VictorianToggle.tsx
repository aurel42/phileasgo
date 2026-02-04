import React from 'react';

interface VictorianToggleProps {
    checked: boolean;
    onChange: (checked: boolean) => void;
    label?: string;
}

export const VictorianToggle: React.FC<VictorianToggleProps> = ({ checked, onChange, label }) => {
    return (
        <div className="v-toggle-container">
            {label && <span className="role-label">{label}</span>}
            <label className="settings-toggle">
                <input
                    type="checkbox"
                    checked={checked}
                    onChange={e => onChange(e.target.checked)}
                />
                <span className="toggle-slider"></span>
            </label>
        </div>
    );
};
