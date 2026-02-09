import React, { useEffect, useRef, useState } from 'react';

const cache = new Map<string, string>();

interface InlineSVGProps {
    src: string;
    className?: string;
    style?: React.CSSProperties;
    fill?: string;
}

export const InlineSVG: React.FC<InlineSVGProps> = ({ src, className, style, fill }) => {
    const [svgContent, setSvgContent] = useState<string | null>(cache.get(src) || null);
    const divRef = useRef<HTMLDivElement>(null);

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
                // Strip XML declaration, keep original viewBox (0 0 15 15) for consistent scaling
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
            ref={divRef}
            className={className}
            style={{
                ...style,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: fill
            }}
            dangerouslySetInnerHTML={{ __html: svgContent }}
        />
    );
};
