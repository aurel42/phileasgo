interface BrandingBoxProps {
    version: string;
}

export const BrandingBox = ({ version }: BrandingBoxProps) => (
    <div className="stat-box branding-box" style={{ minWidth: '120px' }}>
        <div className="role-title" style={{ fontSize: '18px', lineHeight: '1.1', textAlign: 'center' }}>
            PHILEAS<br />
            TOUR GUIDE
        </div>
        <div className="role-num-sm" style={{ textAlign: 'center', marginTop: '6px', color: '#bbb' }}>
            {version}
        </div>
    </div>
);
