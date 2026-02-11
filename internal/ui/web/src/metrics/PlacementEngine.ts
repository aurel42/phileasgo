import RBush from 'rbush';
import type { BBox } from 'rbush';

export interface LabelCandidate {
    id: string;
    lat: number;
    lon: number;
    text: string;
    tier: 'city' | 'town' | 'village' | 'landmark';
    score: number;
    width: number;
    height: number;

    // New fields for sorting
    type: 'settlement' | 'poi' | 'compass';
    isHistorical: boolean;
    size?: 'S' | 'M' | 'L' | 'XL';
    icon?: string;
    visibility?: number; // 0-1 from backend

    // Output properties
    anchor?: 'center' | 'top' | 'bottom' | 'left' | 'right' | 'top-right' | 'top-left' | 'bottom-right' | 'bottom-left' | 'radial';
    finalX?: number;
    finalY?: number;
    trueX?: number; // True screen coord for tethering
    trueY?: number;
    rotation?: number; // degrees
    placedZoom?: number; // Zoom level when first placed (for map-relative scaling)
    secondaryLabel?: {
        text: string;
        width: number;
        height: number;
    };
    secondaryLabelPos?: {
        x: number;
        y: number;
        anchor: LabelCandidate['anchor'];
    };
    custom?: any;
}

interface LabelItem extends BBox {
    ownerId: string;
    type: 'label' | 'marker' | 'compass';
    custom?: any; // For debugging or extra data
}


export interface PlacementState {
    anchor: LabelCandidate['anchor'];
    radialAngle?: number;
    radialDist?: number;
    placedZoom: number;
    secondaryAnchor?: LabelCandidate['anchor'];
}

