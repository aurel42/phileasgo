import { useEffect, useRef } from 'react';
import { Marker } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';

interface AircraftMarkerProps {
    lat: number;
    lon: number;
    heading: number;
}

const planeSvg = `
<svg viewBox="0 0 512 512" width="48" height="48" style="display: block; filter: drop-shadow(0px 0px 4px rgba(0,0,0,0.5));">
    <path fill="#FFD700" stroke="black" stroke-width="10" d="M256 32 C 240 32, 230 50, 230 70 L 230 160 L 32 190 L 32 230 L 230 250 L 230 380 L 130 420 L 130 460 L 256 440 L 382 460 L 382 420 L 282 380 L 282 250 L 480 230 L 480 190 L 282 160 L 282 70 C 282 50, 272 32, 256 32 Z" />
</svg>
`;

export const AircraftMarker = ({ lat, lon, heading }: AircraftMarkerProps) => {
    const markerRef = useRef<L.Marker>(null);

    // Create custom icon
    const icon = L.divIcon({
        className: 'aircraft-marker',
        html: `<div class="plane-icon" style="display: flex; justify-content: center; align-items: center; width: 100%; height: 100%; transform: rotate(${heading}deg); transition: transform 0.1s linear; transform-origin: center;">${planeSvg}</div>`,
        iconSize: [48, 48],
        iconAnchor: [24, 24],
        zIndexOffset: 1000,
    } as L.DivIconOptions);

    useEffect(() => {
        if (markerRef.current) {
            markerRef.current.setLatLng([lat, lon]);
        }
    }, [lat, lon]);

    return <Marker position={[lat, lon]} icon={icon} ref={markerRef} pane="aircraftPane" interactive={false} />;
};
