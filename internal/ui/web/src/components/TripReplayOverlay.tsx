import { useEffect, useState, useRef, useMemo } from 'react';
import { Polyline, Marker, useMap } from 'react-leaflet';
import { createPortal } from 'react-dom';
import L from 'leaflet';
import * as d3 from 'd3-force';
import type { TripEvent } from '../hooks/useTripEvents';

interface TripReplayOverlayProps {
    events: TripEvent[];
    durationMs: number; // Total animation duration
    isPlaying?: boolean; // If false, we should stop and clean up
}

// Marker constants (match SmartMarkerLayer)
const MARKER_SIZE = 28;
const MARKER_RADIUS = MARKER_SIZE / 2;
const COLLISION_PADDING = 3; // Reduced for tighter packing
const TRACK_REPULSION_RADIUS = 2; // Subtle push from track
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

// Credit roll item with timestamp for animation
interface CreditItem {
    id: string;
    name: string;
    addedAt: number; // Timestamp when added to the roll
}

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

// Calculate heading from point A to point B
const calculateHeading = (from: [number, number], to: [number, number]): number => {
    const dLon = (to[1] - from[1]) * Math.PI / 180;
    const lat1 = from[0] * Math.PI / 180;
    const lat2 = to[0] * Math.PI / 180;
    const y = Math.sin(dLon) * Math.cos(lat2);
    const x = Math.cos(lat1) * Math.sin(lat2) - Math.sin(lat1) * Math.cos(lat2) * Math.cos(dLon);
    const bearing = Math.atan2(y, x) * 180 / Math.PI;
    return (bearing + 360) % 360;
};

// Helper to check if a POI is an airport near departure/destination (within 5km)
const isAirportNearTerminal = (poi: TripEvent, departure: [number, number] | null, destination: [number, number] | null): boolean => {
    // Check if this is an airport/aerodrome by icon or category
    const icon = poi.metadata?.icon?.toLowerCase() || '';
    const poiCategory = poi.metadata?.poi_category?.toLowerCase() || '';
    const isAirport = icon === 'airfield' || poiCategory === 'aerodrome';
    if (!isAirport) return false;

    const lat = poi.metadata?.poi_lat ? parseFloat(poi.metadata.poi_lat) : poi.lat;
    const lon = poi.metadata?.poi_lon ? parseFloat(poi.metadata.poi_lon) : poi.lon;
    const threshold = 0.045; // ~5km in degrees

    // Check distance from departure or destination
    if (departure) {
        const dLat = Math.abs(lat - departure[0]);
        const dLon = Math.abs(lon - departure[1]);
        if (dLat < threshold && dLon < threshold) return true;
    }
    if (destination) {
        const dLat = Math.abs(lat - destination[0]);
        const dLon = Math.abs(lon - destination[1]);
        if (dLat < threshold && dLon < threshold) return true;
    }
    return false;
};

// Interpolate position along a polyline based on progress (0-1)
const interpolatePosition = (
    points: [number, number][],
    progress: number
): { position: [number, number]; heading: number; segmentIndex: number } => {
    if (points.length < 2) {
        return { position: points[0] || [0, 0], heading: 0, segmentIndex: 0 };
    }

    // Calculate total distance
    let totalDist = 0;
    const segmentDists: number[] = [];
    for (let i = 1; i < points.length; i++) {
        const d = Math.sqrt(
            Math.pow(points[i][0] - points[i - 1][0], 2) +
            Math.pow(points[i][1] - points[i - 1][1], 2)
        );
        segmentDists.push(d);
        totalDist += d;
    }

    const targetDist = progress * totalDist;
    let accumulated = 0;

    for (let i = 0; i < segmentDists.length; i++) {
        if (accumulated + segmentDists[i] >= targetDist) {
            const remaining = targetDist - accumulated;
            const ratio = remaining / segmentDists[i];
            const lat = points[i][0] + (points[i + 1][0] - points[i][0]) * ratio;
            const lon = points[i][1] + (points[i + 1][1] - points[i][1]) * ratio;
            const heading = calculateHeading(points[i], points[i + 1]);
            return { position: [lat, lon], heading, segmentIndex: i };
        }
        accumulated += segmentDists[i];
    }

    // Fallback to end
    const lastIdx = points.length - 1;
    return {
        position: points[lastIdx],
        heading: calculateHeading(points[lastIdx - 1], points[lastIdx]),
        segmentIndex: segmentDists.length - 1,
    };
};

