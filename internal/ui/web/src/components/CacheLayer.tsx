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
            const bounds = map.getBounds();
            const minLat = bounds.getSouth();
            const maxLat = bounds.getNorth();
            const minLon = bounds.getWest();
            const maxLon = bounds.getEast();

            fetch(`/api/wikidata/cache?min_lat=${minLat}&max_lat=${maxLat}&min_lon=${minLon}&max_lon=${maxLon}`)
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
