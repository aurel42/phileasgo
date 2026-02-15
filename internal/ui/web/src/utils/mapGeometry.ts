import maplibregl from 'maplibre-gl';
import type { Feature, Polygon, MultiPolygon } from 'geojson';

export const rayToEdge = (px: number, py: number, dx: number, dy: number, mapWidth: number, mapHeight: number): number => {
    let tMin = Infinity;
    if (dx > 0) tMin = Math.min(tMin, (mapWidth - px) / dx);
    else if (dx < 0) tMin = Math.min(tMin, -px / dx);
    if (dy > 0) tMin = Math.min(tMin, (mapHeight - py) / dy);
    else if (dy < 0) tMin = Math.min(tMin, -py / dy);
    return tMin === Infinity ? 0 : Math.max(0, tMin);
};

export const maskToPath = (geojson: Feature<Polygon | MultiPolygon>, mapInstance: maplibregl.Map): string => {
    if (!geojson.geometry) return '';
    const coords = geojson.geometry.type === 'Polygon' ? [geojson.geometry.coordinates] : (geojson.geometry as MultiPolygon).coordinates;
    return coords.map((poly: any) => poly.map((ring: any) => ring.map((coord: any) => {
        const p = mapInstance.project([coord[0], coord[1]]);
        return `${p.x},${p.y} `;
    }).join(' L ')).map((ringStr: string) => `M ${ringStr} Z`).join(' ')).join(' ');
};
