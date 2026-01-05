import { useEffect, useState } from 'react';
import { useMap, ImageOverlay } from 'react-leaflet';
import L from 'leaflet';

interface VisibilityResponse {
    gridM: number[];
    gridL: number[];
    gridXL: number[];
    rows: number;
    cols: number;
    bounds: {
        north: number;
        east: number;
        south: number;
        west: number;
    };
}

interface LayerData {
    urlM: string | null;
    urlL: string | null;
    urlXL: string | null;
    bounds: L.LatLngBoundsExpression | null;
}

// Color definitions: RGB
const COLOR_M = { r: 255, g: 80, b: 80 };    // Red
const COLOR_L = { r: 255, g: 180, b: 0 };    // Orange
const COLOR_XL = { r: 255, g: 230, b: 0 };   // Yellow

// Reduced opacity (50%): min 15%, max 40%
const MIN_ALPHA = 38;  // 0.15 * 255
const MAX_ALPHA = 102; // 0.40 * 255

function createCanvasUrl(
    grid: number[],
    cols: number,
    rows: number,
    color: { r: number; g: number; b: number }
): string {
    const cvs = document.createElement('canvas');
    cvs.width = cols;
    cvs.height = rows;
    const ctx = cvs.getContext('2d');
    if (!ctx) return '';

    const imgData = ctx.createImageData(cols, rows);
    for (let i = 0; i < grid.length; i++) {
        const score = grid[i];
        let alpha = 0;
        if (score > 0) {
            alpha = Math.floor(MIN_ALPHA + (score * (MAX_ALPHA - MIN_ALPHA)));
        }
        const idx = i * 4;
        imgData.data[idx] = color.r;
        imgData.data[idx + 1] = color.g;
        imgData.data[idx + 2] = color.b;
        imgData.data[idx + 3] = alpha;
    }
    ctx.putImageData(imgData, 0, 0);
    return cvs.toDataURL();
}

export const VisibilityLayer = ({ enabled }: { enabled: boolean }) => {
    const map = useMap();
    const [layers, setLayers] = useState<LayerData>({ urlM: null, urlL: null, urlXL: null, bounds: null });

    useEffect(() => {
        if (!enabled) {
            setLayers({ urlM: null, urlL: null, urlXL: null, bounds: null });
            return;
        }

        const fetchAndDraw = async () => {
            const b = map.getBounds();
            const query = new URLSearchParams({
                bounds: `${b.getNorth()},${b.getEast()},${b.getSouth()},${b.getWest()}`,
                resolution: '32'
            });

            try {
                const res = await fetch(`/api/map/visibility?${query}`);
                if (!res.ok) return;
                const data: VisibilityResponse = await res.json();

                const urlM = createCanvasUrl(data.gridM, data.cols, data.rows, COLOR_M);
                const urlL = createCanvasUrl(data.gridL, data.cols, data.rows, COLOR_L);
                const urlXL = createCanvasUrl(data.gridXL, data.cols, data.rows, COLOR_XL);

                const bounds: L.LatLngBoundsExpression = [
                    [data.bounds.south, data.bounds.west],
                    [data.bounds.north, data.bounds.east]
                ];

                setLayers({ urlM, urlL, urlXL, bounds });
            } catch (e) {
                console.warn("Visibility fetch failed", e);
            }
        };

        fetchAndDraw();
        const interval = setInterval(fetchAndDraw, 15000); // 15s refresh
        map.on('moveend', fetchAndDraw); // Refresh on drag/zoom

        return () => {
            clearInterval(interval);
            map.off('moveend', fetchAndDraw);
        };
    }, [enabled, map]);

    if (!enabled || !layers.bounds) return null;

    return (
        <>
            {/* XL is the outermost, render first (bottom) */}
            {layers.urlXL && (
                <ImageOverlay url={layers.urlXL} bounds={layers.bounds} opacity={1} zIndex={500} />
            )}
            {/* L is middle */}
            {layers.urlL && (
                <ImageOverlay url={layers.urlL} bounds={layers.bounds} opacity={1} zIndex={501} />
            )}
            {/* M is innermost, render on top */}
            {layers.urlM && (
                <ImageOverlay url={layers.urlM} bounds={layers.bounds} opacity={1} zIndex={502} />
            )}
        </>
    );
};
