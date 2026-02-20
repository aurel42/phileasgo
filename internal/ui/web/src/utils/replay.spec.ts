
import {
    calculateHeading,
    interpolatePosition,
    interpolatePositionFromEvents,
    getSignificantTripEvents,
    isTransitionEvent,
    isAirportNearTerminal
} from './replay';
import type { TripEvent } from '../hooks/useTripEvents';

describe('replay utilities', () => {
    describe('calculateHeading', () => {
        it('calculates cardinal directions correctly', () => {
            // North: 0 deg
            expect(calculateHeading([0, 0], [1, 0])).toBeCloseTo(0, 0);
            // East: 90 deg
            expect(calculateHeading([0, 0], [0, 1])).toBeCloseTo(90, 0);
            // South: 180 deg
            expect(calculateHeading([1, 0], [0, 0])).toBeCloseTo(180, 0);
            // West: 270 deg
            expect(calculateHeading([0, 1], [0, 0])).toBeCloseTo(270, 0);
        });

        it('handles 360/0 degree wrapping', () => {
            expect(calculateHeading([0, 0], [1, -0.001])).toBeGreaterThan(359);
        });
    });

    describe('interpolatePosition', () => {
        const points: [number, number][] = [[0, 0], [1, 1], [2, 0]];

        it('returns start point at progress 0', () => {
            const { position, segmentIndex } = interpolatePosition(points, 0);
            expect(position).toEqual([0, 0]);
            expect(segmentIndex).toBe(0);
        });

        it('returns end point at progress 1', () => {
            const { position, segmentIndex } = interpolatePosition(points, 1);
            expect(position).toEqual([2, 0]);
            expect(segmentIndex).toBe(1);
        });

        it('interpolates middle point correctly', () => {
            const { position, segmentIndex } = interpolatePosition(points, 0.5);
            expect(position[0]).toBeCloseTo(1, 1);
            expect(position[1]).toBeCloseTo(1, 1);
            expect(segmentIndex).toBe(0);
        });

        it('handles single point gracefully', () => {
            const result = interpolatePosition([[10, 20]], 0.5);
            expect(result.position).toEqual([10, 20]);
            expect(result.heading).toBe(0);
        });
    });

    describe('interpolatePositionFromEvents', () => {
        const now = Date.now();
        const events: TripEvent[] = [
            { type: 'activity', timestamp: new Date(now).toISOString(), lat: 0, lon: 0 },
            { type: 'activity', timestamp: new Date(now + 1000).toISOString(), lat: 1, lon: 1 },
            { type: 'activity', timestamp: new Date(now + 2000).toISOString(), lat: 2, lon: 0 },
        ];

        it('returns start point at progress 0', () => {
            const { position, currentTime } = interpolatePositionFromEvents(events, 0);
            expect(position).toEqual([0, 0]);
            expect(currentTime).toBe(now);
        });

        it('returns end point at progress 1', () => {
            const { position, currentTime } = interpolatePositionFromEvents(events, 1);
            expect(position).toEqual([2, 0]);
            expect(currentTime).toBe(now + 2000);
        });

        it('interpolates middle point correctly', () => {
            const { position, currentTime } = interpolatePositionFromEvents(events, 0.5);
            expect(position[0]).toBeCloseTo(1, 1);
            expect(position[1]).toBeCloseTo(1, 1);
            expect(currentTime).toBe(now + 1000);
        });

        it('handles single event gracefully', () => {
            const result = interpolatePositionFromEvents([events[0]], 0.5);
            expect(result.position).toEqual([0, 0]);
        });

        it('handles zero events gracefully', () => {
            const result = interpolatePositionFromEvents([], 0.5);
            expect(result.position).toEqual([0, 0]);
        });
    });

    describe('isTransitionEvent', () => {
        it('identifies transition and flight_stage types', () => {
            expect(isTransitionEvent('transition')).toBe(true);
            expect(isTransitionEvent('flight_stage')).toBe(true);
            expect(isTransitionEvent('poi')).toBe(false);
            expect(isTransitionEvent('')).toBe(false);
        });
    });

    describe('isAirportNearTerminal', () => {
        const mockAirport: TripEvent = {
            type: 'poi',
            timestamp: new Date().toISOString(),
            lat: 10,
            lon: 20,
            metadata: {
                icon: 'airfield',
                poi_category: 'aerodrome'
            }
        };

        it('returns true if airport is near departure', () => {
            expect(isAirportNearTerminal(mockAirport, [10.01, 20.01], null)).toBe(true);
        });

        it('returns true if airport is near destination', () => {
            expect(isAirportNearTerminal(mockAirport, null, [9.99, 19.99])).toBe(true);
        });

        it('returns false if airport is far from both', () => {
            expect(isAirportNearTerminal(mockAirport, [11, 21], [9, 19])).toBe(false);
        });

        it('returns false if POI is not an airport', () => {
            const notAirport = { ...mockAirport, metadata: { icon: 'castle' } };
            expect(isAirportNearTerminal(notAirport, [10.01, 20.01], null)).toBe(false);
        });
    });

    describe('getSignificantTripEvents', () => {
        const events: TripEvent[] = [
            { type: 'poi', timestamp: '2024-01-01T12:00:00Z', lat: 0, lon: 0, title: 'P1' },
            { type: 'transition', timestamp: '2024-01-01T12:05:00Z', lat: 0, lon: 0, title: 'Take-off' },
            { type: 'poi', timestamp: '2024-01-01T12:10:00Z', lat: 1, lon: 1, title: 'P2' },
            { type: 'transition', timestamp: '2024-01-01T12:15:00Z', lat: 1, lon: 1, title: 'Landed' },
            { type: 'poi', timestamp: '2024-01-01T12:20:00Z', lat: 1, lon: 1, title: 'P3' },
        ];

        it('truncates to take-off and landing', () => {
            const result = getSignificantTripEvents(events);
            expect(result).toHaveLength(3);
            expect(result[0].title).toBe('Take-off');
            expect(result[result.length - 1].title).toBe('Landed');
        });

        it('starts from beginning if take-off missing', () => {
            const result = getSignificantTripEvents(events.slice(2));
            expect(result[0].title).toBe('P2');
        });

        it('ends at the last event if landed missing', () => {
            const result = getSignificantTripEvents(events.slice(0, 3));
            expect(result[result.length - 1].title).toBe('P2');
        });
    });
});
