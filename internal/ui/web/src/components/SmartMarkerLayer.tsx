
import { useEffect, useState, useRef, useMemo } from 'react';
import { createPortal } from 'react-dom';
import { useMap, useMapEvents } from 'react-leaflet';
import * as d3 from 'd3-force';
import type { POI } from '../hooks/usePOIs';
import { isPOIVisible } from '../utils/poiUtils';

interface SmartMarkerLayerProps {
    pois: POI[];
    minPoiScore: number;
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
    let zIndex = 100 + Math.floor(poi.lat * 100); // Base z-Index by latitude to fix minor overlaps

    // Active/Playing status logic
    const isHighlighted = priority === 2000;
    const isMSFS = poi.is_msfs_poi;
    const isPlayed = poi.last_played && poi.last_played !== "0001-01-01T00:00:00Z";

    if (isHighlighted) {
        bgColor = '#22c55e'; // Green
        scale = 1.5;
        zIndex = 2000;
    } else if (isMSFS) {
        // MSFS badge logic handled by overlay, but maybe boost scale?
        zIndex = 1000;
    } else if (isPlayed) {
        bgColor = '#3b82f6'; // Blue
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
            className={`smart-marker ${isHighlighted ? 'highlighted' : ''}`}
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

export const SmartMarkerLayer = ({ pois, minPoiScore, selectedPOI, currentNarratedId, preparingId, onPOISelect }: SmartMarkerLayerProps) => {
    const map = useMap();
    const [nodes, setNodes] = useState<SimulationNode[]>([]);
    const [isZooming, setIsZooming] = useState(false);

    // Safe ref to track simulation instance
    const simRef = useRef<d3.Simulation<SimulationNode, undefined>>(null);

    // Filter visible POIs (Logic copied/adapted from Map.tsx)
    const visiblePOIs = useMemo(() => {
        return pois.filter(p => isPOIVisible(p, minPoiScore) || p.wikidata_id === currentNarratedId || p.wikidata_id === preparingId);
    }, [pois, minPoiScore, currentNarratedId, preparingId]);

    // Handle Map Events
    useMapEvents({
        zoomstart: () => setIsZooming(true),
        zoomend: () => setIsZooming(false),
        moveend: () => {
            // Re-project everyone on move end if needed
            // With Leaflet L.DomUtil.setPosition style, we might need to rely on the parent pane moving.
            // But we are rendering absolute divs. We need to update anchors.
            if (simRef.current) simRef.current.alpha(0.3).restart();
        }
    });

    // 1. Initialize Simulation & Nodes
    useEffect(() => {
        // Project POIs to Nodes
        // Only run this when POI list changes significantly or map state settles?
        // Actually we need to run this on every render if we want reactive updates.
        // But we want to preserve node positions for stability.

        const currentNodes = simRef.current ? simRef.current.nodes() : [];
        const nodeMap = new Map(currentNodes.map(n => [n.id, n]));

        const newNodes: SimulationNode[] = visiblePOIs.map(p => {
            const projected = map.latLngToLayerPoint([p.lat, p.lon]);
            const existing = nodeMap.get(p.wikidata_id);

            // Priority Check
            let priority = 0;
            if (p.wikidata_id === currentNarratedId || p.wikidata_id === selectedPOI?.wikidata_id) priority = 2000;
            else if (p.is_msfs_poi) priority = 1000;

            if (existing) {
                // Update anchor, preserve x/y (momentum)
                existing.anchorX = projected.x;
                existing.anchorY = projected.y;
                existing.priority = priority;
                existing.poi = p; // Update data
                return existing;
            }

            return {
                id: p.wikidata_id,
                poi: p,
                anchorX: projected.x,
                anchorY: projected.y,
                x: projected.x, // Start at anchor
                y: projected.y,
                r: MARKER_RADIUS, // + Padding handled in force
                priority: priority,
            };
        });

        // Setup Simulation
        if (!simRef.current) {
            simRef.current = d3.forceSimulation<SimulationNode>(newNodes)
                .force('collide', d3.forceCollide<SimulationNode>().radius(d => d.r + COLLISION_PADDING).strength(0.8))
                .force('x', d3.forceX<SimulationNode>(d => d.anchorX).strength(0.2)) // Pull to anchor
                .force('y', d3.forceY<SimulationNode>(d => d.anchorY).strength(0.2))
                .alphaDecay(0.05) // Stop relatively quickly
                .on('tick', () => {
                    // Update state to trigger render
                    if (simRef.current) {
                        setNodes([...simRef.current.nodes()]);
                    }
                });
        } else {
            // Hot update
            simRef.current.nodes(newNodes);
            simRef.current.alpha(0.3).restart(); // Re-heat slightly
        }

        // Ensure manual tick triggers initial render


        return () => {
            // Cleanup? Usually d3 sim stops itself via alpha decay.
            // simRef.current?.stop();
        };

    }, [visiblePOIs, map, currentNarratedId, selectedPOI]); // Re-run when list changes OR map reference changes (rare) AND relies on periodic updates for position? 
    // Need to handle Map Move! The anchors change when map moves!

    // Move Handler Update
    useEffect(() => {
        if (!simRef.current) return;

        const updateAnchors = () => {
            const activeNodes = simRef.current!.nodes();
            activeNodes.forEach(n => {
                const p = map.latLngToLayerPoint([n.poi.lat, n.poi.lon]);
                n.anchorX = p.x;
                n.anchorY = p.y;
                // Ideally valid, but if we pan far, x/y become huge.
            });
            simRef.current?.alpha(0.3).restart();
        };

        map.on('move', updateAnchors); // update on drag? Might be expensive. 
        // 'moveend' is safer for performance, but markers will 'drift' visually during drag.
        // If we render inside the Overlay Pane, Leaflet moves the pane. We only need to re-project on Zoom.

        return () => {
            map.off('move', updateAnchors);
        };
    }, [map]);



    // Hide during zoom to prevent artifacts
    if (isZooming) return null;

    // We render into the overlayPane so we move with the map (hardware accelerated panning)
    // and can use stable LayerPoints for simulation.
    const overlayPane = map.getPanes().overlayPane;

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
