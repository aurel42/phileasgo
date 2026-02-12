import { useEffect, useState, useRef, useMemo } from 'react';
import { Polyline, Marker, useMap } from 'react-leaflet';
import { createPortal } from 'react-dom';
import L from 'leaflet';
import * as d3 from 'd3-force';
import type { TripEvent } from '../hooks/useTripEvents';
import { interpolatePosition, isTransitionEvent, isAirportNearTerminal, type CreditItem } from '../utils/replay';
import { CreditRoll } from './CreditRoll';

interface TripReplayOverlayProps {
    events: TripEvent[];
    durationMs: number; // Total animation duration
    isPlaying?: boolean; // If false, we should stop and clean up
}

// Marker constants (match SmartMarkerLayer)
const MARKER_SIZE = 28;
const MARKER_RADIUS = MARKER_SIZE / 2;
const COLLISION_PADDING = 3; // Reduced for tighter packing
const ANCHOR_STRENGTH = 0.08; // How strongly markers are pulled toward their anchor (lower = smoother/floatier)

// Lifecycle timing (in milliseconds)
const GROW_DURATION = 4000;        // 0-4s: Grow from 0% to 100%
const LIVE_DURATION = 10000;       // 4-14s: Stay at 100%
const SHRINK_DURATION = 2000;      // 14-16s: Shrink from 100% to shrinkTarget

// Dynamic shrink target based on marker count (logarithmic scaling)
const SHRINK_MAX = 1.0;   // 100% - when 1 marker
const SHRINK_MIN = 0.5;   // 50% - when many markers
const SHRINK_LOG_BASE = 64; // Count at which we hit minimum size

// Calculate the dynamic shrink target based on total marker count
const getShrinkTarget = (totalCount: number): number => {
    if (totalCount <= 1) return SHRINK_MAX;
    // Logarithmic scaling: shrinkTarget decreases as count increases
    // At count=1: 100%, at count=64: 20%
    const logProgress = Math.log2(totalCount) / Math.log2(SHRINK_LOG_BASE);
    const target = SHRINK_MAX - logProgress * (SHRINK_MAX - SHRINK_MIN);
    return Math.max(SHRINK_MIN, Math.min(SHRINK_MAX, target));
};

// Color lifecycle timing (in milliseconds)
const COLOR_YELLOW_TO_ORANGE = 4000;   // 0-4s: Yellow to Orange
const COLOR_ORANGE_TO_GREEN = 200;     // 4-4.2s: Orange to Green
const COLOR_GREEN_DURATION = 8000;     // 4.2-12.2s: Green
const COLOR_GREEN_TO_BLUE = 1000;      // 12.2-13.2s: Green to Blue

// Color definitions
const COLORS = {
    yellow: '#FFD700',
    orange: '#FF8C00',
    green: '#22c55e',
    blue: '#3b82f6',
    // Triadic colors (120° apart from blue) for special markers
    magenta: '#F63B82',   // For screenshots
    lime: '#82F63B',      // For essays
};

// Credit Roll timing - when to add a name to the roll (at spawn)
const CREDIT_TRIGGER_AGE = 0; // Trigger immediately when marker appears

// CreditItem moved to ../utils/replay

// Interpolate between two hex colors
const lerpColor = (colorA: string, colorB: string, t: number): string => {
    const parseHex = (hex: string) => {
        const h = hex.replace('#', '');
        return {
            r: parseInt(h.substring(0, 2), 16),
            g: parseInt(h.substring(2, 4), 16),
            b: parseInt(h.substring(4, 6), 16),
        };
    };
    const a = parseHex(colorA);
    const b = parseHex(colorB);
    const r = Math.round(a.r + (b.r - a.r) * t);
    const g = Math.round(a.g + (b.g - a.g) * t);
    const bl = Math.round(a.b + (b.b - a.b) * t);
    return `rgb(${r}, ${g}, ${bl})`;
};

