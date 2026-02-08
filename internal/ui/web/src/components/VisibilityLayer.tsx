import { useEffect, useState } from 'react';
import { useMap, ImageOverlay } from 'react-leaflet';
import L from 'leaflet';

interface MaskResponse {
    mask: number[];
    rows: number;
    cols: number;
    bounds: {
        north: number;
        east: number;
        south: number;
        west: number;
    };
}

interface OverlayData {
    urlM: string | null;
    urlL: string | null;
    urlXL: string | null;
    bounds: L.LatLngBoundsExpression | null;
}

// Color definitions: RGB
const COLOR_M = { r: 255, g: 200, b: 0 };    // Deep Gold
const COLOR_L = { r: 255, g: 225, b: 60 };   // Bright Yellow
const COLOR_XL = { r: 255, g: 250, b: 120 }; // Pale Yellow

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
            // Use sqrt(score) to boost low visibility areas (non-linear falloff)
            // This makes the "tail" of the visibility (outer rings) more visible
            alpha = Math.floor(MIN_ALPHA + (Math.sqrt(score) * (MAX_ALPHA - MIN_ALPHA)));
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

// --- Light map tile compositing for renderAsMap mode ---

const TILE_SIZE = 256;
const TILE_SUBDOMAINS = ['a', 'b', 'c', 'd'];

// Browser-level tile image cache (persists across renders, cleared on page reload)
const tileImageCache = new Map<string, HTMLImageElement>();

function loadTileImage(url: string): Promise<HTMLImageElement | null> {
    const cached = tileImageCache.get(url);
    if (cached) return Promise.resolve(cached);

    return new Promise((resolve) => {
        const img = new Image();
        img.crossOrigin = 'anonymous';
        img.onload = () => {
            tileImageCache.set(url, img);
            resolve(img);
        };
        img.onerror = () => resolve(null);
        img.src = url;
    });
}

async function createLightMapOverlay(
    mask: number[],
    maskCols: number,
    maskRows: number,
    bounds: { north: number; east: number; south: number; west: number },
    map: L.Map
): Promise<string> {
    const zoom = Math.round(map.getZoom());

    // Determine which tiles cover the visibility bounds in world-pixel space
    const nwPx = map.project([bounds.north, bounds.west], zoom);
    const sePx = map.project([bounds.south, bounds.east], zoom);
    const minTX = Math.floor(nwPx.x / TILE_SIZE);
    const minTY = Math.floor(nwPx.y / TILE_SIZE);
    const maxTX = Math.floor(sePx.x / TILE_SIZE);
    const maxTY = Math.floor(sePx.y / TILE_SIZE);

    // Fetch all needed tiles in parallel
    const tiles = new Map<string, HTMLImageElement>();
    const promises: Promise<void>[] = [];

    for (let tx = minTX; tx <= maxTX; tx++) {
        for (let ty = minTY; ty <= maxTY; ty++) {
            const s = TILE_SUBDOMAINS[Math.abs(tx + ty) % 4];
            const url = `https://${s}.basemaps.cartocdn.com/rastertiles/voyager/${zoom}/${tx}/${ty}.png`;
            promises.push(
                loadTileImage(url).then(img => {
                    if (img) tiles.set(`${tx},${ty}`, img);
                })
            );
        }
    }
    await Promise.all(promises);

    // Canvas at native tile resolution for the covered area
    const canvasW = Math.round(sePx.x - nwPx.x);
    const canvasH = Math.round(sePx.y - nwPx.y);
    const cvs = document.createElement('canvas');
    cvs.width = canvasW;
    cvs.height = canvasH;
    const ctx = cvs.getContext('2d')!;

    // Step 1: Draw tiles at native resolution using drawImage
    for (let tx = minTX; tx <= maxTX; tx++) {
        for (let ty = minTY; ty <= maxTY; ty++) {
            const tileImg = tiles.get(`${tx},${ty}`);
            if (!tileImg) continue;

            const destX = tx * TILE_SIZE - nwPx.x;
            const destY = ty * TILE_SIZE - nwPx.y;
            ctx.drawImage(tileImg, destX, destY, TILE_SIZE, TILE_SIZE);
        }
    }

    // Step 2: Apply visibility mask as alpha modulation
    const imgData = ctx.getImageData(0, 0, canvasW, canvasH);
    for (let py = 0; py < canvasH; py++) {
        const maskRow = Math.min(maskRows - 1, Math.floor(py / canvasH * maskRows));
        for (let px = 0; px < canvasW; px++) {
            const maskCol = Math.min(maskCols - 1, Math.floor(px / canvasW * maskCols));
            const score = mask[maskRow * maskCols + maskCol];
            const idx = (py * canvasW + px) * 4;
            imgData.data[idx + 3] = Math.floor(score * 255);
        }
    }
    ctx.putImageData(imgData, 0, 0);

    return cvs.toDataURL();
}

// --- Component ---

export const VisibilityLayer = ({ enabled, renderAsMap }: { enabled: boolean; renderAsMap: boolean }) => {
    const map = useMap();
    const [layers, setLayers] = useState<OverlayData>({ urlM: null, urlL: null, urlXL: null, bounds: null });

    useEffect(() => {
        if (!enabled) {
            // eslint-disable-next-line react-hooks/set-state-in-effect
            setLayers({ urlM: null, urlL: null, urlXL: null, bounds: null });
            return;
        }

        const fetchAndDraw = async () => {
            const b = map.getBounds();
            const query = new URLSearchParams({
                bounds: `${b.getNorth()},${b.getEast()},${b.getSouth()},${b.getWest()}`,
                resolution: '64'
            });

            try {
                if (renderAsMap) {
                    // Fetch combined visibility mask
                    const res = await fetch(`/api/map/visibility-mask?${query}`);
                    if (!res.ok) return;
                    const data: MaskResponse = await res.json();

                    // Draw Voyager tiles at native resolution, apply visibility as alpha
                    const overlayUrl = await createLightMapOverlay(
                        data.mask, data.cols, data.rows, data.bounds, map
                    );

                    const bounds: L.LatLngBoundsExpression = [
                        [data.bounds.south, data.bounds.west],
                        [data.bounds.north, data.bounds.east]
                    ];

                    setLayers({ urlM: overlayUrl, urlL: null, urlXL: null, bounds });
                } else {
                    const res = await fetch(`/api/map/visibility?${query}`);
                    if (!res.ok) return;
                    const data: any = await res.json();

                    const urlM = createCanvasUrl(data.gridM, data.cols, data.rows, COLOR_M);
                    const urlL = createCanvasUrl(data.gridL, data.cols, data.rows, COLOR_L);
                    const urlXL = createCanvasUrl(data.gridXL, data.cols, data.rows, COLOR_XL);

                    const bounds: L.LatLngBoundsExpression = [
                        [data.bounds.south, data.bounds.west],
                        [data.bounds.north, data.bounds.east]
                    ];

                    setLayers({ urlM, urlL, urlXL, bounds });
                }
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
    }, [enabled, renderAsMap, map]);

    if (!enabled || !layers.bounds) return null;

    return (
        <>
            {renderAsMap ? (
                // Voyager tiles composited with visibility alpha — dark base shows through
                layers.urlM && (
                    <ImageOverlay url={layers.urlM} bounds={layers.bounds} opacity={1} zIndex={500} />
                )
            ) : (
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
            )}
        </>
    );
};
