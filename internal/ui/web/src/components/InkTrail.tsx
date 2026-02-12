import React, { useRef, useEffect, useMemo } from 'react';
import { interpolatePosition, interpolatePositionFromEvents, type TripEvent } from '../utils/replay';

interface InkTrailProps {
    pathPoints: [number, number][]; // [lat, lon][]
    validEvents?: TripEvent[];
    progress: number;
    departure: [number, number] | null;
    destination: [number, number] | null;
    project: (lnglat: [number, number]) => { x: number; y: number };
}

// Deterministic pseudo-random from seed (stable across frames)
const seededRandom = (seed: number): number => {
    const x = Math.sin(seed * 127.1 + seed * 311.7) * 43758.5453;
    return x - Math.floor(x);
};

const CRIMSON = '#e63946';
const NIB_ANGLE = Math.PI / 4;
const NIB_RATIO = 0.75;
const BASE_WIDTH = 4.5;
const DASH_LEN = 15; // 50% longer than before (was 10)
const WOBBLE_AMP = 2.5;

// Quill nib width multiplier based on stroke direction
const nibWidth = (angle: number): number => {
    const nibCross = Math.abs(Math.sin(angle - NIB_ANGLE));
    return (1 - NIB_RATIO + NIB_RATIO * nibCross) * BASE_WIDTH;
};

// A single dash: a short hand-drawn stroke segment
interface Dash {
    x0: number; y0: number;
    cx: number; cy: number;
    x1: number; y1: number;
    angle: number;
    dist: number;
}

// Draw a hand-drawn "X marks the spot" with quill nib
const drawXMark = (ctx: CanvasRenderingContext2D, x: number, y: number, size: number, color: string, alpha: number) => {
    ctx.save();
    ctx.strokeStyle = color;
    ctx.lineCap = 'round';
    ctx.globalAlpha = alpha;
    const h = size / 2;
    const seed = Math.round(x * 7 + y * 13);

    const strokes = [
        { // "\" stroke — perpendicular to nib → THICK
            x0: x - h + (seededRandom(seed + 1) - 0.5) * 3,
            y0: y - h + (seededRandom(seed + 2) - 0.5) * 3,
            cx: x + (seededRandom(seed + 3) - 0.5) * 4,
            cy: y + (seededRandom(seed + 4) - 0.5) * 4,
            x1: x + h + (seededRandom(seed + 5) - 0.3) * 3,
            y1: y + h + (seededRandom(seed + 6) - 0.3) * 3,
            angle: Math.PI / 4,
        },
        { // "/" stroke — parallel to nib → THIN
            x0: x + h + (seededRandom(seed + 7) - 0.5) * 3,
            y0: y - h + (seededRandom(seed + 8) - 0.5) * 3,
            cx: x + (seededRandom(seed + 9) - 0.5) * 4,
            cy: y + (seededRandom(seed + 10) - 0.5) * 4,
            x1: x - h + (seededRandom(seed + 11) - 0.3) * 3,
            y1: y + h + (seededRandom(seed + 12) - 0.3) * 3,
            angle: -Math.PI / 4,
        },
    ];

    for (const s of strokes) {
        ctx.lineWidth = nibWidth(s.angle) * (size / 12); // scale nib with mark size
        ctx.beginPath();
        ctx.moveTo(s.x0, s.y0);
        ctx.quadraticCurveTo(s.cx, s.cy, s.x1, s.y1);
        ctx.stroke();
    }

    ctx.restore();
};

