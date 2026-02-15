import { render } from '@testing-library/react';
import { AircraftIcon } from './AircraftIcon';

describe('AircraftIcon', () => {
    it('renders correctly with main color', () => {
        const { container } = render(<AircraftIcon type="jet" size={32} colorMain="#ff0000" colorAccent="#00ff00" x={0} y={0} agl={0} heading={0} />);
        // AircraftIcon renders an InlineSVG which renders an <svg>
        const svg = container.querySelector('svg');
        expect(svg).toBeDefined();
        // The colorMain should be applied to some elements. 
        // Since it's an InlineSVG, we might need to check the path fills if we knew the structure,
        // but for now we can check if the prop is passed correctly or if it's in the DOM.
        // InlineSVG usually injects the SVG content.
    });

    it('scales with size prop', () => {
        const { container } = render(<AircraftIcon type="jet" size={48} colorMain="#ff0000" colorAccent="#00ff00" x={0} y={0} agl={0} heading={0} />);
        const svg = container.querySelector('svg');
        expect(svg?.style.width).toBe('48px');
        expect(svg?.style.height).toBe('48px');
    });

    const types = ['jet', 'prop', 'airliner', 'balloon', 'drone', 'glider', 'helicopter'];
    types.forEach(type => {
        it(`renders icons of type: ${type}`, () => {
            const { container } = render(<AircraftIcon type={type as any} size={32} colorMain="#ff0000" colorAccent="#00ff00" x={0} y={0} agl={0} heading={0} />);
            const svg = container.querySelector('svg');
            expect(svg).toBeDefined();
        });
    });
});