// Calculate scale based on age and dynamic shrink target
const getScale = (ageMs: number, shrinkTarget: number): number => {
    if (ageMs < GROW_DURATION) {
        // Growing phase: 0% to 100%
        return ageMs / GROW_DURATION;
    } else if (ageMs < GROW_DURATION + LIVE_DURATION) {
        // Living phase: 100%
        return 1;
    } else if (ageMs < GROW_DURATION + LIVE_DURATION + SHRINK_DURATION) {
        // Shrinking phase: 100% to shrinkTarget
        const shrinkProgress = (ageMs - GROW_DURATION - LIVE_DURATION) / SHRINK_DURATION;
        return 1 - (1 - shrinkTarget) * shrinkProgress;
    } else {
        // Final state: shrinkTarget
        return shrinkTarget;
    }
};

// Calculate color based on age
const getColor = (ageMs: number): string => {
    const t1 = COLOR_YELLOW_TO_ORANGE;
    const t2 = t1 + COLOR_ORANGE_TO_GREEN;
    const t3 = t2 + COLOR_GREEN_DURATION;
    const t4 = t3 + COLOR_GREEN_TO_BLUE;

    if (ageMs < t1) {
        // Yellow to Orange
        return lerpColor(COLORS.yellow, COLORS.orange, ageMs / t1);
    } else if (ageMs < t2) {
        // Orange to Green
        return lerpColor(COLORS.orange, COLORS.green, (ageMs - t1) / COLOR_ORANGE_TO_GREEN);
    } else if (ageMs < t3) {
        // Green
        return COLORS.green;
    } else if (ageMs < t4) {
        // Green to Blue
        return lerpColor(COLORS.green, COLORS.blue, (ageMs - t3) / COLOR_GREEN_TO_BLUE);
    } else {
        // Blue
        return COLORS.blue;
    }
};

interface ReplayNode extends d3.SimulationNodeDatum {
    id: string;
    lat: number;
    lon: number;
    icon: string;
    anchorX: number;
    anchorY: number;
    x: number;
    y: number;
    r: number;
    isTrackPoint?: boolean;
    scale?: number; // 0-1 for size animation
    color?: string; // Current color based on lifecycle
    birthTime?: number; // When this marker appeared (for lifecycle calc)
}

// Small plane SVG for the animated tip
const miniPlaneSvg = `
<svg viewBox="0 0 512 512" width="24" height="24" style="display: block; filter: drop-shadow(0px 0px 2px rgba(0,0,0,0.7));">
    <path fill="#FFD700" stroke="black" stroke-width="12" d="M256 32 C 240 32, 230 50, 230 70 L 230 160 L 32 190 L 32 230 L 230 250 L 230 380 L 130 420 L 130 460 L 256 440 L 382 460 L 382 420 L 282 380 L 282 250 L 480 230 L 480 190 L 282 160 L 282 70 C 282 50, 272 32, 256 32 Z" />
</svg>
`;

// Interpolate position along a polyline based on progress (0-1)
// Deleted duplication here as it is now imported from ../utils/replay

// isTransitionEvent and isAirportNearTerminal moved to ../utils/replay

// interpolatePosition moved to ../utils/replay

// CreditRoll moved to ./CreditRoll.tsx

