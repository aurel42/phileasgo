const fontCache = new Map<string, string>();

/**
 * Extracts the computed font string for a given CSS class.
 * Uses a hidden probe element to ensure single source of truth from CSS.
 */
export function getFontFromClass(className: string): { font: string, uppercase: boolean, letterSpacing: number } {
    const cacheKey = `role:${className}`;
    if (fontCache.has(cacheKey)) {
        return JSON.parse(fontCache.get(cacheKey)!);
    }

    const probe = document.createElement('div');
    probe.className = className;
    probe.style.position = 'absolute';
    probe.style.visibility = 'hidden';
    probe.style.pointerEvents = 'none';
    document.body.appendChild(probe);

    const style = window.getComputedStyle(probe);
    const font = `${style.fontStyle} ${style.fontVariant} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`.trim();
    const uppercase = style.textTransform === 'uppercase';
    const letterSpacing = parseFloat(style.letterSpacing) || 0; // px value, 0 if "normal"

    document.body.removeChild(probe);
    const result = { font, uppercase, letterSpacing };
    fontCache.set(cacheKey, JSON.stringify(result));
    return result;
}

let canvas: HTMLCanvasElement | null = null;
let context: CanvasRenderingContext2D | null = null;

function ensureContext() {
    if (!context) {
        canvas = document.createElement('canvas');
        context = canvas.getContext('2d');
    }
    return context;
}

const measurementCache = new Map<string, { width: number, height: number }>();

/**
 * Measures the pixel width and approximate height of a text string with a given font.
 * Caches results for performance.
 */
export function measureText(text: string, font: string, letterSpacing: number = 0): { width: number, height: number } {
    const key = `${text}:${font}:${letterSpacing}`;
    if (measurementCache.has(key)) {
        return measurementCache.get(key)!;
    }

    const ctx = ensureContext();
    if (!ctx) {
        return { width: text.length * 8, height: 16 }; // Fallback
    }

    ctx.font = font;
    const metrics = ctx.measureText(text);

    // Height is tricky in canvas. 'actualBoundingBoxAscent' + 'actualBoundingBoxDescent' is precise but newer.
    // Fallback to estimation if needed, but modern browsers support these.
    const height = (metrics.actualBoundingBoxAscent + metrics.actualBoundingBoxDescent) || 20;
    // Canvas measureText doesn't account for CSS letter-spacing; add it manually
    const width = metrics.width + (letterSpacing * text.length);

    const result = { width: Math.ceil(width), height: Math.ceil(height) };
    measurementCache.set(key, result);
    return result;
}

export function clearMeasurementCache() {
    measurementCache.clear();
}

export function __resetForTests() {
    context = null;
    canvas = null;
    measurementCache.clear();
}
