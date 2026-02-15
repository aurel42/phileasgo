import React from 'react';
import type maplibregl from 'maplibre-gl';
import { InkTrail } from '../InkTrail';
import { AircraftIcon } from '../AircraftIcon';
import type { AircraftType } from '../AircraftIcon';

interface ArtisticMapAircraftProps {
    map: React.MutableRefObject<maplibregl.Map | null>;
    effectiveReplayMode: boolean;
    pathPoints: [number, number][];
    validEvents: any[];
    progress: number;
    departure: [number, number] | null;
    destination: [number, number] | null;
    replayBalloonPos: { x: number, y: number } | null;
    frame: any;
    aircraftIcon: AircraftType;
    aircraftSize: number;
    aircraftColorMain: string;
    aircraftColorAccent: string;
}

export const ArtisticMapAircraft: React.FC<ArtisticMapAircraftProps> = React.memo(({
    map, effectiveReplayMode, pathPoints, validEvents, progress,
    departure, destination, replayBalloonPos, frame,
    aircraftIcon, aircraftSize, aircraftColorMain, aircraftColorAccent
}) => {
    return (
        <div style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 20 }}>
            {effectiveReplayMode && pathPoints.length >= 2 && map.current && (
                <div style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 10 }}>
                    <InkTrail
                        pathPoints={pathPoints}
                        validEvents={validEvents}
                        progress={progress}
                        departure={departure}
                        destination={destination}
                        project={(lnglat) => {
                            const pt = map.current!.project(lnglat);
                            return { x: pt.x, y: pt.y };
                        }}
                    />
                </div>
            )}
            <AircraftIcon
                type={aircraftIcon}
                x={effectiveReplayMode && replayBalloonPos ? replayBalloonPos.x : frame.aircraftX}
                y={effectiveReplayMode && replayBalloonPos ? replayBalloonPos.y : frame.aircraftY}
                agl={effectiveReplayMode ? 5000 : frame.agl}
                heading={frame.heading}
                size={aircraftSize}
                colorMain={aircraftColorMain}
                colorAccent={aircraftColorAccent}
            />
        </div>
    );
});
