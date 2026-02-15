import { lerpColor, adjustFont } from './mapUtils';

describe('mapUtils', () => {
    describe('lerpColor', () => {
        const cases = [
            { c1: '#000000', c2: '#ffffff', t: 0, expected: 'rgb(0, 0, 0)' },
            { c1: '#000000', c2: '#ffffff', t: 1, expected: 'rgb(255, 255, 255)' },
            { c1: '#000000', c2: '#ffffff', t: 0.5, expected: 'rgb(128, 128, 128)' },
            { c1: '#ff0000', c2: '#0000ff', t: 0.5, expected: 'rgb(128, 0, 128)' },
        ];

        cases.forEach(({ c1, c2, t, expected }) => {
            it(`interpolates ${c1} and ${c2} at t=${t} to ${expected}`, () => {
                expect(lerpColor(c1, c2, t)).toBe(expected);
            });
        });
    });

    describe('adjustFont', () => {
        it('increases font size', () => {
            const font = { font: '14px Inter', uppercase: false, letterSpacing: 0 };
            const result = adjustFont(font, 2);
            expect(result.font).toBe('16px Inter');
        });

        it('decreases font size but keeps it at least 1px', () => {
            const font = { font: '14px Inter', uppercase: false, letterSpacing: 0 };
            const result = adjustFont(font, -20);
            expect(result.font).toBe('1px Inter');
        });

        it('preserves other properties', () => {
            const font = { font: '14px Inter', uppercase: true, letterSpacing: 1.5 };
            const result = adjustFont(font, 0);
            expect(result.uppercase).toBe(true);
            expect(result.letterSpacing).toBe(1.5);
        });
    });
});