// Credit Roll component - scrolling POI names
// Uses absolute positioning per item to prevent layout jumps when new items are added
const CreditRoll = ({ items, totalPOICount, mapContainer }: {
    items: CreditItem[];
    totalPOICount: number;
    mapContainer: HTMLElement | null;
}) => {
    const now = Date.now();

    // Adaptive timing: more POIs = faster scroll
    // Base: 9s visible time for few POIs, down to 3s for many (50% slower than original)
    const visibleDuration = Math.max(3000, 9000 - (totalPOICount * 75));

    // Filter to items still in view (not yet scrolled off)
    const visibleItems = items.filter(item => {
        const age = now - item.addedAt;
        return age < visibleDuration;
    });

    if (visibleItems.length === 0 || !mapContainer) return null;

    // Get map bounds for positioning
    const mapRect = mapContainer.getBoundingClientRect();
    const mapHeight = mapRect.height;

    return (
        <div style={{
            position: 'fixed',
            top: mapRect.top,
            left: mapRect.left,
            width: mapRect.width,
            height: mapRect.height,
            overflow: 'hidden', // Clip items outside map bounds
            pointerEvents: 'none',
            zIndex: 9999,
        }}>
            {visibleItems.map((item) => {
                const age = now - item.addedAt;
                const progress = age / visibleDuration;

                // Scroll from bottom to top of map
                // progress 0 = bottom of map, progress 1 = top of map
                const yPos = mapHeight * (1 - progress);

                return (
                    <div
                        key={item.id}
                        className="role-title"
                        style={{
                            position: 'absolute',
                            left: '50%',
                            top: yPos,
                            transform: 'translate(-50%, -50%)',
                            fontSize: '18px',
                            fontWeight: 500,
                            // White text with black outline
                            color: '#ffffff',
                            textShadow: `
                                -1px -1px 0 #000,
                                1px -1px 0 #000,
                                -1px 1px 0 #000,
                                1px 1px 0 #000,
                                0 0 4px rgba(0,0,0,0.8)
                            `,
                            textAlign: 'center',
                            maxWidth: '90%',
                            padding: '0 5%',
                        }}
                    >
                        {item.name}
                    </div>
                );
            })}
        </div>
    );
};

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
                        zIndex: 1000,
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
        return events.filter(e => e.lat !== 0 || e.lon !== 0);
    }, [events]);

    // Extract path as [lat, lon] tuples
    const path = useMemo((): [number, number][] => {
        return validEvents.map(e => [e.lat, e.lon] as [number, number]);
    }, [validEvents]);

    // Identify departure/destination (for bounding box)
    const { departure, destination } = useMemo(() => {
        const dep = validEvents.find(e => e.type === 'transition' && e.title?.toLowerCase().includes('take-off'));
        const dest = validEvents.slice().reverse().find(e => e.type === 'transition' && e.title?.toLowerCase().includes('landed'));

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

    // Fit map to static bounding box on mount (departure to destination)
    // No dynamic zooming during animation - keeps map stable throughout playback
    useEffect(() => {
        if (path.length < 2) return;

        const bounds = L.latLngBounds(path);
        if (departure) bounds.extend(departure);
        if (destination) bounds.extend(destination);

        map.fitBounds(bounds, { padding: [50, 50], animate: false });
    }, [map, path, departure, destination]);

    // Animation loop - continues for a cooldown period after trip ends to let POI lifecycle complete
    const LIFECYCLE_COOLDOWN_MS = 16000; // 16s = grow(4) + live(10) + shrink(2)
    const [tick, setTick] = useState(0); // Force re-renders for lifecycle animation

    // Credit roll state - track POI names as they turn green
    const [credits, setCredits] = useState<CreditItem[]>([]);
    const creditedIdsRef = useRef<Set<string>>(new Set()); // Track which POIs have already been credited

    useEffect(() => {
        if (path.length < 2 || !isPlaying) return;

        startTimeRef.current = Date.now();

        const animate = () => {
            if (!startTimeRef.current) return;
            const elapsed = Date.now() - startTimeRef.current;
            const p = Math.min(1, elapsed / durationMs);
            setProgress(p);

            // Force re-render for lifecycle animations even after progress hits 1
            setTick(t => t + 1);

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

    if (path.length < 2) {
        return null; // Not enough data to draw
    }

    const { position, heading, segmentIndex } = interpolatePosition(path, progress);

    // Path drawn so far
    const drawnPath = path.slice(0, segmentIndex + 2).map((p, i) => {
        if (i === segmentIndex + 1) return position; // Use interpolated tip
        return p;
    });

    // POIs to show (events up to current segment, plus anticipation)
    const visiblePOIs = validEvents.slice(0, segmentIndex + 2);

    // Track birth times for each POI (when it first appeared)
    const birthTimesRef = useRef<Map<string, number>>(new Map());
    const currentTime = Date.now();
    const totalPOIs = useMemo(() => validEvents.filter(e => e.type !== 'transition').length, [validEvents]);
    const shrinkTarget = useMemo(() => getShrinkTarget(totalPOIs), [totalPOIs]);

    // Compute smart POI layout using d3-force with lifecycle-aware growth/shrink/color
    const poiNodes = useMemo(() => {
        const now = currentTime;

        const poiNodeList = (visiblePOIs
            .map((poi): ReplayNode | null => {
                const globalIndex = validEvents.indexOf(poi);
                if (poi.type === 'transition' || isAirportNearTerminal(poi, departure, destination)) {
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

        // Add track points as fixed repulsion barriers (sample every few points to reduce computation)
        // We use the full path so POIs stay clear of both the past and future course
        const trackNodes: ReplayNode[] = [];
        const sampleInterval = Math.max(1, Math.floor(path.length / 50)); // ~50 sample points for better coverage of full path
        for (let i = 0; i < path.length; i += sampleInterval) {
            const projected = map.latLngToLayerPoint(path[i]);
            trackNodes.push({
                id: `track-${i}`,
                lat: path[i][0],
                lon: path[i][1],
                icon: '',
                anchorX: projected.x,
                anchorY: projected.y,
                x: projected.x,
                y: projected.y,
                r: TRACK_REPULSION_RADIUS,
                isTrackPoint: true,
                fx: projected.x,
                fy: projected.y,
            });
        }

        // Also add current plane position as repulsion point
        const planeProjected = map.latLngToLayerPoint(position);
        trackNodes.push({
            id: 'plane',
            lat: position[0],
            lon: position[1],
            icon: '',
            anchorX: planeProjected.x,
            anchorY: planeProjected.y,
            x: planeProjected.x,
            y: planeProjected.y,
            r: MARKER_RADIUS + 10,
            isTrackPoint: true,
            fx: planeProjected.x,
            fy: planeProjected.y,
        });

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
        return allNodes.filter(n => !n.isTrackPoint);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [visiblePOIs, map, drawnPath, position, tick, departure, destination, shrinkTarget]); // tick forces recalc for lifecycle animation

    // Credit roll trigger - check for POIs hitting the green phase
    useEffect(() => {
        const now = Date.now();
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
    }, [visiblePOIs, tick]); // tick ensures we check on each animation frame

    // Total POI count for adaptive credit roll speed
    const totalPOICount = validEvents.filter(e => e.type !== 'transition').length;

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

    // Get marker pane for smart placement
    const markerPane = map.getPanes().markerPane;

    return (
        <>
            {/* Route line */}
            <Polyline
                positions={drawnPath}
                pathOptions={{
                    color: 'crimson',
                    weight: 3,
                    opacity: 0.9,
                    dashArray: '10, 5',
                }}
            />

            {/* Departure marker */}
            {departure && <Marker position={departure} icon={departureIcon} interactive={false} />}

            {/* Destination marker (always visible) */}
            {destination && <Marker position={destination} icon={destinationIcon} interactive={false} />}

            {/* Smart POI markers - rendered via portal to marker pane */}
            {markerPane && createPortal(
                <div className="replay-marker-layer" style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    zIndex: 600,
                    pointerEvents: 'none',
                }}>
                    {poiNodes.map((node) => (
                        <SmartReplayMarker key={node.id} node={node} />
                    ))}
                </div>,
                markerPane
            )}


            {/* Animated plane at tip - highest z-level */}
            <Marker position={position} icon={planeIcon} interactive={false} zIndexOffset={10000} />

            {/* Credit Roll - POI names scrolling up (rendered to body for viewport positioning) */}
            {createPortal(
                <CreditRoll items={credits} totalPOICount={totalPOICount} mapContainer={map.getContainer()} />,
                document.body
            )}
        </>
    );
};