// Smart POI marker component with tether line, lifecycle animation
const SmartReplayMarker = ({ node }: { node: ReplayNode }) => {
    const iconPath = `/icons/${node.icon}.svg`;
    const scale = node.scale ?? 1;
    const color = node.color ?? COLORS.green;

    // Calculate tether line (from anchor to current position)
    const hasOffset = Math.abs(node.x - node.anchorX) > 3 || Math.abs(node.y - node.anchorY) > 3;

    // Scale the marker size
    const scaledSize = MARKER_SIZE * scale;
    const scaledRadius = scaledSize / 2;

    return (
        <>
            {/* Tether line from anchor to marker */}
            {hasOffset && scale > 0.1 && (
                <svg
                    style={{
                        position: 'absolute',
                        left: 0,
                        top: 0,
                        width: '100%',
                        height: '100%',
                        pointerEvents: 'none',
                        overflow: 'visible',
                    }}
                >
                    <line
                        x1={node.anchorX}
                        y1={node.anchorY}
                        x2={node.x}
                        y2={node.y}
                        stroke={`${color}80`} // Use marker color with 50% opacity
                        strokeWidth="1.5"
                        strokeDasharray="4,2"
                    />
                </svg>
            )}
            {/* Marker with lifecycle animation (physics-driven scale, dynamic color) */}
            {scale > 0 && (
                <div
                    className="replay-smart-marker"
                    style={{
                        position: 'absolute',
                        left: 0,
                        top: 0,
                        transform: `translate3d(${node.x - scaledRadius}px, ${node.y - scaledRadius}px, 0)`,
                        width: scaledSize,
                        height: scaledSize,
                        pointerEvents: 'none',
                        transition: 'width 0.3s, height 0.3s, transform 0.3s', // Smoother movement
                    }}
                >
                    <div style={{
                        position: 'relative',
                        backgroundColor: color,
                        border: `2px solid ${color}`,
                        width: '100%',
                        height: '100%',
                        boxShadow: '0 2px 4px rgba(0, 0, 0, 0.5)',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        borderRadius: '50%',
                        transition: 'background-color 0.15s, border-color 0.15s',
                    }}>
                        <img src={iconPath} style={{ width: `${18 * scale}px`, height: `${18 * scale}px`, opacity: Math.min(1, scale * 2) }} draggable={false} />
                    </div>
                </div>
            )}
        </>
    );
};

