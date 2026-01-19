
import { useState, useMemo } from 'react';
import { createPortal } from 'react-dom';
import { useMap, useMapEvents } from 'react-leaflet';
import * as d3 from 'd3-force';
// ... existing imports ...
import type { POI } from '../hooks/usePOIs';
// Removed unused isPOIVisible import

interface SmartMarkerLayerProps {
    pois: POI[];
    selectedPOI: POI | null;
    currentNarratedId?: string;
    preparingId?: string;
    onPOISelect: (poi: POI) => void;
}

interface SimulationNode extends d3.SimulationNodeDatum {
    id: string; // wikidata_id
    poi: POI;
    anchorX: number; // Target X (Layer Point)
    anchorY: number; // Target Y (Layer Point)
    x: number; // Current X (simulated)
    y: number; // Current Y (simulated)
    r: number; // Radius
    priority: number; // Z-Index / Collision priority
}

// Marker Size Constants
const MARKER_SIZE = 32;
const MARKER_RADIUS = MARKER_SIZE / 2;
const COLLISION_PADDING = 5;

// Visual Component for valid POI rendering
const SmartMarker = ({ node, onClick }: { node: SimulationNode; onClick: (p: POI) => void }) => {
    const { poi, priority, x, y } = node;

    // Icon Logic (Same as POIMarker)
    const iconName = poi.icon && poi.icon.length > 0 ? poi.icon : 'attraction';
    const iconPath = `/icons/${iconName}.svg`;

    // Color Logic
    const getColor = (score: number) => {
        const clamped = Math.max(1, Math.min(50, score));
        const ratio = (clamped - 1) / 49;
        const hue = 60 - (ratio * 60);
        return `hsl(${hue}, 100%, 50%)`;
    };

    let bgColor = getColor(poi.score);
    let scale = 1.0;
    // Base Z by latitude to fix minor overlaps locally (North > South vs South > North depending on leaflet, but consistent offset is key)
    // Leaflet typically uses Y coord, so we just add a small variation.
    const baseLatZ = Math.floor(poi.lat * 100);

    // Active/Playing status logic
    const isHighlighted = priority === 2000;
    const isPreparing = priority === 1500;
    const isMSFS = poi.is_msfs_poi;
    const isPlayed = poi.last_played && poi.last_played !== "0001-01-01T00:00:00Z";

    let zIndex = 0;

    if (isHighlighted) {
        bgColor = '#22c55e'; // Green
        scale = 1.5;
        zIndex = 80000 + baseLatZ;
    } else if (isPreparing) {
        bgColor = '#15803d'; // Darker Green (Green-700)
        scale = 1.3;
        zIndex = 60000 + baseLatZ;
    } else if (isPlayed) {
        bgColor = '#3b82f6'; // Blue - played POIs are always blue
        scale = 0.6;
        zIndex = 0 + baseLatZ; // Bottom tier
    } else if (isMSFS) {
        zIndex = 40000 + baseLatZ;
    } else {
        zIndex = 20000 + baseLatZ; // Standard Unplayed
    }

    const starBadge = isMSFS ? (
        <div style={{
            position: 'absolute',
            top: '-6px',
            right: '-6px',
            color: '#fbbf24',
            filter: 'drop-shadow(0 1px 1px rgba(0,0,0,0.5))',
            zIndex: 10,
            fontSize: '16px',
            lineHeight: 1,
        }}>★</div>
    ) : null;

    return (
        <div
            className={`smart-marker ${isHighlighted ? 'highlighted' : ''} ${isPreparing ? 'preparing' : ''}`}
            onClick={(e) => {
                e.stopPropagation(); // Prevent map click
                onClick(poi);
            }}
            style={{
                position: 'absolute',
                left: 0,
                top: 0,
                transform: `translate3d(${x - MARKER_RADIUS}px, ${y - MARKER_RADIUS}px, 0) scale(${scale})`, // Centered
                width: MARKER_SIZE,
                height: MARKER_SIZE,
                zIndex: zIndex,
                cursor: 'pointer',
                pointerEvents: 'auto', // Re-enable clicks
                transition: 'transform 0.1s linear, background-color 0.3s ease', // Smooth out frame jitters
            }}
        >
            <div style={{
                position: 'relative',
                backgroundColor: bgColor,
                border: `2px solid ${bgColor}`,
                width: '100%',
                height: '100%',
                boxShadow: '0 2px 4px rgba(0, 0, 0, 0.5)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                borderRadius: '4px', // Standard leaflet box look? Or circle? Leaflet default is usually images. We use boxes in POIMarker.
            }}>
                <img src={iconPath} style={{ width: '24px', height: '24px' }} draggable={false} />
                {starBadge}
            </div>
        </div>
    );
};

