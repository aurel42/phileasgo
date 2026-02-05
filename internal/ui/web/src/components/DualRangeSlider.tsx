import React, { useCallback, useEffect, useState, useRef } from 'react';

interface DualRangeSliderProps {
    min: number;
    max: number;
    step?: number;
    minVal: number;
    maxVal: number;
    onChange: (min: number, max: number) => void;
}

export const DualRangeSlider: React.FC<DualRangeSliderProps> = ({
    min,
    max,
    step = 1,
    minVal,
    maxVal,
    onChange,
}) => {
    const [minValState, setMinValState] = useState(minVal);
    const [maxValState, setMaxValState] = useState(maxVal);
    const minRef = useRef<HTMLInputElement>(null);
    const maxRef = useRef<HTMLInputElement>(null);
    const range = useRef<HTMLDivElement>(null);

    // Convert value to percentage
    const getPercent = useCallback(
        (value: number) => Math.round(((value - min) / (max - min)) * 100),
        [min, max]
    );

    // Set width of the range to decrease from the left side
    useEffect(() => {
        if (maxRef.current) {
            const minPercent = getPercent(minValState);
            const maxPercent = getPercent(maxValState);

            if (range.current) {
                range.current.style.left = `${minPercent}%`;
                range.current.style.width = `${maxPercent - minPercent}%`;
            }
        }
    }, [minValState, maxValState, getPercent]);

    // Handle min value change
    const handleMinChange = (event: React.ChangeEvent<HTMLInputElement>) => {
        const value = Math.min(Number(event.target.value), maxValState - step);
        setMinValState(value);
        onChange(value, maxValState);
    };

    // Handle max value change
    const handleMaxChange = (event: React.ChangeEvent<HTMLInputElement>) => {
        const value = Math.max(Number(event.target.value), minValState + step);
        setMaxValState(value);
        onChange(minValState, value);
    };

    return (
        <div className="v-dual-slider">
            <input
                type="range"
                min={min}
                max={max}
                step={step}
                value={minValState}
                ref={minRef}
                onChange={handleMinChange}
                className={`v-thumb v-thumb-left ${minValState > max - 100 ? 'v-thumb-z' : ''}`}
            />
            <input
                type="range"
                min={min}
                max={max}
                step={step}
                value={maxValState}
                ref={maxRef}
                onChange={handleMaxChange}
                className="v-thumb v-thumb-right"
            />

            <div className="v-slider">
                <div className="v-slider-track" />
                <div ref={range} className="v-slider-range" />
                <div className="v-slider-left-value">{minValState}m</div>
                <div className="v-slider-right-value">{maxValState}m</div>
            </div>

            <style>{`
                .v-dual-slider {
                    position: relative;
                    width: 100%;
                    height: 40px;
                    display: flex;
                    align-items: center;
                }

                .v-thumb {
                    position: absolute;
                    width: 100%;
                    pointer-events: none;
                    height: 0;
                    outline: none;
                    appearance: none;
                    z-index: 3;
                    background: transparent;
                }

                .v-thumb-z {
                    z-index: 5;
                }

                .v-thumb::-webkit-slider-thumb {
                    appearance: none;
                    background: var(--accent);
                    border: 1px solid #000;
                    border-radius: 50%;
                    cursor: pointer;
                    height: 18px;
                    width: 18px;
                    margin-top: -9px;
                    pointer-events: auto;
                    box-shadow: 0 0 5px rgba(0,0,0,0.5);
                    transition: transform 0.1s;
                }

                .v-thumb::-webkit-slider-thumb:hover {
                    transform: scale(1.2);
                }

                .v-slider {
                    position: relative;
                    width: 100%;
                }

                .v-slider-track {
                    position: absolute;
                    border-radius: 3px;
                    height: 4px;
                    background-color: #333;
                    width: 100%;
                    z-index: 1;
                }

                .v-slider-range {
                    position: absolute;
                    border-radius: 3px;
                    height: 4px;
                    background-color: var(--accent);
                    z-index: 2;
                }

                .v-slider-left-value,
                .v-slider-right-value {
                    position: absolute;
                    color: var(--accent);
                    font-size: 10px;
                    font-family: var(--font-mono);
                    top: 12px;
                }

                .v-slider-left-value { left: 0; }
                .v-slider-right-value { right: 0; }
            `}</style>
        </div>
    );
};
