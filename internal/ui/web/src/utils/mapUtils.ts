/**
 * Linearly interpolates between two hex colors.
 */
export const lerpColor = (c1: string, c2: string, t: number): string => {
    const hex = (c: string) => parseInt(c.slice(1), 16);
    const r1 = (hex(c1) >> 16) & 255;
    const g1 = (hex(c1) >> 8) & 255;
    const b1 = (hex(c1)) & 255;
    const r2 = (hex(c2) >> 16) & 255;
    const g2 = (hex(c2) >> 8) & 255;
    const b2 = (hex(c2)) & 255;
    const r = Math.round(r1 + (r2 - r1) * t);
    const g = Math.round(g1 + (g2 - g1) * t);
    const b = Math.round(b1 + (b2 - b1) * t);
    return `rgb(${r}, ${g}, ${b})`;
};

/**
 * Adjusts font size in a font description string.
 */
export const adjustFont = (f: { font: string, uppercase: boolean, letterSpacing: number }, offset: number) => ({
    ...f,
    font: f.font.replace(/(\d+)px/, (_, s) => `${Math.max(1, parseInt(s) + offset)}px`)
});
