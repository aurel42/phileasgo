import type { TripEvent } from '../hooks/useTripEvents';
export type { TripEvent };

// Calculate heading from point A to point B
export const calculateHeading = (from: [number, number], to: [number, number]): number => {
    const dLon = (to[1] - from[1]) * Math.PI / 180;
    const lat1 = from[0] * Math.PI / 180;
    const lat2 = to[0] * Math.PI / 180;
    const y = Math.sin(dLon) * Math.cos(lat2);
    const x = Math.cos(lat1) * Math.sin(lat2) - Math.sin(lat1) * Math.cos(lat2) * Math.cos(dLon);
    const bearing = Math.atan2(y, x) * 180 / Math.PI;
    return (bearing + 360) % 360;
};

// Credit roll item with timestamp for animation
export interface CreditItem {
    id: string;
    name: string;
    addedAt: number; // Timestamp when added to the roll
}

// Interpolate position along a polyline based on progress (0-1)
// DEPRECATED: Use interpolatePositionFromEvents for synchronized replay
export const interpolatePosition = (
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

/**
 * Interpolates aircraft position based on event timestamps.
 * This ensures the aircraft moves at the recorded speed and stays in sync with POI discoveries.
 */
export const interpolatePositionFromEvents = (
    events: TripEvent[],
    progress: number
): { position: [number, number]; heading: number; segmentIndex: number; currentTime: number } => {
    if (events.length === 0) {
        return { position: [0, 0], heading: 0, segmentIndex: 0, currentTime: Date.now() };
    }
    if (events.length === 1) {
        return { position: [events[0].lat, events[0].lon], heading: 0, segmentIndex: 0, currentTime: new Date(events[0].timestamp).getTime() };
    }

    const firstTime = new Date(events[0].timestamp).getTime();
    const lastTime = new Date(events[events.length - 1].timestamp).getTime();
    const totalTime = lastTime - firstTime;
    const targetTime = firstTime + progress * totalTime;

    for (let i = 0; i < events.length - 1; i++) {
        const t0 = new Date(events[i].timestamp).getTime();
        const t1 = new Date(events[i + 1].timestamp).getTime();

        if (t1 >= targetTime) {
            const ratio = (t1 === t0) ? 0 : (targetTime - t0) / (t1 - t0);
            const lat = events[i].lat + (events[i + 1].lat - events[i].lat) * ratio;
            const lon = events[i].lon + (events[i + 1].lon - events[i].lon) * ratio;
            const heading = calculateHeading([events[i].lat, events[i].lon], [events[i + 1].lat, events[i + 1].lon]);
            return { position: [lat, lon], heading, segmentIndex: i, currentTime: targetTime };
        }
    }

    const lastIdx = events.length - 1;
    return {
        position: [events[lastIdx].lat, events[lastIdx].lon],
        heading: calculateHeading([events[lastIdx - 1].lat, events[lastIdx - 1].lon], [events[lastIdx].lat, events[lastIdx].lon]),
        segmentIndex: lastIdx - 1,
        currentTime: targetTime
    };
};

export const isTransitionEvent = (type: string): boolean => type === 'transition' || type === 'flight_stage';

export const isAirportNearTerminal = (poi: TripEvent, departure: [number, number] | null, destination: [number, number] | null): boolean => {
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

/**
 * Truncates trip events to the segment between take-off and landing.
 * If take-off is missing, starts at the first movement.
 */
export const getSignificantTripEvents = (events: TripEvent[]): TripEvent[] => {
    if (events.length < 2) return events;

    const takeoffIdx = events.findIndex(e => isTransitionEvent(e.type) && e.title?.toLowerCase().includes('take-off'));
    const landedIdx = events.slice().reverse().findIndex(e => isTransitionEvent(e.type) && e.title?.toLowerCase().includes('landed'));

    const startIdx = takeoffIdx !== -1 ? takeoffIdx : 0;
    const endIdx = landedIdx !== -1 ? (events.length - 1 - landedIdx) : (events.length - 1);

    return events.slice(startIdx, endIdx + 1);
};
