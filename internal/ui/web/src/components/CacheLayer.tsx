import { useEffect, useState } from 'react';
import { Circle, useMap } from 'react-leaflet';

interface CacheTile {
    lat: number;
    lon: number;
    radius?: number; // Optional, in meters
}

export const CacheLayer = () => {
    const map = useMap();
    const [tiles, setTiles] = useState<CacheTile[]>([]);

    useEffect(() => {
        const fetchTiles = () => {
            const center = map.getCenter();
            fetch(`/api/wikidata/cache?lat=${center.lat}&lon=${center.lng}`)
                .then(r => {
                    if (r.ok) return r.json();
                    return [];
                })
                .then(data => setTiles(data || []))
                .catch(e => console.error("Failed to fetch cache layer", e));
        };

        fetchTiles();
        const interval = setInterval(fetchTiles, 5000); // Poll every 5s

        return () => clearInterval(interval);
    }, [map]);

    return (
        <>
            {tiles.map((t, i) => (
                <Circle
                    key={i}
                    center={[t.lat, t.lon]}
                    radius={t.radius || 9800} // CacheTile radius is in meters
                    pathOptions={{
                        color: 'white',
                        fillColor: 'white',
                        fillOpacity: 0.075,
                        stroke: false,
                        interactive: false,
                    }}
                />
            ))}
        </>
    );

};
