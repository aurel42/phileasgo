import React from 'react';

export type AircraftType = 'balloon' | 'prop' | 'twin_prop' | 'jet' | 'airliner' | 'helicopter';

interface AircraftIconProps {
    type: AircraftType;
    x: number;
    y: number;
    agl: number;     // Altitude Above Ground Level (feet)
    heading: number; // 0-360 degrees
    size: number;    // Base size in pixels (e.g. 32)
    colorMain: string;
    colorAccent: string;
}

export const AircraftIcon: React.FC<AircraftIconProps> = ({
    type,
    x,
    y,
    agl,
    heading,
    size,
    colorMain,
    colorAccent
}) => {
    // 1. Calculate Shadow (Altitude Effect)
    // Interpolation (0 -> 10,000 ft) for shadow offset and scale
    const ratio = Math.min(Math.max(agl / 10000, 0), 1);
    const shadowOffset = ratio * (size * 0.6); // Scale offset with size
    const shadowScale = 1 - (ratio * 0.5);

    // 2. Determine Rotation
    // Balloon usually doesn't rotate with heading (drifts), but let's keep it upright or follow wind.
    // For now, we'll only rotate non-balloon types, OR rotate balloon if user wants.
    // Standard mockups show balloon upright.
    const rotation = type === 'balloon' ? 0 : heading;

    // 3. SVG Paths for different types
    // ViewBox is 0 0 100 100 for all icons to standardize
    const renderIcon = () => {
        // Common styles
        const strokeW = 1.5;
        const strokeColor = "black";

        switch (type) {
            case 'balloon':
                return (
                    <g>
                        {/* Balloon Envelope */}
                        <path
                            d="M50,10 C30,10 15,25 15,45 C15,60 30,75 50,85 C70,75 85,60 85,45 C85,25 70,10 50,10 Z"
                            fill={colorMain}
                            stroke={strokeColor}
                            strokeWidth={strokeW}
                        />
                        {/* Decorative Bands (Accent) */}
                        <path
                            d="M20,40 L50,55 L80,40 M20,55 L50,70 L80,55"
                            fill="none"
                            stroke={colorAccent}
                            strokeWidth={strokeW}
                        />
                        {/* Basket chords */}
                        <path d="M42,85 L42,92 M58,85 L58,92" stroke={strokeColor} strokeWidth={strokeW} />
                        {/* Basket */}
                        <rect x="40" y="92" width="20" height="8" rx="2" fill={colorAccent} stroke={strokeColor} strokeWidth={strokeW} />
                    </g>
                );
            case 'prop':
                // Single Engine - High Wing, Fixed Gear (Cessna-ish)
                // Focusing on bulkier fuselage and shorter, wider wings.
                return (
                    <g>
                        {/* Fuselage - Stout cigar shape */}
                        <path d="M42,15 C42,10 58,10 58,15 L56,80 L50,95 L44,80 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Wings - Short span, wide chord, rectangular extended */}
                        <path d="M10,38 L90,38 L90,52 L10,52 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Wing Details - Accent Stripes (Filled) */}
                        <rect x="15" y="38" width="8" height="14" fill={colorAccent} stroke="none" />
                        <rect x="77" y="38" width="8" height="14" fill={colorAccent} stroke="none" />

                        {/* Horizontal Stabilizer */}
                        <path d="M30,82 L70,82 L70,90 L30,90 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Cockpit Window - Filled */}
                        <path d="M43,28 L57,28 L57,35 L43,35 Z" fill={colorAccent} stroke="none" />
                        <path d="M43,28 L57,28 L57,35 L43,35 Z" fill="none" stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Propeller Hub */}
                        <circle cx="50" cy="12" r="3" fill={colorAccent} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Prop Arc (Subtle) */}
                        <path d="M30,12 Q50,5 70,12" fill="none" stroke={strokeColor} strokeWidth="0.5" opacity="0.5" />
                    </g>
                );
            case 'twin_prop':
                // Twin Engine - Low Wing (Baron/Seneca)
                // Bulkier fuselage, proper wing shapes
                return (
                    <g>
                        {/* Fuselage - Tapered nose, wider body */}
                        <path d="M44,10 C44,5 56,5 56,10 L55,80 L50,95 L45,80 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Wings - Tapered, mid-span */}
                        <path d="M5,45 L95,45 L90,58 L10,58 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Engines - Nacelles on wings */}
                        <path d="M25,35 L35,35 L35,60 L25,60 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />
                        <path d="M65,35 L75,35 L75,60 L65,60 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Engine Details - Accent fronts */}
                        <rect x="25" y="35" width="10" height="5" fill={colorAccent} stroke="none" />
                        <rect x="65" y="35" width="10" height="5" fill={colorAccent} stroke="none" />

                        {/* Horizontal Stabilizer */}
                        <path d="M32,85 L68,85 L66,92 L34,92 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Cockpit Window */}
                        <path d="M45,20 L55,20 L54,26 L46,26 Z" fill={colorAccent} stroke={strokeColor} strokeWidth={strokeW} />
                    </g>
                );
            case 'jet':
                // Private Jet - T-Tail, Rear Engines
                // Wider fuselage, swept wings
                return (
                    <g>
                        {/* Wings - Swept back */}
                        <path d="M45,45 L5,65 L5,75 L45,60 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />
                        <path d="M55,45 L95,65 L95,75 L55,60 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Fuselage - Bullet shape, wide */}
                        <path d="M43,10 C43,5 57,5 57,10 L56,80 L50,95 L44,80 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Rear Engines - Nacelles */}
                        <path d="M36,70 L42,70 L42,85 L36,85 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />
                        <path d="M58,70 L64,70 L64,85 L58,85 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Engine Intakes - Accent */}
                        <rect x="36" y="70" width="6" height="3" fill={colorAccent} stroke="none" />
                        <rect x="58" y="70" width="6" height="3" fill={colorAccent} stroke="none" />

                        {/* T-Tail */}
                        <path d="M35,88 L65,88 L65,95 L35,95 Z" fill={colorAccent} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Cockpit Window */}
                        <path d="M44,25 L56,25 L55,32 L45,32 Z" fill={colorAccent} stroke={strokeColor} strokeWidth={strokeW} />
                    </g>
                );
            case 'airliner':
                // 4-Engine Heavy - Wide body
                return (
                    <g>
                        {/* Wings - Swept, large span */}
                        <path d="M45,40 L2,70 L8,80 L45,60 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />
                        <path d="M55,40 L98,70 L92,80 L55,60 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Fuselage - Tube */}
                        <path d="M40,15 C40,8 60,8 60,15 L60,85 L50,98 L40,85 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Engines - 4 Pods under wings (Circles/Ovals) */}
                        <ellipse cx="25" cy="65" rx="3.5" ry="5" fill={colorAccent} stroke={strokeColor} strokeWidth={strokeW} />
                        <ellipse cx="75" cy="65" rx="3.5" ry="5" fill={colorAccent} stroke={strokeColor} strokeWidth={strokeW} />
                        <ellipse cx="15" cy="72" rx="3.5" ry="5" fill={colorAccent} stroke={strokeColor} strokeWidth={strokeW} />
                        <ellipse cx="85" cy="72" rx="3.5" ry="5" fill={colorAccent} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Tail */}
                        <path d="M35,85 L65,85 L70,95 L30,95 Z" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Cockpit Window Band */}
                        <path d="M42,20 L58,20 L58,25 L42,25 Z" fill={colorAccent} stroke="none" />
                    </g>
                );
            case 'helicopter':
                // Rotorcraft
                // Diagonal blades, robust body
                return (
                    <g>
                        {/* Skids - Parallel bars */}
                        <path d="M35,35 L35,65" stroke={strokeColor} strokeWidth="2" />
                        <path d="M65,35 L65,65" stroke={strokeColor} strokeWidth="2" />
                        <path d="M30,40 L70,40" stroke={strokeColor} strokeWidth="2" /> {/* Cross bar front */}
                        <path d="M30,60 L70,60" stroke={strokeColor} strokeWidth="2" /> {/* Cross bar rear */}

                        {/* Tail Boom */}
                        <path d="M47,65 L47,90" stroke={colorMain} strokeWidth="6" strokeLinecap="round" />
                        <path d="M47,65 L47,90" stroke={strokeColor} strokeWidth="6" strokeLinecap="round" opacity="0.2" /> {/* Outline/Shade */}

                        {/* Body - Egg shape */}
                        <ellipse cx="50" cy="50" rx="14" ry="18" fill={colorMain} stroke={strokeColor} strokeWidth={strokeW} />

                        {/* Cockpit Bubble - Accent */}
                        <path d="M40,38 Q50,32 60,38 L60,50 Q50,55 40,50 Z" fill={colorAccent} stroke="none" />

                        {/* Tail Rotor */}
                        <rect x="42" y="88" width="16" height="3" fill={colorAccent} stroke={strokeColor} strokeWidth="1" />

                        {/* Main Rotor - Diagonal X */}
                        <path d="M20,20 L80,80" stroke={strokeColor} strokeWidth="3" />
                        <path d="M80,20 L20,80" stroke={strokeColor} strokeWidth="3" />
                        <path d="M20,20 L80,80" stroke={colorAccent} strokeWidth="1.5" />
                        <path d="M80,20 L20,80" stroke={colorAccent} strokeWidth="1.5" />

                        {/* Rotor Hub */}
                        <circle cx="50" cy="50" r="4" fill={strokeColor} />
                    </g>
                );
            default:
                return null;
        }
    };

    return (
        <div style={{ position: 'absolute', left: 0, top: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 100 }}>
            {/* Shadow: Soft grey, offset based on altitude */}
            <svg
                viewBox="0 0 100 100"
                style={{
                    position: 'absolute',
                    left: x - (size * shadowScale) / 2 + shadowOffset,
                    top: y - (size * shadowScale) / 2 + shadowOffset,
                    width: size * shadowScale,
                    height: size * shadowScale,
                    filter: 'blur(3px)',
                    opacity: 0.3,
                    transform: `rotate(${rotation}deg)`
                }}
            >
                {/* Shadow uses simple black fill for whatever shape is rendered */}
                <g fill="black">
                    {renderIcon()}
                </g>
            </svg>

            {/* Aircraft Icon */}
            <svg
                viewBox="0 0 100 100"
                style={{
                    position: 'absolute',
                    left: x - size / 2,
                    top: y - size / 2,
                    width: size,
                    height: size,
                    transform: `rotate(${rotation}deg)`,
                    filter: 'drop-shadow(0px 2px 2px rgba(0,0,0,0.3))'
                }}
            >
                {renderIcon()}
            </svg>
        </div>
    );
};
