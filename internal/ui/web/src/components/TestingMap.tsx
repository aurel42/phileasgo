import React, { useEffect, useRef } from 'react';
import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';

export const TestingMap: React.FC = () => {
    const mapContainer = useRef<HTMLDivElement>(null);
    const map = useRef<maplibregl.Map | null>(null);
    const [zoomLevel, setZoomLevel] = React.useState<number>(9);

    useEffect(() => {
        if (map.current || !mapContainer.current) return;

        map.current = new maplibregl.Map({
            container: mapContainer.current,
            style: {
                version: 8,
                sources: {
                    'stamen-watercolor': {
                        type: 'raster',
                        tiles: ['https://watercolormaps.collection.cooperhewitt.org/tile/watercolor/{z}/{x}/{y}.jpg'],
                        tileSize: 256
                    }
                },
                layers: [
                    { id: 'background', type: 'background', paint: { 'background-color': '#f4ecd8' } },
                    {
                        id: 'watercolor',
                        type: 'raster',
                        source: 'stamen-watercolor',
                        paint: { 'raster-saturation': -0.2, 'raster-contrast': 0.1 }
                    }
                ]
            },
            center: [-121.8947, 36.6002], // Monterey, CA
            zoom: 9,
            minZoom: 0,
            maxZoom: 20,
            zoomSnap: 1,
            attributionControl: false
        });

        const onZoom = () => {
            if (map.current) {
                setZoomLevel(map.current.getZoom());
            }
        };
        map.current.on('zoom', onZoom);

        const resizeMap = () => map.current?.resize();
        window.addEventListener('resize', resizeMap);
        resizeMap();

        return () => {
            window.removeEventListener('resize', resizeMap);
            map.current?.off('zoom', onZoom);
            map.current?.remove();
            map.current = null;
        };
    }, []);

    return (
        <div style={{
            display: 'flex',
            flexDirection: 'column',
            width: '100vw',
            height: '100vh',
            background: '#060606',
            overflow: 'hidden'
        }}>
            <div style={{ height: '50vh', width: '100%', position: 'relative' }}>
                <div ref={mapContainer} style={{ width: '100%', height: '100%' }} />
                <div style={{
                    position: 'absolute',
                    top: '12px',
                    left: '12px',
                    background: 'rgba(0, 0, 0, 0.7)',
                    color: '#D4AF37',
                    padding: '4px 8px',
                    borderRadius: '4px',
                    fontFamily: 'monospace',
                    fontSize: '14px',
                    fontWeight: 'bold',
                    border: '1px solid #D4AF37',
                    zIndex: 1000
                }}>
                    ZOOM: {zoomLevel.toFixed(0)}
                </div>
            </div>
            <div style={{
                height: '50vh',
                width: '100%',
                borderTop: '1px solid rgba(255, 255, 255, 0.1)',
                background: '#1a1a1a',
                flexShrink: 0
            }} />
        </div>
    );
};
