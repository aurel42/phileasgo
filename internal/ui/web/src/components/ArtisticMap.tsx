import React, { useEffect, useRef, useState } from 'react';
import maplibregl from 'maplibre-gl';
import * as turf from '@turf/turf';
import type { Feature, Polygon, MultiPolygon } from 'geojson';
import 'maplibre-gl/dist/maplibre-gl.css';
import type { Telemetry } from '../types/telemetry';
import type { POI } from '../hooks/usePOIs';

interface ArtisticMapProps {
    className?: string;
    center: [number, number];
    zoom: number;
    telemetry: Telemetry | null;
    pois: POI[];
    onPOISelect?: (poi: POI) => void;
}

export const ArtisticMap: React.FC<ArtisticMapProps> = ({ className, center, zoom, telemetry, pois, onPOISelect }) => {
    const mapContainer = useRef<HTMLDivElement>(null);
    const map = useRef<maplibregl.Map | null>(null);
    const [styleLoaded, setStyleLoaded] = useState(false);

    useEffect(() => {
        if (map.current || !mapContainer.current) return;

        map.current = new maplibregl.Map({
            container: mapContainer.current,
            style: {
                version: 8,
                sources: {
                    'stamen-watercolor': {
                        type: 'raster',
                        tiles: [
                            'https://watercolormaps.collection.cooperhewitt.org/tile/watercolor/{z}/{x}/{y}.jpg'
                        ],
                        tileSize: 256,
                        attribution: 'Map tiles by Stamen Design, under CC BY 3.0. Data by OpenStreetMap, under CC BY SA.'
                    }
                },
                layers: [
                    {
                        id: 'background',
                        type: 'background',
                        paint: {
                            'background-color': '#f4ecd8' // Fallback parchment color
                        }
                    },
                    {
                        id: 'watercolor',
                        type: 'raster',
                        source: 'stamen-watercolor',
                        minzoom: 0,
                        maxzoom: 22,
                        paint: {
                            'raster-saturation': -0.6,
                            'raster-contrast': 0.1
                        }
                    }
                ]
            },
            center: [center[1], center[0]], // MapLibre takes [lng, lat]
            zoom: zoom,
            attributionControl: false
        });

        map.current.on('load', () => {
            setStyleLoaded(true);
            map.current?.resize();
        });



    }, []);

    // Update view when center changes (keep zoom user-controlled or auto-controlled)
    // Update view when telemetry changes (LATCH to valid data, ignore App.tsx defaults)
    useEffect(() => {
        if (!map.current || !telemetry) return;

        // Prevent jumping to (0,0) if SimConnect glitches
        if (telemetry.Latitude === 0 && telemetry.Longitude === 0) return;

        // Use jumpTo for smooth tracking without animation lag
        // We do NOT use the 'center' prop here because it falls back to Berlin 
        // when telemetry is null, causing the "Jump to Departure" bug.
        map.current.jumpTo({
            center: [telemetry.Longitude, telemetry.Latitude]
        });
    }, [telemetry]); // Ignore 'center' prop to prevent resets

    // Initial zoom set
    useEffect(() => {
        if (map.current && !styleLoaded) {
            map.current.jumpTo({ zoom: zoom });
        }
    }, [zoom, styleLoaded]);

    // Fetch and update visibility mask
    useEffect(() => {
        if (!map.current || !styleLoaded) return;

        const updateMask = async () => {
            if (!map.current) return;
            try {
                const bounds = map.current.getBounds();
                const north = bounds.getNorth();
                const east = bounds.getEast();
                const south = bounds.getSouth();
                const west = bounds.getWest();

                const response = await fetch(`/api/map/visibility-mask?bounds=${north},${east},${south},${west}&resolution=20`);
                if (!response.ok) return;
                const data = await response.json();

                // Create "Fog" by inverting the visibility mask
                // 1. Create a large world polygon
                const world = turf.polygon([[
                    [-180, -90],
                    [180, -90],
                    [180, 90],
                    [-180, 90],
                    [-180, -90]
                ]]);

                let fogGeoJSON: Feature<Polygon | MultiPolygon> = world as Feature<Polygon>;

                if (data && data.geometry && data.geometry.coordinates && data.geometry.coordinates.length > 0) {
                    try {
                        // 2. Subtract the visibility polygon from the world
                        // Verify data is a valid polygon
                        const visibilityPoly = turf.polygon(data.geometry.coordinates);
                        const diff = turf.difference(turf.featureCollection([world, visibilityPoly]));
                        if (diff) {
                            fogGeoJSON = diff;
                        } else {
                            // Visibility covers the world (rare), so no fog.
                            // Set to empty polygon
                            fogGeoJSON = {
                                type: 'Feature',
                                properties: {},
                                geometry: {
                                    type: 'Polygon',
                                    coordinates: []
                                }
                            };
                        }
                    } catch (err) {
                        console.error("Turf difference failed", err);
                    }
                }

                const source = map.current?.getSource('fog-mask') as maplibregl.GeoJSONSource;
                if (source) {
                    source.setData(fogGeoJSON);
                } else {
                    map.current?.addSource('fog-mask', {
                        type: 'geojson',
                        data: fogGeoJSON
                    });

                    // Add the "Fog" layer
                    // This covers the UNEXPLORED areas with a semi-transparent parchment color
                    map.current?.addLayer({
                        id: 'fog-layer',
                        type: 'fill',
                        source: 'fog-mask',
                        paint: {
                            'fill-color': '#f4ecd8', // Parchment color
                            'fill-opacity': 0.85,    // High opacity for unexplored areas
                            'fill-outline-color': 'rgba(0,0,0,0)'
                        }
                    });

                    // Add a subtle border to the revealed area for style
                    map.current?.addLayer({
                        id: 'fog-border',
                        type: 'line',
                        source: 'fog-mask',
                        paint: {
                            'line-color': '#8b4513', // SaddleBrown
                            'line-width': 2,
                            'line-opacity': 0.3,
                            'line-blur': 1
                        }
                    });
                }

                // Auto-Zoom Layer
                // Calculate a bbox around the aircraft based on visibility
                if (data && data.properties && data.properties.radius_nm && map.current) {
                    try {
                        const r = data.properties.radius_nm;
                        // 1 NM = 1/60 degrees latitude roughly
                        const latOffset = (r * 1.2) / 60.0; // 20% padding
                        const lonOffset = latOffset / Math.cos(center[0] * Math.PI / 180.0);

                        const bounds: [number, number, number, number] = [
                            center[1] - lonOffset, center[0] - latOffset,
                            center[1] + lonOffset, center[0] + latOffset
                        ];

                        // Smoothly float to the new zoom level
                        map.current.fitBounds(bounds, {
                            padding: 20,
                            maxZoom: 14, // Don't zoom in too close (pixelated tiles)
                            linear: true,
                            duration: 2000 // Slow drift
                        });
                    } catch (err) {
                        console.error("Autozoom failed", err);
                    }
                }

            } catch (e) {
                console.error("Failed to fetch visibility mask", e);
            }
        };

        updateMask();
        // Poll every 5 seconds
        const interval = setInterval(updateMask, 5000);
        return () => clearInterval(interval);

    }, [styleLoaded]);

    // Marker refs
    const poiMarkersRef = useRef<Map<string, maplibregl.Marker>>(new Map());

    // Bearing Line Source
    useEffect(() => {
        if (!map.current || !styleLoaded) return;

        map.current.addSource('bearing-line', {
            type: 'geojson',
            data: {
                type: 'Feature',
                properties: {},
                geometry: {
                    type: 'LineString',
                    coordinates: []
                }
            }
        });

        map.current.addLayer({
            id: 'bearing-line-layer',
            type: 'line',
            source: 'bearing-line',
            paint: {
                'line-color': '#5c4033', // Dark brown
                'line-width': 2,
                'line-dasharray': [2, 2], // Dashed
                'line-opacity': 0.7
            }
        });
    }, [styleLoaded]);

    // Update Bearing Line
    useEffect(() => {
        if (!map.current || !telemetry) return;

        // Valid heading check

        // We only update if truthy or explicitly 0 but valid.
        // If it jumps to 0 seemingly erroneously, we might want to filter exact 0 if it was previously set.
        // However, 0 is North. Let's assume if lat/lon is 0,0 it's invalid.
        if (telemetry.Latitude === 0 && telemetry.Longitude === 0) return;

        const h = telemetry.Heading;

        const lat1 = telemetry.Latitude * Math.PI / 180.0;
        const lon1 = telemetry.Longitude * Math.PI / 180.0;
        const brng = h * Math.PI / 180.0;

        // Draw line 50NM out
        const d = 50.0;
        const R = 3440.065;

        const lat2 = Math.asin(Math.sin(lat1) * Math.cos(d / R) + Math.cos(lat1) * Math.sin(d / R) * Math.cos(brng));
        const lon2 = lon1 + Math.atan2(Math.sin(brng) * Math.sin(d / R) * Math.cos(lat1), Math.cos(d / R) - Math.sin(lat1) * Math.sin(lat2));

        const lineGeoJSON: Feature<any> = {
            type: 'Feature',
            properties: {},
            geometry: {
                type: 'LineString',
                coordinates: [
                    [telemetry.Longitude, telemetry.Latitude],
                    [lon2 * 180 / Math.PI, lat2 * 180 / Math.PI]
                ]
            }
        };

        const source = map.current.getSource('bearing-line') as maplibregl.GeoJSONSource;
        if (source) {
            source.setData(lineGeoJSON);
        }

    }, [telemetry, styleLoaded]);

    // Update POI Markers
    useEffect(() => {
        if (!map.current) return;

        const markers = poiMarkersRef.current;
        const currentIds = new Set(pois.map(p => p.wikidata_id));

        // Remove old markers
        for (const [id, marker] of markers.entries()) {
            if (!currentIds.has(id)) {
                marker.remove();
                markers.delete(id);
            }
        }

        // Add/Update new markers
        pois.forEach(poi => {
            if (markers.has(poi.wikidata_id)) {
                // Update position if needed (rare for POIs but possible)
                markers.get(poi.wikidata_id)?.setLngLat([poi.lon, poi.lat]);
            } else {
                const el = document.createElement('div');
                el.className = 'artistic-poi-marker';
                el.style.width = '24px';
                el.style.height = '24px';
                el.style.cursor = 'pointer';
                el.innerHTML = `<div style="
                    width: 100%; height: 100%;
                    border: 2px solid #8b4513; 
                    border-radius: 50%; 
                    background: rgba(244, 236, 216, 0.8);
                    display: flex; align-items: center; justify-content: center;
                    font-family: 'IM Fell DW Pica', serif;
                    font-weight: bold;
                    color: #5c4033;
                ">${poi.category === 'natural' ? 'N' : 'P'}</div>`;

                el.onclick = () => onPOISelect?.(poi);

                const marker = new maplibregl.Marker({ element: el, anchor: 'center' })
                    .setLngLat([poi.lon, poi.lat])
                    .addTo(map.current!);

                markers.set(poi.wikidata_id, marker);
            }
        });

    }, [pois, onPOISelect]);

    return (
        <div className={className} style={{ position: 'relative', width: '100%', height: '100%' }}>
            <div ref={mapContainer} style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', color: 'black' }} />
            <div style={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                height: '100%',
                pointerEvents: 'none',
                backgroundColor: '#f4ecd8',
                backgroundImage: 'url(/assets/textures/paper.jpg), radial-gradient(#d4af37 1px, transparent 1px)',
                backgroundSize: 'cover, 20px 20px',
                opacity: 0.15,
                mixBlendMode: 'multiply',
                zIndex: 10
            }} />
        </div>
    );
};
