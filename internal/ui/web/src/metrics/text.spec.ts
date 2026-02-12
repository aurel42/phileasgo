import { describe, it, expect, vi, beforeEach } from 'vitest';
import { measureText, getFontFromClass, __resetForTests } from './text';

describe('text.ts utility', () => {
    beforeEach(() => {
        __resetForTests();
        vi.restoreAllMocks();
    });

    describe('measureText', () => {
        it('should return estimated dimensions when context is missing', () => {
            // Mock getContext to return null
            const spy = vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null as any);

            const result = measureText('Hello', '16px serif');
            expect(result).toEqual({ width: 40, height: 16 }); // 5 chars * 8

            spy.mockRestore();
        });

        it('should use canvas context to measure text', () => {
            const mockContext = {
                font: '',
                measureText: vi.fn().mockReturnValue({
                    width: 50,
                    actualBoundingBoxAscent: 10,
                    actualBoundingBoxDescent: 5
                })
            };

            vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(mockContext as any);

            const result = measureText('Hello', '16px serif', 2);

            expect(mockContext.font).toBe('16px serif');
            // width = 50 + (2 * 5 chars) = 60
            // height = 10 + 5 = 15
            expect(result).toEqual({ width: 60, height: 15 });
        });

        it('should cache measurement results', () => {
            const measureTextSpy = vi.fn().mockReturnValue({
                width: 50,
                actualBoundingBoxAscent: 10,
                actualBoundingBoxDescent: 5
            });

            vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
                font: '',
                measureText: measureTextSpy
            } as any);

            // First call
            measureText('CacheMe', '16px serif');
            // Second call with same params
            measureText('CacheMe', '16px serif');

            expect(measureTextSpy).toHaveBeenCalledTimes(1);
        });
    });

    describe('getFontFromClass', () => {
        it('should extract font properties from a CSS class using a probe element', () => {
            const mockStyle = {
                fontStyle: 'italic',
                fontVariant: 'normal',
                fontWeight: 'bold',
                fontSize: '14px',
                fontFamily: 'Inter',
                textTransform: 'uppercase',
                letterSpacing: '1.5px'
            };

            vi.spyOn(window, 'getComputedStyle').mockReturnValue(mockStyle as any);

            const result = getFontFromClass('my-test-class');

            expect(result.font).toBe('italic normal bold 14px Inter');
            expect(result.uppercase).toBe(true);
            expect(result.letterSpacing).toBe(1.5);
        });

        it('should handle "normal" letter spacing as 0', () => {
            const mockStyle = {
                fontStyle: 'normal',
                fontVariant: 'normal',
                fontWeight: '400',
                fontSize: '12px',
                fontFamily: 'sans-serif',
                textTransform: 'none',
                letterSpacing: 'normal'
            };

            vi.spyOn(window, 'getComputedStyle').mockReturnValue(mockStyle as any);

            const result = getFontFromClass('basic-class');
            expect(result.letterSpacing).toBe(0);
        });
    });
});
