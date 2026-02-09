import React, { useEffect, useState } from 'react';

const cache = new Map<string, string>();

interface InlineSVGProps {
    src: string;
    className?: string;
    style?: React.CSSProperties;
    fill?: string;
}

export const InlineSVG: React.FC<InlineSVGProps> = ({ src, className, style, fill }) => {
    const [svgContent, setSvgContent] = useState<string | null>(cache.get(src) || null);

    useEffect(() => {
        if (cache.has(src)) {
            setSvgContent(cache.get(src)!);
            return;
        }

        let active = true;
        fetch(src)
            .then(res => res.text())
            .then(text => {
                if (!active) return;
                // Simple cleanup: remove XML declaration and ensure width/height are 100% or stripped
                // For now, we just strip the XML tag.
                const clean = text.replace(/<\?xml.*?\?>/g, '').trim();
                cache.set(src, clean);
                setSvgContent(clean);
            })
            .catch(err => console.error(`Failed to load SVG: ${src}`, err));

        return () => { active = false; };
    }, [src]);

    if (!svgContent) return <div className={className} style={style} />;

    return (
        <div
            className={className}
            style={{
                ...style,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                // Propagate colors to SVG via CSS variables if not set explicitly
                // But inline styles on the container don't auto-inherit to paths unless paths use 'currentColor'
                // The SVGs in public/icons likely don't use currentColor.
                // We'll trust the parent to use CSS or style props that target 'svg path'.
                color: fill
            }}
            dangerouslySetInnerHTML={{ __html: svgContent }}
        />
    );
};
