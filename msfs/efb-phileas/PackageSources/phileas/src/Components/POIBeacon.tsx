import { ComponentProps, DisplayComponent, FSComponent, VNode } from "@microsoft/msfs-sdk";

export interface POIBeaconProps extends ComponentProps {
    color: string;
    size: number;
    showHalo?: boolean;
}

export class POIBeacon extends DisplayComponent<POIBeaconProps> {
    public render(): VNode {
        const { color, size, showHalo = true } = this.props;

        return (
            <div class="poi-beacon-wrapper" style="position: absolute; transform: translate(-50%, -100%); pointer-events: none;">
                {/* Halo Effect */}
                {showHalo && (
                    <div
                        style={{
                            position: 'absolute',
                            left: '50%',
                            top: '50%',
                            width: `${size * 2}px`,
                            height: `${size * 2}px`,
                            background: `radial-gradient(circle, ${color}66 0%, transparent 70%)`,
                            transform: 'translate(-50%, -50%)',
                            borderRadius: '50%'
                        }}
                    />
                )}

                {/* The Balloon / Pin */}
                <svg
                    width={size}
                    height={size * 1.5}
                    viewBox="0 0 100 150"
                    style={{ position: 'relative', display: 'block' }}
                >
                    <defs>
                        <filter id="beacon-shadow" x="-20%" y="-20%" width="140%" height="140%">
                            <feGaussianBlur in="SourceAlpha" stdDeviation="5" />
                            <feOffset dx="0" dy="2" result="offsetblur" />
                            <feComponentTransfer>
                                <feFuncA type="linear" slope="0.3" />
                            </feComponentTransfer>
                            <feMerge>
                                <feMergeNode />
                                <feMergeNode in="SourceGraphic" />
                            </feMerge>
                        </filter>
                    </defs>

                    {/* Balloon Path */}
                    <path
                        d="M50,140 C50,140 10,90 10,50 A40,40 0 1,1 90,50 C90,90 50,140 50,140 Z"
                        fill={color}
                        stroke="white"
                        stroke-width="5"
                        filter="url(#beacon-shadow)"
                    />

                    {/* Inner Circle / Highlight */}
                    <circle
                        cx="50"
                        cy="50"
                        r="15"
                        fill="white"
                        opacity="0.5"
                    />
                </svg>
            </div>
        );
    }
}
