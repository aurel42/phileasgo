export const ARTISTIC_MAP_STYLES = {
    colors: {
        text: {
            active: '#0a0805',
            historical: '#3a2a1d',
        },
        icon: {
            gold: '#D4AF37',
            silver: '#C0C0C0',
            copper: '#B55A30',
            historical: '#484848', // Cast Iron — visited/historic (Dark Grey to contrast with Silver)
            // Harmonized with Red Balloon (#E63946)
            // Complementary contrast for visibility
            selected: '#e63946',     // Comic Red — Active (Matches Balloon/Seal)
            next: '#E9C46A',         // Saffron Gold — Preparing
            normalHalo: '#f4ecd8',   // Paper color — Cutout effect for Low Score (< 20)
            selectedHalo: '#f4ecd8', // Reverting to Paper — Thickness handles state, color handles score
            nextHalo: '#f4ecd8',     // Reverting to Paper — Thickness handles state, color handles score
            neonCyan: '#00f3ff',     // Cyberpunk Neon
            neonPink: '#ff00ff',     // Cyberpunk Neon
            organicSmudge: '#3a2a1d', // Ink smudge
            stroke: '#0a0805',
            compass: '#0D3B3F' // Dark Teal
        },
        shadows: {
            // Halo effect: Strong light stroke (paper color) to cut text out from background
            atmosphere: '-1px -1px 0 #f4ecd8, 1px -1px 0 #f4ecd8, -1px 1px 0 #f4ecd8, 1px 1px 0 #f4ecd8, 0 0 3px #f4ecd8, 0 0 1px rgba(10, 8, 5, 0.35)'
        }
    },
    tethers: {
        stroke: '#292929',
        width: '1.2',
        opacity: 0.6,
        dotRadius: 3.0,
        dotOpacity: 0.8
    }
};
