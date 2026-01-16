import { useEffect, useState } from 'react';
import { Circle } from 'react-leaflet';

interface CachedTile {
    lat: number;
    lon: number;
    radius: number;
}

export const CoverageLayer = () => {
    const [tiles, setTiles] = useState<CachedTile[]>([]);

    useEffect(() => {
        fetch('/api/map/coverage')
            .then(r => {
                if (r.ok) return r.json();
                return [];
            })
            .then(data => setTiles(data || []))
            .catch(e => console.error("Failed to fetch coverage layer", e));
    }, []); // Fetch once on mount

    if (tiles.length === 0) return null;

    return (
        <>
            {tiles.map((t, i) => (
                <Circle
                    key={i}
                    center={[t.lat, t.lon]}
                    radius={t.radius}
                    pathOptions={{
                        color: '#60a5fa', // Blue-400
                        fillColor: '#60a5fa',
                        fillOpacity: 0.2, // More visible than cache layer
                        stroke: false,
                        interactive: false,
                    }}
                />
            ))}
        </>
    );
};