export const InkTrail: React.FC<InkTrailProps> = ({ pathPoints, validEvents, progress, departure, destination, project }) => {
    const canvasRef = useRef<HTMLCanvasElement>(null);

    const { position, segmentIndex } = useMemo(() => {
        if (validEvents && validEvents.length >= 2) {
            return interpolatePositionFromEvents(validEvents, progress);
        }
        return interpolatePosition(pathPoints, progress);
    }, [pathPoints, validEvents, progress]);

    // Project visible points + interpolated tip to pixel space
    const projected = useMemo(() => {
        if (pathPoints.length < 2) return [];
        const pts = pathPoints.slice(0, segmentIndex + 1).map(p => project([p[1], p[0]]));
        const tip = project([position[1], position[0]]);
        pts.push(tip);
        return pts;
    }, [pathPoints, segmentIndex, position, project]);

    // Subdivide the projected polyline into small dashes with wobble
    const dashes = useMemo(() => {
        if (projected.length < 2) return [];

        const result: Dash[] = [];
        let cumDist = 0;
        let dashSeed = 0;

        let segIdx = 0;
        let segT = 0;

        const segStart = (i: number) => projected[i];
        const segEnd = (i: number) => projected[i + 1];
        const segLen = (i: number) => {
            const dx = segEnd(i).x - segStart(i).x;
            const dy = segEnd(i).y - segStart(i).y;
            return Math.sqrt(dx * dx + dy * dy);
        };

        const lerp = (i: number, t: number) => ({
            x: segStart(i).x + (segEnd(i).x - segStart(i).x) * t,
            y: segStart(i).y + (segEnd(i).y - segStart(i).y) * t,
        });

        const advance = (dist: number): { x: number; y: number } | null => {
            let remaining = dist;
            while (segIdx < projected.length - 1) {
                const sl = segLen(segIdx);
                const availableInSeg = sl * (1 - segT);
                if (remaining <= availableInSeg) {
                    segT += remaining / sl;
                    cumDist += remaining;
                    return lerp(segIdx, segT);
                }
                remaining -= availableInSeg;
                cumDist += availableInSeg;
                segIdx++;
                segT = 0;
            }
            return null;
        };

        // Skip initial gap (room for departure X mark)
        const firstGap = advance(DASH_LEN);
        if (!firstGap) return result;
        let dashStart = firstGap;

        while (segIdx < projected.length - 1) {
            const p0 = dashStart;
            // Dash length: ±15% human variation
            const dashLen = DASH_LEN * (0.85 + seededRandom(dashSeed * 19 + 7) * 0.3);
            const p1 = advance(dashLen);
            if (!p1) break;

            const angle = Math.atan2(p1.y - p0.y, p1.x - p0.x);
            const n1 = (seededRandom(dashSeed * 7 + 3) - 0.5) * 2;
            const n2 = (seededRandom(dashSeed * 13 + 17) - 0.5);
            const wobble = (n1 + n2) * WOBBLE_AMP;
            const nx = -Math.sin(angle);
            const ny = Math.cos(angle);

            result.push({
                x0: p0.x + (seededRandom(dashSeed * 3 + 1) - 0.5) * WOBBLE_AMP * 0.5,
                y0: p0.y + (seededRandom(dashSeed * 3 + 2) - 0.5) * WOBBLE_AMP * 0.5,
                cx: (p0.x + p1.x) / 2 + nx * wobble,
                cy: (p0.y + p1.y) / 2 + ny * wobble,
                x1: p1.x + (seededRandom(dashSeed * 5 + 9) - 0.5) * WOBBLE_AMP * 0.5,
                y1: p1.y + (seededRandom(dashSeed * 5 + 10) - 0.5) * WOBBLE_AMP * 0.5,
                angle,
                dist: cumDist,
            });

            dashSeed++;

            // Gap: visible through the ink bleed, with human variation
            const gapSize = 10 + seededRandom(dashSeed * 11 + 5) * 6; // 10-16px
            const gapEnd = advance(gapSize);
            if (!gapEnd) break;
            dashStart = gapEnd;
        }

        // Skip last dash (room for destination X mark) - only if not finished
        if (progress < 1 && result.length > 0) result.pop();

        return result;
    }, [projected]);

    useEffect(() => {
        const canvas = canvasRef.current;
        if (!canvas) return;

        const parent = canvas.parentElement;
        if (!parent) return;

        const dpr = window.devicePixelRatio || 1;
        const w = parent.clientWidth;
        const h = parent.clientHeight;

        canvas.width = w * dpr;
        canvas.height = h * dpr;
        canvas.style.width = `${w}px`;
        canvas.style.height = `${h}px`;

        const ctx = canvas.getContext('2d');
        if (!ctx) return;
        ctx.scale(dpr, dpr);
        ctx.clearRect(0, 0, w, h);

        // Draw airport X marks (always visible)
        if (departure) {
            const dp = project([departure[1], departure[0]]);
            drawXMark(ctx, dp.x, dp.y, 12, CRIMSON, 0.9);
        }
        if (destination) {
            const dp = project([destination[1], destination[0]]);
            drawXMark(ctx, dp.x, dp.y, 12, CRIMSON, 0.9);
        }


        if (dashes.length === 0) return;

        // All dashes are visible — projected is already clipped to current progress
        const visibleCount = dashes.length;

        const TIP_FADE_DASHES = 3;

        const drawDashes = (widthScale: number, alpha: number, blur: number) => {
            ctx.save();
            ctx.strokeStyle = CRIMSON;
            ctx.lineCap = 'round';
            ctx.lineJoin = 'round';
            if (blur > 0) ctx.filter = `blur(${blur}px)`;

            for (let i = 0; i < visibleCount; i++) {
                const d = dashes[i];

                let a = alpha;
                const fromTip = visibleCount - 1 - i;
                if (progress < 1 && fromTip < TIP_FADE_DASHES) {
                    a *= (fromTip + 1) / (TIP_FADE_DASHES + 1);
                }

                ctx.globalAlpha = Math.max(0, a);
                ctx.lineWidth = nibWidth(d.angle) * widthScale;
                ctx.beginPath();
                ctx.moveTo(d.x0, d.y0);
                ctx.quadraticCurveTo(d.cx, d.cy, d.x1, d.y1);
                ctx.stroke();
            }

            ctx.restore();
        };

        // Pass 1: strong ink bleed halo
        drawDashes(3.0, 0.25, 3);
        // Pass 2: core ink strokes
        drawDashes(1, 0.9, 0);

    }, [dashes, progress, departure, destination, project]);

    if (pathPoints.length < 2 && !departure && !destination) return null;

    return (
        <canvas
            ref={canvasRef}
            style={{
                position: 'absolute',
                left: 0,
                top: 0,
                width: '100%',
                height: '100%',
                pointerEvents: 'none',
                zIndex: 5,
            }}
        />
    );
};
