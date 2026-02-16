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
                // Single Engine - High Wing - Cessna (Noun Project)
                // Zealous crop and centered
                // Main Body = Large Path (Sub 1) + Detail Path (Path 1)
                // Windows = Internal Sub-paths of Large Path (Sub 2 & 3)
                return (
                    <g transform="translate(50,50) scale(1.15) translate(-50,-51)">
                        {/* Main Body & Details - Main Color */}
                        <path
                            d="M93.904,35.638l-22.881-1.486c-0.018-0.001-0.037-0.001-0.057-0.001H55.199v-3.449c0-0.061-0.008-0.12-0.018-0.176c-0.111-0.798-0.576-4.21-0.729-6c-0.158-1.856-1.676-2.785-2.73-3.113l-0.891-2.779c-0.113-0.356-0.443-0.598-0.818-0.598l0,0c-0.373,0-0.705,0.241-0.82,0.598l-0.887,2.773c-1.057,0.324-2.588,1.253-2.748,3.118c-0.152,1.787-0.614,5.18-0.727,5.997c-0.012,0.058-0.018,0.118-0.018,0.179v3.449H29.031c-0.018,0-0.037,0-0.056,0.001L6.095,35.638c-0.453,0.03-0.805,0.405-0.805,0.857v8.916c0,0.43,0.317,0.793,0.743,0.852l22.881,3.157c0.039,0.005,0.078,0.009,0.117,0.009h15.875l2.369,20.886l-11.373,2.184c-0.405,0.078-0.698,0.432-0.698,0.846v5.992c0,0.426,0.311,0.787,0.73,0.85L47.92,82.02c0.043,0.006,0.087,0.008,0.129,0.008c0.209,0,0.412-0.074,0.57-0.213l1.389-1.227l1.369,1.223c0.191,0.17,0.449,0.246,0.703,0.209l11.984-1.834c0.42-0.063,0.73-0.424,0.73-0.85v-5.992c0-0.414-0.295-0.768-0.699-0.846l-11.359-2.18l2.371-20.89h15.859c0.041,0,0.078-0.004,0.117-0.009l22.883-3.157c0.426-0.059,0.742-0.421,0.742-0.852v-8.916C94.709,36.042,94.355,35.667,93.904,35.638z M50.014,30.694c-1.189,0-2.302,0.563-3.133,1.589c-0.063,0.078-0.096,0.173-0.096,0.272v0.722c0,0.237,0.193,0.43,0.431,0.43h5.595c0.238,0,0.43-0.193,0.43-0.43v-0.722c0-0.099-0.033-0.194-0.096-0.272C52.314,31.257,51.201,30.694,50.014,30.694z M52.381,32.847h-4.734v-0.135c0.65-0.748,1.486-1.158,2.367-1.158s1.717,0.41,2.367,1.158V32.847z"
                            fill={colorMain}
                            stroke={strokeColor}
                            strokeWidth="1"
                        />
                        {/* Windows - Secondary Color */}
                        <path
                            d="M92.988,44.661l-22.08,3.047H54.346c-0.004,0-0.008,0.001-0.012,0.001c-0.43,0.001-0.801,0.324-0.85,0.763l-0.244,2.152c0-0.031,0-0.063,0-0.094c-0.023-1.546-0.912-1.959-1.436-1.959c-0.006,0-0.012,0-0.016,0h-3.551c-0.004,0-0.011,0-0.016,0c-0.523,0-1.412,0.413-1.437,1.959c-0.011,0.666,0.222,1.259,0.671,1.716c0.586,0.594,1.541,0.949,2.557,0.949s1.971-0.355,2.557-0.949c0.314-0.32,0.521-0.709,0.615-1.141l-2.248,19.811c-0.049,0.422,0.219,0.807,0.613,0.922c0,0,0,0,0.002,0c0.023,0.006,0.049,0.014,0.074,0.02c0.002,0,0.006,0.002,0.008,0.002l11.439,2.195v4.543l-10.855,1.66l-1.637-1.461c-0.324-0.289-0.814-0.289-1.141-0.004l-1.658,1.465l-10.859-1.66v-4.543l11.458-2.199c0.441-0.086,0.744-0.496,0.693-0.941l-2.547-22.443c-0.051-0.443-0.432-0.771-0.87-0.764c-0.002,0-0.004,0-0.006,0H29.091L7.01,44.661v-7.359l22.049-1.431h16.594c0.003,0,0.007-0.001,0.011-0.001s0.006,0.001,0.01,0.001c0.476,0,0.859-0.386,0.859-0.86v-4.235c0.108-0.78,0.582-4.258,0.74-6.104c0.104-1.203,1.384-1.585,1.688-1.658h0.014c0.414,0,0.76-0.292,0.842-0.682l0.197-0.616l0.197,0.616c0.081,0.389,0.426,0.682,0.842,0.682c0.311,0.075,1.584,0.458,1.686,1.658c0.156,1.838,0.631,5.307,0.74,6.104v4.235c0,0.474,0.387,0.86,0.861,0.86l0.002-0.001c0.002,0,0.002,0.001,0.004,0.001h16.596l22.047,1.431V44.661z M52.381,50.542c0.008,0.438-0.137,0.807-0.424,1.1c-0.504,0.51-1.334,0.691-1.943,0.691c-0.61,0-1.441-0.182-1.943-0.691c-0.289-0.293-0.432-0.662-0.424-1.1c0.016-1.045,0.492-1.108,0.57-1.11c0.006,0,0.004,0,0.014,0h3.564c0.01,0,0.01,0,0.016,0C51.891,49.434,52.365,49.499,52.381,50.542z"
                            fill={colorAccent}
                            stroke={strokeColor}
                            strokeWidth="1"
                        />
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
