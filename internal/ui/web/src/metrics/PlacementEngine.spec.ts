import { PlacementEngine, type LabelCandidate } from './PlacementEngine';

describe('PlacementEngine', () => {
    let engine: PlacementEngine;
    const projector = (lat: number, lon: number) => ({ x: lat, y: lon }); // Simple 1:1 projector for testing
    const viewport = { w: 1000, h: 1000 };

    beforeEach(() => {
        engine = new PlacementEngine();
    });

    describe('Priority & Sorting', () => {
        it('should rank Cities higher than Towns and Villages', () => {
            const city: LabelCandidate = {
                id: '1', lat: 100, lon: 100, text: 'Big City', tier: 'city', type: 'settlement',
                score: 100, width: 50, height: 20, isHistorical: false
            };
            const town: LabelCandidate = {
                id: '2', lat: 200, lon: 200, text: 'Small Town', tier: 'town', type: 'settlement',
                score: 100, width: 50, height: 20, isHistorical: false
            };
            const village: LabelCandidate = {
                id: '3', lat: 300, lon: 300, text: 'Tiny Village', tier: 'village', type: 'settlement',
                score: 100, width: 50, height: 20, isHistorical: false
            };

            engine.register(village);
            engine.register(city);
            engine.register(town);

            const result = engine.compute(projector, viewport.w, viewport.h, 10);

            expect(result[0].id).toBe('1'); // City
            expect(result[1].id).toBe('2'); // Town
            expect(result[2].id).toBe('3'); // Village
        });

        it('should rank Active POIs higher than Historical POIs of the same size', () => {
            const histPOI: LabelCandidate = {
                id: 'h', lat: 100, lon: 100, text: 'Old Spot', tier: 'landmark', type: 'poi',
                score: 10, width: 20, height: 20, isHistorical: true, size: 'M'
            };
            const activePOI: LabelCandidate = {
                id: 'a', lat: 200, lon: 200, text: 'New Spot', tier: 'landmark', type: 'poi',
                score: 10, width: 20, height: 20, isHistorical: false, size: 'M'
            };

            engine.register(histPOI);
            engine.register(activePOI);

            const result = engine.compute(projector, viewport.w, viewport.h, 10);
            expect(result[0].id).toBe('a');
            expect(result[1].id).toBe('h');
        });
    });

    describe('Collision Detection', () => {
        it('should drop a lower priority label if it collides with a higher priority one', () => {
            const high: LabelCandidate = {
                id: 'high', lat: 500, lon: 500, text: 'CAPITAL', tier: 'city', type: 'settlement',
                score: 100, width: 100, height: 50, isHistorical: false
            };
            const low: LabelCandidate = {
                id: 'low', lat: 510, lon: 510, text: 'Village', tier: 'village', type: 'settlement',
                score: 10, width: 50, height: 20, isHistorical: false
            };

            engine.register(low);
            engine.register(high);

            const result = engine.compute(projector, viewport.w, viewport.h, 10);

            expect(result).toHaveLength(1);
            expect(result[0].id).toBe('high');
        });

        it('should allow non-overlapping labels', () => {
            const a: LabelCandidate = {
                id: 'a', lat: 100, lon: 100, text: 'A', tier: 'city', type: 'settlement',
                score: 100, width: 50, height: 20, isHistorical: false
            };
            const b: LabelCandidate = {
                id: 'b', lat: 200, lon: 200, text: 'B', tier: 'city', type: 'settlement',
                score: 100, width: 50, height: 20, isHistorical: false
            };

            engine.register(a);
            engine.register(b);

            const result = engine.compute(projector, viewport.w, viewport.h, 10);
            expect(result).toHaveLength(2);
        });
    });

    describe('Locked Cache (Stability)', () => {
        it('should preserve coordinates from placedCache even if they would otherwise collide', () => {
            // We force a "locked" state where a label is already placed at a specific spot
            // via a mock or by running compute once.
            const item: LabelCandidate = {
                id: 'locked', lat: 500, lon: 500, text: 'STAY', tier: 'city', type: 'settlement',
                score: 100, width: 50, height: 20, isHistorical: false
            };

            engine.register(item);
            engine.compute(projector, viewport.w, viewport.h, 10); // First placement at zoom 10

            // Now "re-center" or change conditions, but keep the ID in queue
            // The engine should use the cached state.
            const result = engine.compute(projector, viewport.w, viewport.h, 11);

            expect(result[0].placedZoom).toBe(10); // Preserved from cache
            expect(result[0].anchor).toBe('center');
        });
    });

    describe('Radial Search', () => {
        it('should move a POI to a radial ring if primary anchors are blocked', () => {
            // Block the center with a high-priority landmark
            const blocker: LabelCandidate = {
                id: 'blocker', lat: 500, lon: 500, text: 'BLOCKER', tier: 'landmark', type: 'settlement',
                score: 1000, width: 20, height: 20, isHistorical: false
            };
            const poi: LabelCandidate = {
                id: 'poi', lat: 500, lon: 500, text: 'FIND SPACE', tier: 'landmark', type: 'poi',
                score: 10, width: 10, height: 10, isHistorical: false
            };

            engine.register(blocker);
            engine.register(poi);

            const result = engine.compute(projector, viewport.w, viewport.h, 10);

            const placedPoi = result.find(r => r.id === 'poi');
            expect(placedPoi).toBeDefined();
            expect(placedPoi?.anchor).toBe('radial');
            // Should be pushed away from (500, 500)
            expect(Math.abs(placedPoi!.finalX! - 500)).toBeGreaterThanOrEqual(20);
        });

        it('should never drop a symbol even if it must be displaced far from origin', () => {
            // Create a giant blocker that covers more than 80px radius
            const blocker: LabelCandidate = {
                id: 'blocker', lat: 500, lon: 500, text: 'GIANT BLOCKER', tier: 'landmark', type: 'settlement',
                score: 1000, width: 200, height: 200, isHistorical: false
            };
            const poi: LabelCandidate = {
                id: 'poi', lat: 500, lon: 500, text: '', tier: 'landmark', type: 'poi',
                score: 10, width: 26, height: 26, isHistorical: false
            };

            engine.register(blocker);
            engine.register(poi);

            const result = engine.compute(projector, viewport.w, viewport.h, 10);

            const placedPoi = result.find(r => r.id === 'poi');
            expect(placedPoi).toBeDefined();
            expect(placedPoi?.anchor).toBe('radial');
            // Displacement should be > 100px (since blocker is 200x200 centered at 500,500)
            const dx = placedPoi!.finalX! - 500;
            const dy = placedPoi!.finalY! - 500;
            const dist = Math.sqrt(dx * dx + dy * dy);
            expect(dist).toBeGreaterThan(100);
        });
    });

    describe('Viewport Clipping', () => {
        it('should place labels that are partially inside the viewport', () => {
            const edgeItem: LabelCandidate = {
                id: 'edge', lat: 5, lon: 5, text: 'EDGE', tier: 'city', type: 'settlement',
                score: 100, width: 50, height: 20, isHistorical: false
            };

            engine.register(edgeItem);
            const result = engine.compute(projector, viewport.w, viewport.h, 10);

            // Should be placed even if cut-off (Printed Map Design)
            expect(result).toHaveLength(1);
        });
    });

    describe('POI Marker Labels', () => {
        it('should place a marker label if space is available', () => {
            const poi: LabelCandidate = {
                id: 'poi', lat: 500, lon: 500, tier: 'landmark', type: 'poi',
                score: 100, width: 20, height: 20, isHistorical: false,
                text: '', // icon only primary
                markerLabel: { text: 'Label', width: 40, height: 15 }
            };

            engine.register(poi);
            const result = engine.compute(projector, viewport.w, viewport.h, 10);

            expect(result[0].markerLabelPos).toBeDefined();
            expect(result[0].markerLabelPos?.anchor).toBe('top-right');
        });

        it('should clear the marker label when the cache is reset', () => {
            const poi: LabelCandidate = {
                id: 'poi', lat: 500, lon: 500, tier: 'landmark', type: 'poi',
                score: 100, width: 20, height: 20, isHistorical: false,
                text: '',
                markerLabel: { text: 'Label', width: 40, height: 15 }
            };

            engine.register(poi);
            engine.compute(projector, viewport.w, viewport.h, 10);

            // Reset cache, re-register, re-compute — should get fresh placement
            engine.resetCache();
            engine.clear();
            engine.register(poi);
            const result = engine.compute(projector, viewport.w, viewport.h, 12);

            expect(result[0].placedZoom).toBe(12); // Fresh placement at new zoom
            expect(result[0].markerLabelPos).toBeDefined();
        });
    });

    describe('Lifecycle', () => {
        it('should deduplicate registrations by ID', () => {
            const item: LabelCandidate = {
                id: 'dup', lat: 500, lon: 500, text: 'DUP', tier: 'city', type: 'settlement',
                score: 100, width: 50, height: 20, isHistorical: false
            };

            engine.register(item);
            engine.register(item); // duplicate

            const result = engine.compute(projector, viewport.w, viewport.h, 10);
            expect(result).toHaveLength(1);
        });

        it('should forget a specific cached item', () => {
            const item: LabelCandidate = {
                id: 'forgettable', lat: 500, lon: 500, text: 'FORGET', tier: 'city', type: 'settlement',
                score: 100, width: 50, height: 20, isHistorical: false
            };

            engine.register(item);
            engine.compute(projector, viewport.w, viewport.h, 10);

            engine.forget('forgettable');
            engine.clear();
            engine.register(item);
            const result = engine.compute(projector, viewport.w, viewport.h, 12);

            // Should be re-placed at new zoom since cache was forgotten
            expect(result[0].placedZoom).toBe(12);
        });

        it('should return visible labels via getVisibleLabels', () => {
            const item: LabelCandidate = {
                id: 'vis', lat: 500, lon: 500, text: 'VISIBLE', tier: 'city', type: 'settlement',
                score: 100, width: 50, height: 20, isHistorical: false
            };

            engine.register(item);
            engine.compute(projector, viewport.w, viewport.h, 10);

            const visible = engine.getVisibleLabels();
            expect(visible).toHaveLength(1);
            expect(visible[0].id).toBe('vis');
        });
    });
});
