import React, { useMemo } from 'react';

// "Clean" distance steps for rounding (in the unit's base: km or NM)
const CLEAN_STEPS = [0.1, 0.2, 0.5, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000];

// Faded blue ink palette (antique cartographic blue)
const BLUE_INK = 'rgba(45, 65, 105, 0.85)';

interface ScaleBarProps {
    zoom: number;
    latitude: number;
}

/** Find the nearest "clean" value ≤ rawValue from the step table */
function snapToClean(rawValue: number): number {
    for (let i = CLEAN_STEPS.length - 1; i >= 0; i--) {
        if (CLEAN_STEPS[i] <= rawValue) return CLEAN_STEPS[i];
    }
    return CLEAN_STEPS[0];
}

/**
 * Compute the scale axis for a given unit.
 * Returns { cleanValue, barWidthPx, segmentWidthPx }
 */
function computeAxis(metersPerPixel: number, targetPx: number, unitMeters: number) {
    const rawDistance = (metersPerPixel * targetPx) / unitMeters;
    const cleanValue = snapToClean(rawDistance);
    const barWidthPx = (cleanValue * unitMeters) / metersPerPixel;

    // Heuristic: Prefer 4 splits, but switch to 5 if 4 results in decimals and 5 results in integers
    const isInt = (n: number) => n % 1 === 0;
    let segmentCount = 4;
    // Check if 4-split is messy (decimals) but 5-split is clean (integers)
    // Only applies for values >= 1 to avoid over-complicating sub-unit scales
    if (cleanValue >= 1 && !isInt(cleanValue / 4) && isInt(cleanValue / 5)) {
        segmentCount = 5;
    }

    const segmentWidthPx = barWidthPx / segmentCount;
    return { cleanValue, barWidthPx, segmentWidthPx, segmentCount };
}

/**
 * Navigator's Dual-Scale Bar
 *
 * Renders two parallel checkered bars:
 *   Top = Kilometers, Bottom = Nautical Miles
 *
 * Positioned bottom-left, ON TOP of the paper overlay (zIndex 15).
 * Uses faded blue ink for an antique nautical chart look.
 * No collision detection — purely decorative.
 */
export const ScaleBar: React.FC<ScaleBarProps> = ({ zoom, latitude }) => {
    const scaleData = useMemo(() => {
        // Mercator-corrected meters per pixel at this latitude and zoom
        const metersPerPixel = (156543.03 * Math.cos((latitude * Math.PI) / 180)) / Math.pow(2, zoom);

        // Target bar width: ~28% of a typical viewport width (use 400px as a sensible default)
        // The component itself will use the computed pixel width, so this is approximate
        const targetPx = 300;

        const km = computeAxis(metersPerPixel, targetPx, 1000);
        const nm = computeAxis(metersPerPixel, targetPx, 1852);

        return { km, nm };
    }, [zoom, latitude]);

    const barHeight = 7;
    const gap = 9; // vertical gap between the two bars
    const tickHeight = 5;

    // Format number: drop decimals for >= 1, keep one decimal for < 1
    const fmt = (v: number) => (v >= 1 ? String(v) : v.toFixed(1));

    const renderAxis = (
        axis: { cleanValue: number; barWidthPx: number; segmentWidthPx: number; segmentCount: number },
        unit: string,
        yOffset: number,
        labelAbove: boolean
    ) => {
        const segments = [];
        const ticks = [];
        const labels = [];

        for (let i = 0; i < axis.segmentCount; i++) {
            const x = i * axis.segmentWidthPx;
            const fill = i % 2 === 0 ? 'url(#scale-hatch)' : 'none';
            segments.push(
                <rect
                    key={`seg-${i}`}
                    x={x}
                    y={yOffset}
                    width={axis.segmentWidthPx}
                    height={barHeight}
                    fill={fill}
                    stroke={BLUE_INK}
                    strokeWidth={0.7}
                />
            );
        }

        // Tick marks and number labels at each division
        for (let i = 0; i <= axis.segmentCount; i++) {
            const x = i * axis.segmentWidthPx;
            const ty = labelAbove ? yOffset - tickHeight : yOffset + barHeight;

            ticks.push(
                <line
                    key={`tick-${i}`}
                    x1={x}
                    y1={ty}
                    x2={x}
                    y2={ty + tickHeight}
                    stroke={BLUE_INK}
                    strokeWidth={1.0}
                />
            );

            // Number label at each tick (skip 0 to avoid clutter)
            if (i > 0) {
                const value = (axis.cleanValue / axis.segmentCount) * i;
                const ly = labelAbove ? yOffset - tickHeight - 3 : yOffset + barHeight + tickHeight + 10;

                labels.push(
                    <text
                        key={`label-${i}`}
                        x={x}
                        y={ly}
                        textAnchor="middle"
                        style={{
                            fontFamily: 'var(--font-mono)',
                            fontSize: '13px',
                            fill: BLUE_INK,
                            letterSpacing: '-0.5px'
                        }}
                    >
                        {fmt(value)}
                    </text>
                );
            }
        }

        // "0" label at the start
        const zeroY = labelAbove ? yOffset - tickHeight - 3 : yOffset + barHeight + tickHeight + 10;
        labels.push(
            <text
                key="label-zero"
                x={0}
                y={zeroY}
                textAnchor="middle"
                style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: '13px',
                    fill: BLUE_INK,
                    letterSpacing: '-0.5px'
                }}
            >
                0
            </text>
        );

        // Unit label at far right
        const unitY = yOffset + barHeight / 2 + 1;
        labels.push(
            <text
                key="unit"
                x={axis.barWidthPx + 6}
                y={unitY}
                textAnchor="start"
                dominantBaseline="middle"
                style={{
                    fontFamily: 'var(--font-main)',
                    fontStyle: 'italic',
                    fontSize: '14px',
                    fill: BLUE_INK,
                }}
            >
                {unit}
            </text>
        );

        return [...segments, ...ticks, ...labels];
    };

    // Compute total SVG dimensions (no cartouche frame)
    const maxBarWidth = Math.max(scaleData.km.barWidthPx, scaleData.nm.barWidthPx);
    const topBarY = 16; // space for km numbers above
    const bottomBarY = topBarY + barHeight + gap;
    const svgWidth = maxBarWidth + 30; // room for unit label
    const svgHeight = bottomBarY + barHeight + tickHeight + 16;

    return (
        <div
            style={{
                position: 'absolute',
                bottom: 20,
                left: 20,
                zIndex: 15, // Above paper (10), below labels (20)
                pointerEvents: 'none',
                opacity: 0.85,
            }}
        >
            <svg width={svgWidth} height={svgHeight}>
                {/* Hatching pattern definition for dark segments */}
                <defs>
                    <pattern
                        id="scale-hatch"
                        width={3}
                        height={3}
                        patternUnits="userSpaceOnUse"
                        patternTransform="rotate(45)"
                    >
                        <line
                            x1={0} y1={0} x2={0} y2={3}
                            stroke={BLUE_INK}
                            strokeWidth={1.2}
                        />
                    </pattern>
                </defs>

                {renderAxis(scaleData.km, 'km', topBarY, true)}
                {renderAxis(scaleData.nm, 'NM', bottomBarY, false)}
            </svg>
        </div>
    );
};
