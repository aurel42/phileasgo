import { render, screen } from '@testing-library/react';
import { POIBeacon } from './POIBeacon';

describe('POIBeacon', () => {
    it('renders correctly with given color', () => {
        render(<POIBeacon color="#ff0000" size={12} x={100} y={100} />);
        const svg = screen.getByTestId('poi-beacon');
        const path = svg.querySelector('path');
        expect(path?.getAttribute('fill')).toBe('#ff0000');
    });

    it('calculates dynamic offset based on iconSize', () => {
        const iconSize = 40;
        // Expected translateY: -((40 / 2) + 2 + 7.5) = -29.5
        render(<POIBeacon color="#ff0000" size={12} x={100} y={100} iconSize={iconSize} zoomScale={1} />);
        const svg = screen.getByTestId('poi-beacon');
        const transform = svg.style.transform;
        expect(transform).toContain('translateY(-29.5px)');
    });

    it('applies zoomScale to transform', () => {
        render(<POIBeacon color="#ff0000" size={12} x={100} y={100} zoomScale={2.5} />);
        const svg = screen.getByTestId('poi-beacon');
        const transform = svg.style.transform;
        expect(transform).toContain('scale(2.5)');
    });

    it('positions correctly using left and top', () => {
        render(<POIBeacon color="#ff0000" size={12} x={150} y={250} />);
        const svg = screen.getByTestId('poi-beacon');
        expect(svg.style.left).toBe('150px');
        expect(svg.style.top).toBe('250px');
    });
});