export class PlacementEngine {
    private tree: RBush<LabelItem>;
    private queue: LabelCandidate[] = [];
    private placedCache: Map<string, PlacementState> = new Map();
    private lastVisibleLabels: LabelCandidate[] = [];

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
        // Do NOT clear placedCache here. That's for resetCache().
    }

    public resetCache() {
        this.placedCache.clear();
    }

    public forget(id: string) {
        this.placedCache.delete(id);
    }

    public compute(
        projector: (lat: number, lon: number) => { x: number, y: number },
        viewportWidth: number,
        viewportHeight: number,
        zoom: number
    ): LabelCandidate[] {
        // Sort queue by Priority
        this.queue.sort((a, b) => {
            const pA = this.getPriority(a);
            const pB = this.getPriority(b);
            if (pA !== pB) return pB - pA; // Higher priority first

            // Tie-break by score
            if (a.score !== b.score) return b.score - a.score;
            return a.id.localeCompare(b.id);
        });

        const placed: LabelCandidate[] = [];
        const labelPadding = 4; // Spec: 4px buffer for text
        const iconPadding = 0;  // User: No padding for icons

        // Markers are now inserted alongside their labels according to the Sorted Intake Queue (greedy).
        // This ensures high-priority features can claim space before low-priority features block them.

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

        // SEPARATE QUEUES: Locked vs New
        const lockedCandidates: LabelCandidate[] = [];
        const newCandidates: LabelCandidate[] = [];

        for (const candidate of this.queue) {
            if (this.placedCache.has(candidate.id)) {
                lockedCandidates.push(candidate);
            } else {
                newCandidates.push(candidate);
            }
        }

        // PHASE 1: Locked Items — unconditional force-insert. These never move or get dropped.
        for (const candidate of lockedCandidates) {
            const pos = projector(candidate.lat, candidate.lon);
            const state = this.placedCache.get(candidate.id)!;
            candidate.trueX = Math.round(pos.x);
            candidate.trueY = Math.round(pos.y);

            // Scale collision box by discrete zoom ratio
            // Both current zoom and placedZoom MUST be integers to maintain the discrete parchment design.
            const zoomScale = Math.pow(2, zoom - state.placedZoom);
            const p = (candidate.type === 'poi' || candidate.type === 'compass') ? iconPadding : labelPadding;
            const halfW = ((candidate.width * zoomScale) / 2) + p;
            const halfH = ((candidate.height * zoomScale) / 2) + p;

            let cx = pos.x;
            let cy = pos.y;
            const itemType: 'marker' | 'label' | 'compass' = candidate.type === 'compass' ? 'compass' : (candidate.text ? 'label' : 'marker');

            // Apply cached anchor offset (for both icon-only POIs and text labels)
            if (state.anchor === 'center') {
                // Stays at projected position
            } else if (state.anchor === 'radial') {
                cx = pos.x + ((state.radialDist || 0) * zoomScale * Math.cos(state.radialAngle || 0));
                cy = pos.y + ((state.radialDist || 0) * zoomScale * Math.sin(state.radialAngle || 0));
            } else {
                const markerW = candidate.width * zoomScale;
                const pointRadius = (markerW / 2) + 2;
                const anchorDef = anchors.find(a => a.type === state.anchor);
                if (anchorDef) {
                    if (anchorDef.dx !== 0) cx = pos.x + (anchorDef.dx * (pointRadius + halfW));
                    if (anchorDef.dy !== 0) cy = pos.y + (anchorDef.dy * (pointRadius + halfH));
                }
            }

            // Snap to integer pixels to eliminate sub-pixel jitter
            cx = Math.round(cx);
            cy = Math.round(cy);

            const item: LabelItem = {
                minX: cx - halfW, minY: cy - halfH,
                maxX: cx + halfW, maxY: cy + halfH,
                ownerId: candidate.id, type: itemType
            };

            // COMPASS EXEMPTION: If a locked compass collides with a higher-priority settlement/landmark,
            // we drop it here so the map heartbeat can swap it to a better corner.
            if (candidate.type === 'compass') {
                const potentialCollisions = this.tree.search(item);
                // We must filter out OTHER parts of the same candidate (like its secondary label if it had one, 
                // though compass doesn't typically have one) and definitely its own previous marker box.
                if (potentialCollisions.some(other => other.ownerId !== candidate.id && !other.ownerId.startsWith(candidate.id))) {
                    continue; // Skip placement to trigger fallback
                }
            }

            // Force-insert: claim space unconditionally so new items must work around us
            this.tree.insert(item);
            candidate.anchor = state.anchor;
            candidate.finalX = cx;
            candidate.finalY = cy;
            candidate.placedZoom = state.placedZoom;
            candidate.rotation = 0;

            // RE-REGISTER SECONDARY LABEL if it was persistent
            if (candidate.secondaryLabel && state.secondaryAnchor) {
                const sHalfW = (candidate.secondaryLabel.width * zoomScale) / 2;
                const sHalfH = (candidate.secondaryLabel.height * zoomScale) / 2;
                const p = (candidate.type === 'poi' || candidate.type === 'compass') ? iconPadding : labelPadding;
                const sPointRadius = ((candidate.width * zoomScale) / 2) + p + 2;

                const sAnchor = anchors.find(a => a.type === state.secondaryAnchor);
                if (sAnchor) {
                    let scx = cx;
                    let scy = cy;
                    if (sAnchor.dx !== 0) scx = cx + (sAnchor.dx * (sPointRadius + sHalfW));
                    if (sAnchor.dy !== 0) scy = cy + (sAnchor.dy * (sPointRadius + sHalfH));

                    const sItem: LabelItem = {
                        minX: scx - sHalfW, minY: scy - sHalfH,
                        maxX: scx + sHalfW, maxY: scy + sHalfH,
                        ownerId: candidate.id + "_sec", type: 'label'
                    };
                    this.tree.insert(sItem);
                    candidate.secondaryLabelPos = {
                        x: Math.round(scx),
                        y: Math.round(scy),
                        anchor: state.secondaryAnchor
                    };
                }
            }

            placed.push(candidate);
        }

        // PHASE 2: Place New Items (Greedy Search)
        for (const candidate of newCandidates) {
            const pos = projector(candidate.lat, candidate.lon);
            candidate.trueX = Math.round(pos.x);
            candidate.trueY = Math.round(pos.y);

            const p = (candidate.type === 'poi' || candidate.type === 'compass') ? iconPadding : labelPadding;
            const halfW = (candidate.width / 2) + p;
            const halfH = (candidate.height / 2) + p;
            candidate.rotation = 0;

            // 1. For Settlements and Icon-only POI: Try true position FIRST
            // Design Correction: Settlements MUST be centered on origin (no offsets).
            if (candidate.type === 'settlement' || candidate.type === 'compass' || (candidate.type === 'poi' && !candidate.text)) {
                const item: LabelItem = {
                    minX: pos.x - halfW, minY: pos.y - halfH,
                    maxX: pos.x + halfW, maxY: pos.y + halfH,
                    ownerId: candidate.id, type: candidate.type === 'settlement' ? 'label' : (candidate.type === 'compass' ? 'compass' : 'marker')
                };

                const potentialCollisions = this.tree.search(item);
                const isBlocked = potentialCollisions.some(other => other.ownerId !== candidate.id);

                if (!isBlocked) {
                    this.tree.insert(item);
                    candidate.finalX = Math.round(pos.x);
                    candidate.finalY = Math.round(pos.y);
                    candidate.anchor = 'center';
                    candidate.placedZoom = Math.floor(zoom);

                    // Cache MUST be set before tryPlaceSecondary (it updates secondaryAnchor on this entry)
                    this.placedCache.set(candidate.id, { anchor: 'center', placedZoom: Math.floor(zoom) });

                    if (candidate.secondaryLabel) {
                        this.tryPlaceSecondary(candidate, pos.x, pos.y, viewportWidth, viewportHeight);
                    }

                    placed.push(candidate);
                    continue;
                }

                // If a settlement or compass is blocked at its origin, it's dropped (no legacy anchors)
                if (candidate.type === 'settlement' || candidate.type === 'compass') continue;
                // If blocked POI, fall through to anchor/radial search
            }

            // 3. Search for available space (Labels OR Blocked POIs)
            let isPlaced = false;

            // STAGE 1: 8-Point Anchor Search
            for (const textAnchor of anchors) {
                if (this.tryPlace(candidate, pos.x, pos.y, textAnchor.dx, textAnchor.dy, p, textAnchor.type, viewportWidth, viewportHeight)) {
                    isPlaced = true;
                    candidate.placedZoom = Math.floor(zoom);

                    // Cache MUST be set before tryPlaceSecondary (it updates secondaryAnchor on this entry)
                    this.placedCache.set(candidate.id, { anchor: textAnchor.type, placedZoom: Math.floor(zoom) });

                    if (candidate.secondaryLabel) {
                        this.tryPlaceSecondary(candidate, candidate.finalX!, candidate.finalY!, viewportWidth, viewportHeight);
                    }

                    placed.push(candidate);
                    break;
                }
            }

            // STAGE 2: Radial Search Fallback
            if (!isPlaced) {
                const maxRadius = 80;
                const radiusStep = 5;
                const angleStep = 15 * (Math.PI / 180);

                for (let r = radiusStep; r <= maxRadius; r += radiusStep) {
                    for (let theta = 0; theta < 2 * Math.PI; theta += angleStep) {
                        const dx = Math.cos(theta);
                        const dy = Math.sin(theta);
                        const cx = pos.x + (r * dx);
                        const cy = pos.y + (r * dy);

                        if (this.isOutsideViewport(cx, cy, halfW, halfH, viewportWidth, viewportHeight)) continue;

                        if (this.checkCollisionAndInsert(candidate, cx, cy, p)) {
                            candidate.anchor = 'radial';
                            candidate.finalX = Math.round(cx);
                            candidate.finalY = Math.round(cy);
                            candidate.placedZoom = Math.floor(zoom);

                            this.placedCache.set(candidate.id, {
                                anchor: 'radial',
                                radialAngle: theta,
                                radialDist: r,
                                placedZoom: Math.floor(zoom)
                            });

                            // No secondary label for radial placements — marker is too far from true position
                            placed.push(candidate);
                            isPlaced = true;
                            break;
                        }
                    }
                    if (isPlaced) break;
                }
            }
        }

        this.lastVisibleLabels = placed;
        return placed;
    }

    public getVisibleLabels(): LabelCandidate[] {
        return this.lastVisibleLabels;
    }

    private isOutsideViewport(cx: number, cy: number, hw: number, hh: number, vw: number, vh: number): boolean {
        return (cx - hw < 0 || cx + hw > vw || cy - hh < 0 || cy + hh > vh);
    }

    private getPriority(c: LabelCandidate): number {
        // High number = High Priority

        // 0. Landmarks (Fixed Viewport Elements)
        if (c.tier === 'landmark') return 300;

        // 1. Compass Rose (200)
        if (c.type === 'compass') return 200;

        // 1. Highest-Tier Settlements (100)
        if (c.type === 'settlement') {
            if (c.tier === 'city') return 100;
            if (c.tier === 'town') return 95;
            return 90; // village
        }

        // 2. POIs Interleaved by size and freshness (As per Design Doc 3.2)
        // Order: Active S > Historical S > Active M > Historical M > ...
        const sizeWeight = { 'S': 80, 'M': 70, 'L': 60, 'XL': 50 };
        const base = sizeWeight[c.size || 'S'];
        const freshnessBonus = c.isHistorical ? 0 : 5;

        // Normalized score (0..2) as tie-break within buckets (scores are 1-50)
        const normalizedScore = Math.min(2, ((c.score || 1) - 1) / 49 * 2);
        return base + freshnessBonus + normalizedScore;
    }

    private tryPlace(
        candidate: LabelCandidate,
        baseX: number,
        baseY: number,
        dx: number,
        dy: number,
        padding: number,
        anchorType: LabelCandidate['anchor'],
        vw: number,
        vh: number
    ): boolean {
        const halfW = (candidate.width / 2) + padding;
        const halfH = (candidate.height / 2) + padding;
        const pointRadius = (candidate.width / 2) + 2;

        let cx = baseX;
        let cy = baseY;

        // Shift center away from point
        if (dx !== 0) cx = baseX + (dx * (pointRadius + halfW));
        if (dy !== 0) cy = baseY + (dy * (pointRadius + halfH));

        // Viewport Clipping (Design 6.0)
        if (this.isOutsideViewport(cx, cy, halfW, halfH, vw, vh)) {
            return false;
        }

        if (this.checkCollisionAndInsert(candidate, cx, cy, padding)) {
            candidate.anchor = anchorType;
            candidate.finalX = Math.round(cx);
            candidate.finalY = Math.round(cy);
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
            // A secondary label (ownerId: X_sec) SHOULD collide with its own primary (ownerId: X)
            // to ensure it clears the icon properly.
            if (other.ownerId === item.ownerId) continue;
            return false; // Collision
        }

        this.tree.insert(item);
        return true;
    }

    private tryPlaceSecondary(
        candidate: LabelCandidate,
        baseX: number,
        baseY: number,
        vw: number,
        vh: number
    ): boolean {
        if (!candidate.secondaryLabel) return false;

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

        const halfW = candidate.secondaryLabel.width / 2;
        const halfH = candidate.secondaryLabel.height / 2;
        // Padding removal: POI labels (secondary) now have 0 padding as per user request.
        // Settlement labels (primary) keep 4px.
        const sPadding = (candidate.type === 'poi' || candidate.type === 'compass') ? 0 : 4;
        const pointRadius = (candidate.width / 2) + sPadding + 2;

        for (const anchor of anchors) {
            let cx = baseX;
            let cy = baseY;

            if (anchor.dx !== 0) cx = baseX + (anchor.dx * (pointRadius + halfW));
            if (anchor.dy !== 0) cy = baseY + (anchor.dy * (pointRadius + halfH));

            if (this.isOutsideViewport(cx, cy, halfW, halfH, vw, vh)) continue;

            const item: LabelItem = {
                minX: cx - halfW, minY: cy - halfH,
                maxX: cx + halfW, maxY: cy + halfH,
                ownerId: candidate.id + "_sec",
                type: 'label'
            };

            const collisions = this.tree.search(item);
            // Secondary labels MUST NOT overlap anything else, INCLUDING their parent icon.
            // (Wait, actually they MUST NOT overlap anything EXCEPT themselves)
            const isBlocked = collisions.some(other => other.ownerId !== item.ownerId);

            if (!isBlocked) {
                this.tree.insert(item);
                candidate.secondaryLabelPos = {
                    x: Math.round(cx),
                    y: Math.round(cy),
                    anchor: anchor.type!
                };

                // Update the state in the cache to include the secondary anchor
                const state = this.placedCache.get(candidate.id);
                if (state) {
                    state.secondaryAnchor = anchor.type;
                }

                return true;
            }
        }
        return false;
    }
}
