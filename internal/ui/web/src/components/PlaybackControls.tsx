import { } from 'react'; // Checking what else is there, usually standard React imports.
// If it was just "import { useRef } from 'react';" and nothing else, remove the line?
// Based on error log line 1: import { useRef } from 'react';
import { Pause, Play, Square, SkipForward, Volume2, RotateCcw } from 'lucide-react';
import { useAudio } from '../hooks/useAudio';
import { useNarrator } from '../hooks/useNarrator';
import type { AudioStatus } from '../types/audio';

interface PlaybackControlsProps {
    status?: AudioStatus;
}

export const PlaybackControls = ({ status: externalStatus }: PlaybackControlsProps) => {
    const { status: hookStatus, control, setVolume } = useAudio();
    const { status: narratorStatus } = useNarrator();

    // Use external status if provided, otherwise use hook status
    const status = externalStatus ?? hookStatus;

    const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const newVol = parseFloat(e.target.value);
        setVolume(newVol);
    };

    const isPlaying = status?.is_playing ?? false;
    const isPaused = status?.is_paused ?? false;
    const isUserPaused = status?.is_user_paused ?? false;
    const volume = status?.volume ?? 1.0;

    // Determine badge text/color
    let badgeText = "IDLE";
    let badgeColor = "#666";

    if (narratorStatus?.playback_status === 'preparing') {
        badgeText = "PREPARING";
        badgeColor = "#eab308"; // Yellow-500
    } else if (narratorStatus?.playback_status === 'playing') {
        badgeText = "PLAYING";
        badgeColor = "#22c55e"; // Green-500
    } else if (narratorStatus?.playback_status === 'paused' || isUserPaused) {
        badgeText = "PAUSED";
        badgeColor = "#f97316"; // Orange-500
    }

    // Determine Title
    // If not active/playing/preparing, maybe show nothing or "IDLE"
    const showTitle = narratorStatus?.playback_status && narratorStatus.playback_status !== 'idle';
    const displayTitle = narratorStatus?.current_title || "";

    return (
        <div className="playback-controls">
            {/* Controls Row */}
            <div className="playback-row">
                {/* Replay last narration */}
                <button
                    className="btn-icon"
                    onClick={() => control('replay')}
                    title="Replay Last Narration"
                >
                    <RotateCcw size={18} />
                </button>

                {/* Play/Pause */}
                {isUserPaused ? (
                    <button
                        className="btn-icon btn-icon--active"
                        onClick={() => control('resume')}
                        title="Resume Auto-Select"
                    >
                        <Play size={18} />
                    </button>
                ) : (
                    <button
                        className="btn-icon"
                        onClick={() => control('pause')}
                        title="Pause Auto-Select"
                    >
                        <Pause size={18} />
                    </button>
                )}

                {/* Stop */}
                <button
                    className="btn-icon"
                    onClick={() => control('stop')}
                    disabled={!isPlaying && !isPaused}
                    title="Stop"
                >
                    <Square size={18} />
                </button>

                {/* Skip */}
                <button
                    className="btn-icon"
                    onClick={() => control('skip')}
                    title="Skip / Next"
                >
                    <SkipForward size={18} />
                </button>

                {/* Volume Slider */}
                <div className="volume-control">
                    <Volume2 size={14} />
                    <input
                        type="range"
                        min="0"
                        max="1"
                        step="0.05"
                        value={volume}
                        onChange={handleVolumeChange}
                        className="volume-slider"
                    />
                </div>

                {/* Status Badge */}
                <div style={{
                    marginLeft: '8px',
                    padding: '2px 6px',
                    borderRadius: '4px',
                    backgroundColor: 'rgba(255,255,255,0.1)',
                    border: `1px solid ${badgeColor}`,
                    color: badgeColor,
                    fontSize: '10px',
                    fontWeight: 600,
                    letterSpacing: '0.5px',
                    minWidth: 'fit-content'
                }}>
                    {badgeText}
                </div>
            </div>

            {/* Title Row with Progress */}
            {showTitle && displayTitle && (
                <div className="playback-title" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px' }}>
                    {/* Circular Progress */}
                    {status?.duration && status.duration > 0 && (
                        <div style={{ position: 'relative', width: '14px', height: '14px', flexShrink: 0 }}>
                            <svg width="14" height="14" viewBox="0 0 24 24" style={{ transform: 'rotate(-90deg)' }}>
                                <circle
                                    cx="12"
                                    cy="12"
                                    r="10"
                                    fill="none"
                                    stroke="#444"
                                    strokeWidth="4"
                                />
                                <circle
                                    cx="12"
                                    cy="12"
                                    r="10"
                                    fill="none"
                                    stroke={isPaused ? '#fbc02d' : '#4caf50'}
                                    strokeWidth="4"
                                    strokeDasharray={`${2 * Math.PI * 10}`}
                                    strokeDashoffset={`${2 * Math.PI * 10 * (1 - (status.position / status.duration))}`}
                                    strokeLinecap="round"
                                    style={{ transition: 'stroke-dashoffset 0.5s linear' }}
                                />
                            </svg>
                        </div>
                    )}

                    <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {displayTitle}
                    </span>

                    {/* Time */}
                    {status?.duration && status.duration > 0 && (
                        <span style={{ fontSize: '10px', color: '#888', fontFamily: 'monospace', flexShrink: 0 }}>
                            -{Math.floor(status.duration - status.position)}s
                        </span>
                    )}
                </div>
            )}
        </div>
    );
};
