import type { StyleSpecification } from 'maplibre-gl';

export const getMapStyle = (): StyleSpecification => ({
    version: 8,
    sources: {
        'stamen-watercolor-hd': {
            type: 'raster',
            // HD Source: 128px tiles (displays Z+1 content at Z)
            tiles: ['https://watercolormaps.collection.cooperhewitt.org/tile/watercolor/{z}/{x}/{y}.jpg'],
            tileSize: 128
        },
        'hillshade-source': {
            type: 'raster-dem',
            tiles: ['https://tiles.stadiamaps.com/data/terrarium/{z}/{x}/{y}.png'],
            tileSize: 256,
            encoding: 'terrarium'
        },
        'openfreemap': {
            type: 'vector',
            url: 'https://tiles.openfreemap.org/planet'
        }
    },
    layers: [
        { id: 'background', type: 'background', paint: { 'background-color': '#f4ecd8' } },
        // HD Layer: Active for Zoom 0-10 (e.g. at Z9, uses Z10 tiles)
        {
            id: 'watercolor', type: 'raster', source: 'stamen-watercolor-hd',
            paint: { 'raster-saturation': -0.2, 'raster-contrast': 0.1 }
        },
        {
            id: 'hillshading',
            type: 'hillshade',
            source: 'hillshade-source',
            maxzoom: 10,
            paint: {
                'hillshade-exaggeration': [
                    'interpolate',
                    ['linear'],
                    ['zoom'],
                    4, 0.0,
                    6, 0.45
                ],
                'hillshade-shadow-color': 'rgba(0, 0, 0, 0.35)',
                'hillshade-accent-color': 'rgba(0, 0, 0, 0.15)'
            }
        },
        {
            id: 'runways-fill',
            type: 'fill',
            source: 'openfreemap',
            'source-layer': 'aeroway',
            minzoom: 8,
            filter: ['all', ["match", ["geometry-type"], ["MultiPolygon", "Polygon"], true, false], ["==", ["get", "class"], "runway"]],
            paint: {
                'fill-color': [
                    'case',
                    ['match', ['get', 'surface'], ['grass', 'dirt', 'earth', 'ground', 'unpaved'], true, false],
                    '#769b58', // Green for unpaved/grass
                    '#707070'  // Grey for paved
                ],
                'fill-opacity': 0.8
            }
        },
        {
            id: 'runways-line',
            type: 'line',
            source: 'openfreemap',
            'source-layer': 'aeroway',
            minzoom: 8,
            filter: ['all', ["match", ["geometry-type"], ["LineString", "MultiLineString"], true, false], ["==", ["get", "class"], "runway"]],
            paint: {
                'line-color': [
                    'case',
                    ['match', ['get', 'surface'], ['grass', 'dirt', 'earth', 'ground', 'unpaved'], true, false],
                    '#769b58', // Green for unpaved/grass
                    '#707070'  // Grey for paved
                ],
                'line-width': 6,
                'line-opacity': 1.0
            }
        }
    ]
});
