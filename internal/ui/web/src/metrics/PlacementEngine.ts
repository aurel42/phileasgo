import RBush from 'rbush';
import type { BBox } from 'rbush';

export interface LabelCandidate {
    id: string;
    lat: number;
    lon: number;
    text: string;
    tier: 'city' | 'town' | 'village';
    score: number;
    width: number;
    height: number;

    // New fields for sorting
    type: 'settlement' | 'poi';
    isHistorical: boolean;
    size?: 'S' | 'M' | 'L' | 'XL';

    // Output properties
    anchor?: 'center' | 'top' | 'bottom' | 'left' | 'right' | 'top-right' | 'top-left' | 'bottom-right' | 'bottom-left' | 'radial';
    finalX?: number;
    finalY?: number;
    rotation?: number; // degrees
}

interface LabelItem extends BBox {
    ownerId: string;
    type: 'label' | 'marker';
    custom?: any; // For debugging or extra data
}

export class PlacementEngine {
    private tree: RBush<LabelItem>;
    private queue: LabelCandidate[] = [];

    constructor() {
        this.tree = new RBush<LabelItem>();
    }

    public register(candidate: LabelCandidate) {
        // Deduplicate by ID
        if (this.queue.some(c => c.id === candidate.id)) {
            return;
        }
        this.queue.push(candidate);
    }

    public clear() {
        this.tree.clear();
        this.queue = [];
    }

    public compute(projector: (lat: number, lon: number) => { x: number, y: number }): LabelCandidate[] {
        // Sort queue by Priority
        // 1. Settlements (City > Town > Village)
        // 2. Active POIs (Score DESC)
        // 3. Historical POIs (Score DESC)
        this.queue.sort((a, b) => {
            const pA = this.getPriority(a);
            const pB = this.getPriority(b);
            if (pA !== pB) return pB - pA; // Higher priority first

            // Tie-break by score
            if (a.score !== b.score) return b.score - a.score;
            return a.id.localeCompare(b.id);
        });

        const placed: LabelCandidate[] = [];
        const padding = 4; // Spec: 4px buffer
        const markerSize = 24; // Fixed marker size (w/h)
        const markerHalf = markerSize / 2;

        // 1. Insert ALL Markers as obstacles first
        for (const candidate of this.queue) {
            const pos = projector(candidate.lat, candidate.lon);
            const mItem: LabelItem = {
                minX: pos.x - markerHalf,
                minY: pos.y - markerHalf,
                maxX: pos.x + markerHalf,
                maxY: pos.y + markerHalf,
                ownerId: candidate.id,
                type: 'marker'
            };
            this.tree.insert(mItem);
        }

        // Define anchor offsets (dx, dy multipliers)
        // order: Top-Right (Preferred), Top, Right, Bottom, Left...
        const anchors: { type: LabelCandidate['anchor'], dx: number, dy: number }[] = [
            { type: 'top-right', dx: 1, dy: -1 },
            { type: 'top', dx: 0, dy: -1 },
            { type: 'right', dx: 1, dy: 0 },
            { type: 'bottom', dx: 0, dy: 1 },
            { type: 'left', dx: -1, dy: 0 },
            { type: 'top-left', dx: -1, dy: -1 },
            { type: 'bottom-right', dx: 1, dy: 1 },
            { type: 'bottom-left', dx: -1, dy: 1 },
        ];

        for (const candidate of this.queue) {
            const pos = projector(candidate.lat, candidate.lon);
            let isPlaced = false;

            // Apply rotation for settlements
            if (candidate.type === 'settlement') {
                candidate.rotation = -20;
            } else {
                candidate.rotation = 0;
            }

            // STAGE 1: 8-Point Anchor Search
            for (const textAnchor of anchors) {
                if (this.tryPlace(candidate, pos.x, pos.y, textAnchor.dx, textAnchor.dy, padding, textAnchor.type)) {
                    isPlaced = true;
                    placed.push(candidate);
                    break;
                }
            }

            // STAGE 2: Radial Search Fallback
            if (!isPlaced) {
                // Spiral out: max 50px radius, 5px steps, 15 degree angle steps
                const maxRadius = 50;
                const radiusStep = 5;
                const angleStep = 15 * (Math.PI / 180);

                for (let r = radiusStep; r <= maxRadius; r += radiusStep) {
                    for (let theta = 0; theta < 2 * Math.PI; theta += angleStep) {
                        const dx = Math.cos(theta);
                        const dy = Math.sin(theta); // Screen Y is down, but radial math is agnostic

                        // We treat radial search as "custom offset"
                        // Effectively similar to an anchor but with continuous position
                        // We use dx/dy as unit vectors for the offset direction
                        // But wait, tryPlace expects -1, 0, 1 grid logic? 
                        // No, let's just calculate raw CX/CY and check collision manually to be cleaner.

                        // For radial search, we just want to push the label away from the center.
                        // Let's assume the label is centered on the radial point?
                        // Or "docks" to the point?
                        // Let's assume we place the center of the label at (pos.x + r*dx, pos.y + r*dy)

                        const cx = pos.x + (r * dx);
                        const cy = pos.y + (r * dy);

                        if (this.checkCollisionAndInsert(candidate, cx, cy, padding)) {
                            candidate.anchor = 'radial';
                            candidate.finalX = cx;
                            candidate.finalY = cy;
                            placed.push(candidate);
                            isPlaced = true;
                            break;
                        }
                    }
                    if (isPlaced) break;
                }
            }
        }

        return placed;
    }

    private getPriority(c: LabelCandidate): number {
        // High number = High Priority
        if (c.type === 'settlement') {
            if (c.tier === 'city') return 100;
            if (c.tier === 'town') return 90;
            return 80; // village
        }

        // POIs
        // Active > Historical
        // Bucketed by score? Or just use raw score?
        // We'll give broad buckets to mix sizes if needed, but per spec:
        // "Active POIs" > "Historical POIs"
        // Let's just create a base offset
        const base = c.isHistorical ? 0 : 40; // Active = 40+, Historical = 0+

        // Add score (0..1) * 10 to give intra-group sorting
        return base + (c.score * 10);
    }

    private tryPlace(
        candidate: LabelCandidate,
        baseX: number,
        baseY: number,
        dx: number,
        dy: number,
        padding: number,
        anchorType: LabelCandidate['anchor']
    ): boolean {
        const halfW = (candidate.width / 2) + padding;
        const halfH = (candidate.height / 2) + padding;
        const pointRadius = 6;

        let cx = baseX;
        let cy = baseY;

        // Shift center away from point
        if (dx !== 0) cx = baseX + (dx * (pointRadius + halfW));
        if (dy !== 0) cy = baseY + (dy * (pointRadius + halfH));

        if (this.checkCollisionAndInsert(candidate, cx, cy, padding)) {
            candidate.anchor = anchorType;
            candidate.finalX = cx;
            candidate.finalY = cy;
            return true;
        }
        return false;
    }

    private checkCollisionAndInsert(candidate: LabelCandidate, cx: number, cy: number, padding: number): boolean {
        const halfW = (candidate.width / 2) + padding;
        const halfH = (candidate.height / 2) + padding;

        const minX = cx - halfW;
        const minY = cy - halfH;
        const maxX = cx + halfW;
        const maxY = cy + halfH;

        const item: LabelItem = {
            minX, minY, maxX, maxY,
            ownerId: candidate.id,
            type: 'label'
        };

        const potentialCollisions = this.tree.search(item);
        for (const other of potentialCollisions) {
            if (other.ownerId === candidate.id) continue;
            return false; // Collision
        }

        this.tree.insert(item);
        return true;
    }
}
