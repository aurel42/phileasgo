
import { useEffect, useState } from 'react';
import type { POI } from '../hooks/usePOIs';
import { useQueryClient } from '@tanstack/react-query';
import type { AudioStatus } from '../types/audio';

interface POIInfoPanelProps {
    poi: POI | null;
    pois: POI[];  // Fresh POI list from polling
    aircraftHeading: number;
    onClose: () => void;
}

const getColor = (score: number) => {
    const clamped = Math.max(1, Math.min(50, score));
    const ratio = (clamped - 1) / 49;
    const hue = 60 - (ratio * 60);
    return `hsl(${hue}, 100%, 50%)`;
};

const getName = (poi: POI) => {
    if (poi.name_user) return poi.name_user;
    if (poi.name_en) return poi.name_en;
    return poi.name_local || 'Unknown';
};

const getLocalNameIfDifferent = (poi: POI, primaryName: string) => {
    if (poi.name_local && poi.name_local !== primaryName) {
        return poi.name_local;
    }
    return null;
};

const formatTimeAgo = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);

    if (seconds < 60) return 'Just now';
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
};

export const POIInfoPanel = ({ poi, pois, onClose }: POIInfoPanelProps) => {
    const [thumbnailUrl, setThumbnailUrl] = useState<string | null>(null);
    const [strategy, setStrategy] = useState<'min_skew' | 'uniform' | 'max_skew'>('uniform');
    const queryClient = useQueryClient();

    // Get fresh POI data from the polled pois array
    const freshPoi = pois.find(p => p.wikidata_id === poi?.wikidata_id);
    const thumbnailFromData = freshPoi?.thumbnail_url || poi?.thumbnail_url;

    useEffect(() => {
        if (!poi) {
            // eslint-disable-next-line react-hooks/set-state-in-effect
            setThumbnailUrl(null);
            return;
        }

        // Sync narration strategy if available in fresh data
        if (freshPoi?.narration_strategy) {
            setStrategy(freshPoi.narration_strategy as any);
        }

        // 1. Use thumbnail from fresh data if available
        if (thumbnailFromData) {
            setThumbnailUrl(thumbnailFromData);
            return;
        }

        // 2. Fallback: Fetch thumbnail on-demand from API
        const fetchThumbnail = async () => {
            try {
                const res = await fetch(`/api/pois/${poi.wikidata_id}/thumbnail`);
                if (res.ok) {
                    const data = await res.json();
                    if (data.url) {
                        setThumbnailUrl(data.url);
                    }
                }
            } catch (e) {
                console.error("Failed to fetch thumbnail", e);
            }
        };

        fetchThumbnail();
    }, [poi, thumbnailFromData]);

    if (!poi) return null;

    const primaryName = getName(poi);
    const localName = getLocalNameIfDifferent(poi, primaryName);

    const handlePlay = async () => {
        // Optimistic update
        queryClient.setQueryData(['audioStatus'], (old: AudioStatus | undefined) => {
            if (!old) return old;
            return {
                ...old,
                is_playing: true,
                title: 'Loading: ' + primaryName,
                // zero out progress
                position: 0,
                duration: 0
            };
        });

        try {
            await fetch('/api/narrator/play', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    poi_id: poi.wikidata_id,
                    strategy: strategy
                })
            });
            // Force immediate refetch to get real state if it happened fast
            queryClient.invalidateQueries({ queryKey: ['audioStatus'] });
        } catch (e) {
            console.error("Failed to trigger play", e);
            // Revert on error? Handled by next poll usually.
            queryClient.invalidateQueries({ queryKey: ['audioStatus'] });
        }
    };

    return (
        <div className="hud-container poi-info-panel" style={{ position: 'relative', flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
            {/* Close button (absolute top-right) */}
            <button
                onClick={onClose}
                style={{
                    position: 'absolute',
                    top: '8px',
                    right: '8px',
                    background: 'transparent',
                    border: 'none',
                    color: '#666',
                    fontSize: '20px',
                    cursor: 'pointer',
                    padding: '0 4px',
                    lineHeight: 1,
                    zIndex: 10,
                }}
            >
                &times;
            </button>

            {/* Main layout: Text on left, Thumbnail on right */}
            <div style={{ display: 'flex', gap: '12px', flex: 1, minHeight: 0 }}>
                {/* Left column: Text content (40%) */}
                <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                    {/* Header */}
                    <div className="value" style={{ fontSize: '16px', marginBottom: '4px', textTransform: 'none', display: 'flex', alignItems: 'center', gap: '4px', flexWrap: 'wrap', paddingRight: '24px' }}>
                        {primaryName}
                        <div style={{ display: 'flex', alignItems: 'center' }}>
                            <button
                                onClick={(e) => {
                                    console.log("Play button clicked for", poi.wikidata_id);
                                    e.stopPropagation();
                                    handlePlay();
                                }}
                                title="Play Narration"
                                style={{
                                    background: 'transparent',
                                    border: '1px solid var(--accent)',
                                    borderRadius: '50%',
                                    width: '24px',
                                    height: '24px',
                                    display: 'flex',
                                    alignItems: 'center',
                                    justifyContent: 'center',
                                    color: 'var(--accent)',
                                    cursor: 'pointer',
                                    fontSize: '12px',
                                    flexShrink: 0,
                                    zIndex: 20, /* Ensure it's clickable */
                                    position: 'relative' /* Context for z-index */
                                }}
                            >
                                ▶
                            </button>
                            <div className="length-selector">
                                <button
                                    className={`length-btn ${strategy === 'min_skew' ? 'active' : ''}`}
                                    onClick={(e) => { e.stopPropagation(); setStrategy('min_skew'); }}
                                    title="Short Narration"
                                >S</button>
                                <button
                                    className={`length-btn ${strategy === 'uniform' ? 'active' : ''}`}
                                    onClick={(e) => { e.stopPropagation(); setStrategy('uniform'); }}
                                    title="Standard Narration"
                                >M</button>
                                <button
                                    className={`length-btn ${strategy === 'max_skew' ? 'active' : ''}`}
                                    onClick={(e) => { e.stopPropagation(); setStrategy('max_skew'); }}
                                    title="Long Narration"
                                >L</button>
                            </div>
                        </div>
                    </div>
                    {localName && (
                        <div style={{ fontSize: '12px', color: '#888', marginBottom: '4px' }}>
                            ({localName})
                        </div>
                    )}
                    <div className="label" style={{ marginBottom: '8px' }}>
                        {poi.category}
                        {poi.specific_category && poi.specific_category !== poi.category && (
                            <span style={{ color: '#888', fontWeight: 'normal' }}> ({poi.specific_category})</span>
                        )}
                    </div>

                    {/* Info */}
                    <div className="status-pill" style={{ display: 'inline-flex', alignItems: 'center', marginBottom: '8px', flexShrink: 0 }}>
                        <div className="status-dot connected" style={{ backgroundColor: getColor(poi.score) }}></div>
                        <span style={{ color: '#ccc' }}>Score: {poi.score?.toFixed(1)}</span>
                    </div>

                    {poi.last_played && poi.last_played !== "0001-01-01T00:00:00Z" && (
                        <div style={{ fontSize: '11px', color: '#666', marginBottom: '8px' }}>
                            Last Played: {formatTimeAgo(poi.last_played)}
                        </div>
                    )}

                    {poi.wp_url && (
                        <div style={{ marginBottom: '8px' }}>
                            <a href={poi.wp_url} target="_blank" rel="noopener noreferrer" style={{ color: 'var(--accent)', textDecoration: 'none', fontSize: '12px' }}>
                                Wikipedia Article &rarr;
                            </a>
                        </div>
                    )}

                    {/* Score Details */}
                    {poi.score_details && (
                        <div style={{ marginTop: '4px', flex: '1 1 auto', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
                            <div className="label" style={{ marginBottom: '4px', flexShrink: 0 }}>Score Breakdown</div>
                            <div style={{
                                fontSize: '10px',
                                color: '#888',
                                fontFamily: 'monospace',
                                whiteSpace: 'pre-wrap',
                                overflowY: 'auto',
                                background: 'rgba(0,0,0,0.2)',
                                padding: '6px',
                                borderRadius: '4px',
                                border: '1px solid rgba(255,255,255,0.05)',
                                flex: 1
                            }}>
                                {poi.score_details}
                            </div>
                        </div>
                    )}
                </div>

                {/* Right column: Thumbnail (60% basis) */}
                {thumbnailUrl && (
                    <div style={{ flex: '0 0 60%' }}>
                        <img
                            src={thumbnailUrl}
                            alt={primaryName}
                            style={{
                                width: '100%',
                                height: 'auto',
                                objectFit: 'cover',
                                borderRadius: '4px',
                                border: '1px solid rgba(255, 255, 255, 0.1)'
                            }}
                        />
                    </div>
                )}
            </div>
        </div>
    );
};
