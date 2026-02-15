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
        switch (type) {
            case 'balloon':
                return (
                    <g>
                        {/* Balloon Envelope - Main Body */}
                        <path
                            d="M50,10 C30,10 15,25 15,45 C15,60 30,75 50,85 C70,75 85,60 85,45 C85,25 70,10 50,10 Z"
                            fill={colorMain}
                            stroke="black"
                            strokeWidth="3"
                        />
                        {/* Chevron Patterns - Accent */}
                        <path
                            d="M20,40 L50,55 L80,40 M20,55 L50,70 L80,55"
                            fill="none"
                            stroke={colorAccent}
                            strokeWidth="3"
                        />
                        {/* Basket chords */}
                        <path d="M42,85 L42,92 M58,85 L58,92" stroke="black" strokeWidth="2" />
                        {/* Basket */}
                        <rect x="40" y="92" width="20" height="8" rx="2" fill={colorAccent} stroke="black" strokeWidth="2" />
                    </g>
                );
            case 'prop':
                return (
                    <g>
                        {/* Wings */}
                        <path d="M10,40 L90,40 L90,55 L10,55 Z" fill={colorMain} stroke="black" strokeWidth="3" />
                        {/* Fuselage */}
                        <path d="M45,15 L55,15 L55,85 L45,85 Z" fill={colorMain} stroke="black" strokeWidth="3" />
                        {/* Tail */}
                        <path d="M35,85 L65,85 L65,95 L35,95 Z" fill={colorAccent} stroke="black" strokeWidth="3" />
                        {/* Propeller/Nose */}
                        <path d="M40,15 L60,15 L50,5 Z" fill={colorAccent} stroke="black" strokeWidth="2" />
                    </g>
                );
            case 'twin_prop':
                return (
                    <g>
                        {/* Wings */}
                        <path d="M5,45 L95,45 L90,60 L10,60 Z" fill={colorMain} stroke="black" strokeWidth="3" />
                        {/* Fuselage */}
                        <path d="M46,10 L54,10 L54,90 L46,90 Z" fill={colorMain} stroke="black" strokeWidth="3" />
                        {/* Engines */}
                        <path d="M25,40 L35,40 L35,65 L25,65 Z" fill={colorAccent} stroke="black" strokeWidth="3" />
                        <path d="M65,40 L75,40 L75,65 L65,65 Z" fill={colorAccent} stroke="black" strokeWidth="3" />
                        {/* Tail */}
                        <path d="M30,85 L70,85 L60,95 L40,95 Z" fill={colorAccent} stroke="black" strokeWidth="3" />
                    </g>
                );
            case 'jet':
                return (
                    <g>
                        {/* Swept Wings */}
                        <path d="M50,45 L90,65 L90,75 L50,55 L10,75 L10,65 Z" fill={colorMain} stroke="black" strokeWidth="3" />
                        {/* Fuselage */}
                        <path d="M48,5 L52,5 L52,95 L48,95 Z" fill={colorMain} stroke="black" strokeWidth="3" />
                        {/* Engines (Rear mounted) */}
                        <path d="M40,70 L45,70 L45,85 L40,85 Z" fill={colorAccent} stroke="black" strokeWidth="2" />
                        <path d="M55,70 L60,70 L60,85 L55,85 Z" fill={colorAccent} stroke="black" strokeWidth="2" />
                        {/* T-Tail */}
                        <path d="M35,90 L65,90 L65,95 L35,95 Z" fill={colorAccent} stroke="black" strokeWidth="3" />
                    </g>
                );
            case 'airliner':
                return (
                    <g>
                        {/* Wings */}
                        <path d="M50,35 L95,65 L85,75 L50,50 L15,75 L5,65 Z" fill={colorMain} stroke="black" strokeWidth="3" />
                        {/* Fuselage */}
                        <path d="M45,5 L55,5 L55,95 L45,95 Z" fill={colorMain} stroke="black" strokeWidth="3" />
                        {/* Engines (Wing mounted) */}
                        <circle cx="30" cy="65" r="5" fill={colorAccent} stroke="black" strokeWidth="2" />
                        <circle cx="70" cy="65" r="5" fill={colorAccent} stroke="black" strokeWidth="2" />
                        {/* Tail */}
                        <path d="M40,85 L60,85 L65,95 L35,95 Z" fill={colorAccent} stroke="black" strokeWidth="3" />
                    </g>
                );
            case 'helicopter':
                return (
                    <g>
                        {/* Main Rotor Blades (Cross) */}
                        <rect x="48" y="5" width="4" height="90" fill={colorAccent} stroke="black" strokeWidth="2" />
                        <rect x="5" y="48" width="90" height="4" fill={colorAccent} stroke="black" strokeWidth="2" />
                        {/* Body */}
                        <ellipse cx="50" cy="50" rx="15" ry="25" fill={colorMain} stroke="black" strokeWidth="3" />
                        {/* Tail Boom */}
                        <rect x="48" y="70" width="4" height="25" fill={colorMain} stroke="black" strokeWidth="2" />
                        {/* Tail Rotor */}
                        <rect x="42" y="90" width="16" height="4" fill={colorAccent} stroke="black" strokeWidth="2" />
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