export const TripReplayOverlay = ({ events, durationMs, isPlaying }: TripReplayOverlayProps) => {
    const map = useMap();
    const [progress, setProgress] = useState(0);
    const startTimeRef = useRef<number | null>(null);
    const animationRef = useRef<number | null>(null);

    // Note: We previously had a cleanup effect that called map.stop() here,
    // but it caused errors when the map was already being unmounted.
    // Leaflet handles animation cleanup automatically when the map is removed.

    // Filter events with valid coordinates
    const validEvents = useMemo(() => {
        return events;
    }, [events]);

    // Extract path as [lat, lon] tuples
    const path = useMemo((): [number, number][] => {
        return validEvents.map(e => [e.lat, e.lon] as [number, number]);
    }, [validEvents]);

    // Identify departure/destination (for bounding box)
    const { departure, destination } = useMemo(() => {
        const dep = validEvents.find(e => isTransitionEvent(e.type) && e.title?.toLowerCase().includes('take-off'));
        const dest = validEvents.slice().reverse().find(e => isTransitionEvent(e.type) && e.title?.toLowerCase().includes('landed'));

        // Fallback to first/last path points if no transition events
        const depCoords = dep ? [dep.lat, dep.lon] as [number, number]
            : (path.length > 0 ? path[0] : null);
        const destCoords = dest ? [dest.lat, dest.lon] as [number, number]
            : (path.length > 1 ? path[path.length - 1] : null);

        return {
            departure: depCoords,
            destination: destCoords,
        };
    }, [validEvents, path]);

    const [panesReady, setPanesReady] = useState(false);

    // Fit map to static bounding box on mount (departure to destination)
    // No dynamic zooming during animation - keeps map stable throughout playback
    useEffect(() => {
        if (path.length < 2) return;

        const bounds = L.latLngBounds(path);
        if (departure) bounds.extend(departure);
        if (destination) bounds.extend(destination);

        map.fitBounds(bounds, { padding: [50, 50], animate: false });

        if (!map.getPane('trailPane')) {
            const pane = map.createPane('trailPane');
            pane.style.zIndex = '610';
            pane.style.pointerEvents = 'none';
        }
        if (!map.getPane('terminalPane')) {
            const pane = map.createPane('terminalPane');
            pane.style.zIndex = '615';
            pane.style.pointerEvents = 'none';
        }
        if (!map.getPane('replayPlanePane')) {
            const pane = map.createPane('replayPlanePane');
            pane.style.zIndex = '620';
            pane.style.pointerEvents = 'none';
        }
        setPanesReady(true);
    }, [map, path, departure, destination]);

    // Animation loop - continues for a cooldown period after trip ends to let POI lifecycle complete
    const LIFECYCLE_COOLDOWN_MS = 16000; // 16s = grow(4) + live(10) + shrink(2)
    // Use state for current time to drive animation cleanly and strictly
    const [currentTime, setCurrentTime] = useState(Date.now());

    // Credit roll state - track POI names as they turn green
    const [credits, setCredits] = useState<CreditItem[]>([]);
    const creditedIdsRef = useRef<Set<string>>(new Set()); // Track which POIs have already been credited

    useEffect(() => {
        if (path.length < 2 || !isPlaying) return;

        startTimeRef.current = Date.now();

        const animate = () => {
            if (!startTimeRef.current) return;
            const now = Date.now();
            const elapsed = now - startTimeRef.current;
            setCurrentTime(now); // Drive the render loop with time
            const p = Math.min(1, elapsed / durationMs);
            setProgress(p);

            // Force re-render for lifecycle animations even after progress hits 1
            // setTick(t => t + 1); // Replaced by setCurrentTime

            // Continue animation until cooldown period after trip ends
            const tripEndTime = durationMs;
            const cooldownEnd = tripEndTime + LIFECYCLE_COOLDOWN_MS;

            if (elapsed < cooldownEnd) {
                animationRef.current = requestAnimationFrame(animate);
            }
        };

        animationRef.current = requestAnimationFrame(animate);
        return () => {
            if (animationRef.current) {
                cancelAnimationFrame(animationRef.current);
            }
        };
    }, [path.length, durationMs, isPlaying]);

    // Guarded execution to prevent conditional hooks
    // If path is invalid, we process dummy values but hooks still run
    const isValidPath = path.length >= 2;

    let position: [number, number] = isValidPath ? path[0] : [0, 0];
    let heading = 0;
    let segmentIndex = 0;

    if (isValidPath) {
        const res = interpolatePosition(path, progress);
        position = res.position;
        heading = res.heading;
        segmentIndex = res.segmentIndex;
    }

    // Path drawn so far
    const drawnPath = isValidPath ? path.slice(0, segmentIndex + 2).map((p, i) => {
        if (i === segmentIndex + 1) return position; // Use interpolated tip
        return p;
    }) : [];

    // POIs to show (events up to current segment, plus anticipation)
    // POIs to show (events up to current segment, plus anticipation)
    const visiblePOIs = isValidPath ? validEvents.slice(0, segmentIndex + 2) : [];

    // Track birth times for each POI (when it first appeared)
    const birthTimesRef = useRef<Map<string, number>>(new Map());
    const totalPOIs = useMemo(() => validEvents.filter(e => !isTransitionEvent(e.type)).length, [validEvents]);
    const shrinkTarget = useMemo(() => getShrinkTarget(totalPOIs), [totalPOIs]);

    // Compute smart POI layout using d3-force with lifecycle-aware growth/shrink/color
    // Compute smart POI layout using d3-force with lifecycle-aware growth/shrink/color
    const poiNodes = useMemo(() => {
        // Fix usage of Date.now() with stable state time
        const now = currentTime;

        const poiNodeList = (visiblePOIs
            .map((poi): ReplayNode | null => {
                const globalIndex = validEvents.indexOf(poi);
                if (isTransitionEvent(poi.type) || isAirportNearTerminal(poi, departure, destination)) {
                    return null;
                }

                const lat = poi.metadata?.poi_lat ? parseFloat(poi.metadata.poi_lat) : poi.lat;
                const lon = poi.metadata?.poi_lon ? parseFloat(poi.metadata.poi_lon) : poi.lon;
                const projected = map.latLngToLayerPoint([lat, lon]);
                const icon = poi.metadata?.icon && poi.metadata.icon.length > 0 ? poi.metadata.icon : 'attraction';
                const nodeId = `poi-${globalIndex}`;

                // Track birth time - first time we see this marker
                if (!birthTimesRef.current.has(nodeId)) {
                    birthTimesRef.current.set(nodeId, now);
                }
                const birthTime = birthTimesRef.current.get(nodeId)!;

                // Calculate age and use lifecycle functions for scale and color
                const age = now - birthTime;
                const scale = getScale(age, shrinkTarget);
                // Screenshots and essays get fixed triadic colors
                let color: string;
                if (poi.category === 'screenshot') {
                    color = COLORS.magenta;
                } else if (poi.category === 'essay') {
                    color = COLORS.lime;
                } else {
                    color = getColor(age);
                }

                // Physics radius is proportional to scale
                const physicsRadius = MARKER_RADIUS * scale;

                return {
                    id: nodeId,
                    lat,
                    lon,
                    icon,
                    anchorX: projected.x,
                    anchorY: projected.y,
                    x: projected.x + (Math.sin(globalIndex) * 1),
                    y: projected.y + (Math.cos(globalIndex) * 1),
                    r: physicsRadius,
                    isTrackPoint: false,
                    scale,
                    color,
                    birthTime,
                };
            })
            .filter((node) => node !== null) as ReplayNode[]);

        if (poiNodeList.length === 0) return [];

        const trackNodes: ReplayNode[] = [];
        // Removed track and plane repulsion to prevent "nervous" markers move away from the path.
        // Keeping only terminii repulsion.

        // Add departure airport as repulsion point
        if (departure) {
            const depProjected = map.latLngToLayerPoint(departure);
            trackNodes.push({
                id: 'departure',
                lat: departure[0],
                lon: departure[1],
                icon: '',
                anchorX: depProjected.x,
                anchorY: depProjected.y,
                x: depProjected.x,
                y: depProjected.y,
                r: MARKER_RADIUS + 5,
                isTrackPoint: true,
                fx: depProjected.x,
                fy: depProjected.y,
            });
        }

        // Add destination airport as repulsion point
        if (destination) {
            const destProjected = map.latLngToLayerPoint(destination);
            trackNodes.push({
                id: 'destination',
                lat: destination[0],
                lon: destination[1],
                icon: '',
                anchorX: destProjected.x,
                anchorY: destProjected.y,
                x: destProjected.x,
                y: destProjected.y,
                r: MARKER_RADIUS + 5,
                isTrackPoint: true,
                fx: destProjected.x,
                fy: destProjected.y,
            });
        }

        const allNodes = [...poiNodeList, ...trackNodes];

        // Run d3-force simulation - markers with larger scale push harder
        const simulation = d3.forceSimulation<ReplayNode>(allNodes)
            .force('collide', d3.forceCollide<ReplayNode>().radius(d => d.r + COLLISION_PADDING).strength(0.8))
            .force('x', d3.forceX<ReplayNode>(d => d.anchorX).strength(d => d.isTrackPoint ? 0 : ANCHOR_STRENGTH))
            .force('y', d3.forceY<ReplayNode>(d => d.anchorY).strength(d => d.isTrackPoint ? 0 : ANCHOR_STRENGTH))
            .stop();

        for (let i = 0; i < 100; ++i) { // Reduced from 500 to 100 for performance - enough for incremental updates
            simulation.tick();
        }

        // Return only non-track nodes (the ones we want to render)
        // Return only non-track nodes (the ones we want to render)
        return allNodes.filter(n => !n.isTrackPoint);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [visiblePOIs, map, drawnPath, position, currentTime, departure, destination, shrinkTarget]); // currentTime forces recalc for lifecycle animation

    // Credit roll trigger - check for POIs hitting the green phase
    // Credit roll trigger - check for POIs hitting the green phase
    useEffect(() => {
        const now = currentTime;
        const newCredits: CreditItem[] = [];

        visiblePOIs.forEach((poi) => {
            // Only credit narration-type events
            if (poi.type !== 'narration' || isAirportNearTerminal(poi, departure, destination)) return;

            const globalIndex = validEvents.indexOf(poi);
            const nodeId = `poi-${globalIndex}`;
            const birthTime = birthTimesRef.current.get(nodeId);
            if (!birthTime) return;

            const age = now - birthTime;
            let name = poi.title || poi.metadata?.name || 'Unknown';
            // Strip "Briefing: " prefix if present
            if (name.startsWith('Briefing: ')) {
                name = name.replace('Briefing: ', '');
            }

            // Check if this POI just crossed the green threshold and hasn't been credited yet
            if (age >= CREDIT_TRIGGER_AGE && !creditedIdsRef.current.has(nodeId)) {
                creditedIdsRef.current.add(nodeId);
                newCredits.push({
                    id: nodeId,
                    name,
                    addedAt: now,
                });
            }
        });

        if (newCredits.length > 0) {
            setCredits(prev => [...prev, ...newCredits]);
        }
        if (newCredits.length > 0) {
            setCredits(prev => [...prev, ...newCredits]);
        }
    }, [visiblePOIs, currentTime]); // currentTime ensures we check on each animation frame

    // Total POI count for adaptive credit roll speed
    const totalPOICount = validEvents.filter(e => !isTransitionEvent(e.type)).length;

    // Plane icon
    const planeIcon = L.divIcon({
        className: 'replay-plane',
        html: `<div style="transform: rotate(${heading}deg); transform-origin: center;">${miniPlaneSvg}</div>`,
        iconSize: [24, 24],
        iconAnchor: [12, 12],
    });

    // Aerodrome icons - full size airport markers
    const createAirportIcon = (color: string, borderColor: string) => L.divIcon({
        className: 'aerodrome-marker',
        html: `<div style="
            width: ${MARKER_SIZE}px;
            height: ${MARKER_SIZE}px;
            background: ${color};
            border: 2px solid ${borderColor};
            border-radius: 50%;
            box-shadow: 0 2px 4px rgba(0,0,0,0.5);
            display: flex;
            align-items: center;
            justify-content: center;
        "><img src="/icons/airfield.svg" style="width: 18px; height: 18px;" /></div>`,
        iconSize: [MARKER_SIZE, MARKER_SIZE],
        iconAnchor: [MARKER_RADIUS, MARKER_RADIUS],
    });

    const departureIcon = createAirportIcon('#4CAF50', '#4CAF50'); // Green
    const destinationIcon = createAirportIcon('#FFD700', '#333');   // Gold

    // Get custom panes (fallback to markerPane just for type safety, but guarded by panesReady)
    const markerPane = map.getPanes().markerPane;

    if (!isValidPath) return null; // Final render guard

    return (
        <>
            {/* Route line - double polyline for parchment outline effect */}
            {panesReady && (
                <>
                    <Polyline
                        positions={drawnPath}
                        pathOptions={{
                            color: '#FCF5E5', // Parchment white
                            weight: 10,       // Outer width
                            opacity: 1,
                            dashArray: '10, 5',
                            pane: 'trailPane',
                        }}
                        interactive={false}
                    />
                    <Polyline
                        positions={drawnPath}
                        pathOptions={{
                            color: 'crimson', // Inner core
                            weight: 6,        // Inner width
                            opacity: 0.9,
                            dashArray: '10, 5',
                            pane: 'trailPane',
                        }}
                        interactive={false}
                    />
                </>
            )}

            {/* Departure marker */}
            {departure && panesReady && <Marker position={departure} icon={departureIcon} interactive={false} pane="terminalPane" />}

            {/* Destination marker (always visible) */}
            {destination && panesReady && <Marker position={destination} icon={destinationIcon} interactive={false} pane="terminalPane" />}

            {/* Smart POI markers - rendered via portal to default marker pane */}
            {panesReady && markerPane && createPortal(
                <div className="replay-marker-layer" style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    pointerEvents: 'none',
                }}>
                    {poiNodes.map((node) => (
                        <SmartReplayMarker key={node.id} node={node} />
                    ))}
                </div>,
                markerPane
            )}


            {/* Animated plane at tip - highest z-level */}
            {panesReady && <Marker position={position} icon={planeIcon} interactive={false} pane="replayPlanePane" />}

            {/* Credit Roll - POI names scrolling up (rendered to body for viewport positioning) */}
            {createPortal(
                <CreditRoll items={credits} totalPOICount={totalPOICount} mapContainer={map.getContainer()} currentTime={currentTime} />,
                document.body
            )}
        </>
    );
};