export const SmartMarkerLayer = ({ pois, selectedPOI, currentNarratedId, preparingId, onPOISelect }: SmartMarkerLayerProps) => {
    const map = useMap();
    const [isZooming, setIsZooming] = useState(false);
    const [mapVersion, setMapVersion] = useState(0); // Force recalc on map move

    useMapEvents({
        zoomstart: () => setIsZooming(true),
        zoomend: () => {
            setIsZooming(false);
            setMapVersion(v => v + 1); // Trigger recalc after zoom
        },
        moveend: () => setMapVersion(v => v + 1), // Trigger recalc after pan
    });

    // Use all POIs returned by API (API is source of truth)
    const visiblePOIs = pois;

    // Compute layout SYNCHRONOUSLY using D3 force simulation (no animation)
    const nodes = useMemo(() => {
        // Create nodes with projected positions
        const newNodes: SimulationNode[] = visiblePOIs.map(p => {
            const projected = map.latLngToLayerPoint([p.lat, p.lon]);

            // Priority & Scale Logic
            let priority = 0;
            let scale = 1.0;

            const isNarrated = p.wikidata_id === currentNarratedId || p.wikidata_id === selectedPOI?.wikidata_id;
            const isPreparing = p.wikidata_id === preparingId;
            const isPlayed = p.last_played && p.last_played !== "0001-01-01T00:00:00Z";

            if (isNarrated) {
                priority = 2000;
                scale = 1.5;
            } else if (isPreparing) {
                priority = 1500;
                scale = 1.3;
            } else if (isPlayed) {
                // Played POIs get lower priority but are smaller
                if (p.is_msfs_poi) priority = 1000; // MSFS still stays on top of generic played
                else priority = 500; // Generic played pushed down
                scale = 0.6;
            } else if (p.is_msfs_poi) {
                priority = 1000;
            }

            return {
                id: p.wikidata_id,
                poi: p,
                anchorX: projected.x,
                anchorY: projected.y,
                x: projected.x,
                y: projected.y,
                r: MARKER_RADIUS * scale, // Actual physics radius
                scale: scale, // Pass scale to renderer to ensure exact match
                priority: priority,
            };
        });

        if (newNodes.length === 0) return [];

        // Create a fresh simulation and run it to completion synchronously
        const simulation = d3.forceSimulation<SimulationNode>(newNodes)
            .force('collide', d3.forceCollide<SimulationNode>().radius(d => d.r + COLLISION_PADDING).strength(1.0)) // Hard collision (strength 1.0)
            .force('x', d3.forceX<SimulationNode>(d => d.anchorX).strength(0.1)) // Softer anchor pull to prioritize separation
            .force('y', d3.forceY<SimulationNode>(d => d.anchorY).strength(0.1))
            .stop(); // Don't auto-start

        // Run simulation to completion (more iterations for stability)
        for (let i = 0; i < 300; ++i) {
            simulation.tick();
        }

        return newNodes;
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [visiblePOIs, currentNarratedId, selectedPOI?.wikidata_id, mapVersion]); // mapVersion forces recalc on map move



    // Hide during zoom to prevent artifacts
    if (isZooming) return null;

    // We render into the overlayPane so we move with the map (hardware accelerated panning)
    // and can use stable LayerPoints for simulation.
    const overlayPane = map.getPanes().overlayPane;

    // Guard: If overlayPane isn't ready (e.g., during initial render), don't render
    if (!overlayPane) return null;

    return createPortal(
        <div className="smart-marker-layer" style={{
            position: 'absolute',
            top: 0,
            left: 0,
            zIndex: 600, // Leaflet marker pane is usually 600
            pointerEvents: 'none', // Allow map interaction
        }}>
            <svg style={{
                position: 'absolute',
                left: 0, top: 0,
                overflow: 'visible',
                pointerEvents: 'none',
                // Explicitly disable pointer events on the SVG container to let clicks pass through to map
                // Individual elements (lines, circles) can remain non-interactive or receive specific styling
            }}>
                {nodes.filter(n => {
                    const dx = n.x - n.anchorX;
                    const dy = n.y - n.anchorY;
                    return Math.sqrt(dx * dx + dy * dy) > 10;
                }).map(n => (
                    <line
                        key={`line-${n.id}`}
                        x1={n.anchorX} y1={n.anchorY}
                        x2={n.x} y2={n.y}
                        stroke="rgba(255, 255, 255, 0.6)"
                        strokeWidth={1.5}
                    />
                ))}
                {nodes.map(n => (
                    <circle
                        key={`dot-${n.id}`}
                        cx={n.anchorX} cy={n.anchorY} r={2}
                        fill="rgba(255, 255, 255, 0.4)"
                    />
                ))}
            </svg>

            {nodes.map(node => (
                <SmartMarker key={node.id} node={node} onClick={onPOISelect} />
            ))}
        </div>,
        overlayPane
    );
};
