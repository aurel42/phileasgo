
const canvas = document.createElement('canvas');
const context = canvas.getContext('2d');

const measurementCache = new Map<string, { width: number, height: number }>();

/**
 * Measures the pixel width and approximate height of a text string with a given font.
 * Caches results for performance.
 */
export function measureText(text: string, font: string): { width: number, height: number } {
    const key = `${text}:${font}`;
    if (measurementCache.has(key)) {
        return measurementCache.get(key)!;
    }

    if (!context) {
        return { width: text.length * 8, height: 16 }; // Fallback
    }

    context.font = font;
    const metrics = context.measureText(text);

    // Height is tricky in canvas. 'actualBoundingBoxAscent' + 'actualBoundingBoxDescent' is precise but newer.
    // Fallback to estimation if needed, but modern browsers support these.
    const height = (metrics.actualBoundingBoxAscent + metrics.actualBoundingBoxDescent) || 20;
    const width = metrics.width;

    const result = { width: Math.ceil(width), height: Math.ceil(height) };
    measurementCache.set(key, result);
    return result;
}

export function clearMeasurementCache() {
    measurementCache.clear();
}
